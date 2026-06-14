package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"assh/asshc/domain"
)

// OpenAIClient 实现 OpenAI 兼容 API 的客户端。
type OpenAIClient struct {
	config     *domain.AIConfig
	httpClient *http.Client
}

// NewOpenAIClient 创建 OpenAIClient 实例。
func NewOpenAIClient(config *domain.AIConfig) *OpenAIClient {
	return &OpenAIClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ChatMessage 表示聊天消息。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 表示聊天请求。
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

// ChatResponse 表示聊天响应。
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Generate 根据用户提示生成内容。
func (c *OpenAIClient) Generate(ctx context.Context, prompt string) (string, error) {
	return c.GenerateWithSystem(ctx, "", prompt)
}

// GenerateWithSystem 使用系统提示和用户提示生成内容。
func (c *OpenAIClient) GenerateWithSystem(ctx context.Context, system, user string) (string, error) {
	messages := []ChatMessage{}

	if system != "" {
		messages = append(messages, ChatMessage{
			Role:    "system",
			Content: system,
		})
	}

	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: user,
	})

	request := ChatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
	}

	response, err := c.doRequest(ctx, request)
	if err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from AI service")
	}

	return response.Choices[0].Message.Content, nil
}

// GenerateScript 根据漏洞信息生成检测脚本。
func (c *OpenAIClient) GenerateScript(ctx context.Context, vulnInfo string, cveDetail *domain.CVEDetail) (string, error) {
	systemPrompt := `你是一个安全检测专家。根据用户提供的漏洞信息，生成一个 bash 检测脚本。

要求：
1. 脚本必须是可执行的 bash 脚本
2. 包含注释说明检测目标
3. 检查逻辑清晰
4. 结果输出符合统一格式
5. 退出码约定：0=无漏洞, 1=存在漏洞, 2=检测失败

输出格式：
- 只输出脚本内容，不要包含其他说明
- 脚本必须以 #!/bin/bash 开头
- 包含检测结果输出`

	userPrompt := fmt.Sprintf("漏洞信息：%s", vulnInfo)
	if cveDetail != nil {
		userPrompt += fmt.Sprintf("\n\nCVE 详情：\n- ID: %s\n- 描述: %s\n- 评分: %.1f\n- 严重程度: %s",
			cveDetail.ID, cveDetail.Description, cveDetail.Score, cveDetail.Severity)
	}

	return c.GenerateWithSystem(ctx, systemPrompt, userPrompt)
}

// ValidateConnection 验证 AI 服务连接。
func (c *OpenAIClient) ValidateConnection(ctx context.Context) error {
	// 发送简单的测试请求
	_, err := c.Generate(ctx, "Hello")
	return err
}

// GetModel 返回当前使用的模型名称。
func (c *OpenAIClient) GetModel() string {
	return c.config.Model
}

// SetModel 设置使用的模型名称。
func (c *OpenAIClient) SetModel(model string) {
	c.config.Model = model
}

// doRequest 执行 HTTP 请求。
func (c *OpenAIClient) doRequest(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	// 序列化请求
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建请求
	url := fmt.Sprintf("%s/chat/completions", c.config.Endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned error: %s - %s", resp.Status, string(respBody))
	}

	// 解析响应
	var response ChatResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}
