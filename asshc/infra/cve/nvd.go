package cve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"assh/asshc/domain"
)

// NVDClient 实现 NVD NIST API 的客户端。
type NVDClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewNVDClient 创建 NVDClient 实例。
func NewNVDClient() *NVDClient {
	return &NVDClient{
		baseURL: "https://services.nvd.nist.gov/rest/json/cves/2.0",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NVDResponse 表示 NVD API 响应。
type NVDResponse struct {
	Vulnerabilities []struct {
		CVE struct {
			ID          string `json:"id"`
			Descriptions []struct {
				Lang  string `json:"lang"`
				Value string `json:"value"`
			} `json:"descriptions"`
			Metrics struct {
				CVSSMetricV31 []struct {
					CVSSData struct {
						BaseScore    float64 `json:"baseScore"`
						BaseSeverity string  `json:"baseSeverity"`
					} `json:"cvssData"`
				} `json:"cvssMetricV31"`
				CVSSMetricV2 []struct {
					CVSSData struct {
						BaseScore    float64 `json:"baseScore"`
						BaseSeverity string  `json:"baseSeverity"`
					} `json:"cvssData"`
				} `json:"cvssMetricV2"`
			} `json:"metrics"`
			Published  string `json:"published"`
			LastModified string `json:"lastModified"`
			References []struct {
				URL string `json:"url"`
			} `json:"references"`
		} `json:"cve"`
	} `json:"vulnerabilities"`
}

// Fetch 获取 CVE 详细信息。
func (c *NVDClient) Fetch(ctx context.Context, cveID string) (*domain.CVEDetail, error) {
	// 验证 CVE ID 格式
	if !c.ValidateCVEID(cveID) {
		return nil, fmt.Errorf("invalid CVE format: %s", cveID)
	}

	// 构建请求 URL
	url := fmt.Sprintf("%s?cveId=%s", c.baseURL, cveID)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "assh/2.2.0")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NVD API returned error: %s - %s", resp.Status, string(body))
	}

	// 解析响应
	var nvdResp NVDResponse
	if err := json.Unmarshal(body, &nvdResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 检查是否有数据
	if len(nvdResp.Vulnerabilities) == 0 {
		return nil, fmt.Errorf("CVE not found: %s", cveID)
	}

	// 转换为领域模型
	vuln := nvdResp.Vulnerabilities[0].CVE
	detail := &domain.CVEDetail{
		ID: vuln.ID,
	}

	// 获取描述
	for _, desc := range vuln.Descriptions {
		if desc.Lang == "en" {
			detail.Description = desc.Value
			break
		}
	}

	// 获取评分
	if len(vuln.Metrics.CVSSMetricV31) > 0 {
		detail.Score = vuln.Metrics.CVSSMetricV31[0].CVSSData.BaseScore
		detail.Severity = vuln.Metrics.CVSSMetricV31[0].CVSSData.BaseSeverity
	} else if len(vuln.Metrics.CVSSMetricV2) > 0 {
		detail.Score = vuln.Metrics.CVSSMetricV2[0].CVSSData.BaseScore
		detail.Severity = vuln.Metrics.CVSSMetricV2[0].CVSSData.BaseSeverity
	}

	// 解析时间
	if vuln.Published != "" {
		detail.Published, _ = time.Parse(time.RFC3339, vuln.Published)
	}
	if vuln.LastModified != "" {
		detail.Modified, _ = time.Parse(time.RFC3339, vuln.LastModified)
	}

	// 获取参考链接
	for _, ref := range vuln.References {
		detail.References = append(detail.References, ref.URL)
	}

	return detail, nil
}

// FetchBatch 批量获取 CVE 详细信息。
func (c *NVDClient) FetchBatch(ctx context.Context, cveIDs []string) (map[string]*domain.CVEDetail, error) {
	result := make(map[string]*domain.CVEDetail)

	for _, cveID := range cveIDs {
		detail, err := c.Fetch(ctx, cveID)
		if err != nil {
			// 记录错误但继续处理其他 CVE
			result[cveID] = &domain.CVEDetail{
				ID:          cveID,
				Description: fmt.Sprintf("Failed to fetch: %v", err),
			}
			continue
		}
		result[cveID] = detail
	}

	return result, nil
}

// ValidateCVEID 验证 CVE ID 格式。
func (c *NVDClient) ValidateCVEID(cveID string) bool {
	// CVE 格式：CVE-YYYY-NNNNNN
	pattern := `^CVE-\d{4}-\d{4,}$`
	matched, _ := regexp.MatchString(pattern, cveID)
	return matched
}
