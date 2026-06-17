package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"assh/asshc/domain"
)

// OpenAIClient 实现 OpenAI 兼容 API 的客户端。
type OpenAIClient struct {
	config     *domain.AIConfig
	httpClient *http.Client
	debug      bool // --show-logs 时输出 HTTP 请求/响应详情
}

// SetDebug 启用或禁用 HTTP 调试日志。
func (c *OpenAIClient) SetDebug(on bool) {
	c.debug = on
}

// NewOpenAIClient 创建 OpenAIClient 实例。
func NewOpenAIClient(config *domain.AIConfig) *OpenAIClient {
	timeout := 60 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}
	return &OpenAIClient{
		config: config,
		httpClient: &http.Client{
			Timeout: timeout,
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
- 只输出纯脚本内容，不要使用 markdown 代码块包裹
- 脚本必须以 #!/bin/bash 开头
- 包含检测结果输出`

	userPrompt := fmt.Sprintf("漏洞信息：%s", vulnInfo)
	if cveDetail != nil {
		userPrompt += fmt.Sprintf("\n\nCVE 详情：\n- ID: %s\n- 描述: %s\n- 评分: %.1f\n- 严重程度: %s",
			cveDetail.ID, cveDetail.Description, cveDetail.Score, cveDetail.Severity)
	}

	script, err := c.GenerateWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	return cleanMarkdownFence(script), nil
}

// AnalyzeExecution 分析脚本执行结果，给出修复建议或安全确认。
func (c *OpenAIClient) AnalyzeExecution(ctx context.Context, vulnInfo string, script string, output string, exitCode int) (string, error) {
	var resultText string
	switch exitCode {
	case 0:
		resultText = "安全（退出码 0，未发现漏洞迹象）"
	case 1:
		resultText = "存在漏洞（退出码 1，检测到漏洞特征）"
	case 2:
		resultText = "检测失败（退出码 2，脚本执行出错）"
	default:
		resultText = fmt.Sprintf("未知结果（退出码 %d）", exitCode)
	}

	systemPrompt := `你是一个安全分析专家。根据检测脚本的执行结果，进行分析并输出汇总报告。

请分析以下内容：
1. 执行结果的总结（通过/失败/异常）
2. 如果存在漏洞：说明漏洞的严重程度、影响范围，并给出具体的修复建议
3. 如果安全：确认安全状态，说明检测覆盖了哪些方面
4. 对输出的关键信息进行解读

输出格式要求：
- 以简洁的 Markdown 格式输出
- 如果存在漏洞，使用 ### 修复建议 小节列出具体步骤
- 如果安全，使用 ### 安全确认 小节总结
- 不要使用代码块包裹`

	userPrompt := fmt.Sprintf(`## 漏洞信息
%s

## 检测脚本
%s

## 执行结果
%s

## 脚本输出
%s`, vulnInfo, script, resultText, output)

	return c.GenerateWithSystem(ctx, systemPrompt, userPrompt)
}

// cleanMarkdownFence 去除 AI 返回内容中的 markdown 代码块标记。
// 有些 AI 即使被要求输出纯文本，仍然会包裹 ```bash ... ``` 标记。
// 处理示例：
//
//	```bash\n#!/bin/bash\necho "hi"\n```  →  #!/bin/bash\necho "hi"
//	```\n#!/bin/bash\necho "hi"\n```      →  #!/bin/bash\necho "hi"
//	```bash\necho "hi"\n```               →  echo "hi"
//	#!/bin/bash\necho "hi"                 →  #!/bin/bash\necho "hi" (不变)
func cleanMarkdownFence(s string) string {
	s = strings.TrimSpace(s)

	// 去除开头的 ```bash, ```sh, ```shell, ``` 等标记（可选语言标识）
	reOpen := regexp.MustCompile("^```[a-zA-Z]*\n")
	s = reOpen.ReplaceAllString(s, "")

	// 去除结尾的 ```
	reClose := regexp.MustCompile("\n```$")
	s = reClose.ReplaceAllString(s, "")

	// 处理单行形式（无换行的短脚本）
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")

	return strings.TrimSpace(s)
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

	// --show-logs 调试输出
	if c.debug {
		fmt.Fprintf(os.Stderr, "\n── AI Request ──────────────────────────────\n")
		fmt.Fprintf(os.Stderr, "POST %s\n", url)
		fmt.Fprintf(os.Stderr, "Headers:\n")
		for k, v := range req.Header {
			if k == "Authorization" {
				fmt.Fprintf(os.Stderr, "  %s: Bearer %s…\n", k, c.config.APIKey[:8])
			} else {
				fmt.Fprintf(os.Stderr, "  %s: %s\n", k, v)
			}
		}
		fmt.Fprintf(os.Stderr, "Body:\n%s\n", string(body))
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

	// --show-logs 调试输出
	if c.debug {
		fmt.Fprintf(os.Stderr, "\n── AI Response ─────────────────────────────\n")
		fmt.Fprintf(os.Stderr, "Status: %s\n", resp.Status)
		bodyPreview := string(respBody)
		if len(bodyPreview) > 800 {
			bodyPreview = bodyPreview[:800] + "\n... (truncated)"
		}
		fmt.Fprintf(os.Stderr, "Body:\n%s\n", bodyPreview)
		fmt.Fprintf(os.Stderr, "────────────────────────────────────────────\n\n")
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
