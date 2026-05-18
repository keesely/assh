package ssh

import (
	"fmt"
	"strings"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
	asshssh "golang.org/x/crypto/ssh"
)

// HealthChecker 实现 port.HealthChecker 接口。
// 通过 SSH 连接检测服务器可达性，可选采集远程系统信息。
type HealthChecker struct {
	connector *Connector
}

// NewHealthChecker 创建 HealthChecker 实例。
func NewHealthChecker(connector *Connector) *HealthChecker {
	return &HealthChecker{connector: connector}
}

// Check 对单台服务器执行健康检查。
// 测量 SSH 连接延迟，如果 detail 为 true 则采集系统信息。
func (h *HealthChecker) Check(server *domain.Server, detail bool) (*port.HealthResult, error) {
	result := &port.HealthResult{
		Server:    domain.JoinName(server.Group, server.Name),
		Host:      server.Host,
		Port:      server.Port,
		CheckedAt: time.Now(),
	}

	// 测量连接延迟
	start := time.Now()
	client, err := h.connector.Connect(server)
	latency := time.Since(start)
	result.Latency = latency

	if err != nil {
		result.Status = classifyError(err, latency)
		result.Error = err.Error()
		return result, nil
	}
	defer client.Close()

	result.Status = port.HealthStatusHealthy

	// 采集详细系统信息
	if detail {
		details, err := h.collectDetails(client)
		if err != nil {
			// 连接成功但采集信息失败，标记为 unhealthy
			result.Status = port.HealthStatusUnhealthy
			result.Error = fmt.Sprintf("connected but failed to collect details: %v", err)
		} else {
			result.Details = details
		}
	}

	return result, nil
}

// collectDetails 通过 SSH 执行远程命令采集系统信息。
func (h *HealthChecker) collectDetails(client *asshssh.Client) (*port.HealthDetails, error) {
	details := &port.HealthDetails{}

	// 获取 uptime
	if output, err := runCmd(client, "uptime -p 2>/dev/null || uptime"); err == nil {
		details.Uptime = strings.TrimSpace(output)
	}

	// 获取 load average
	if output, err := runCmd(client, "cat /proc/loadavg 2>/dev/null | awk '{print $1, $2, $3}'"); err == nil {
		details.LoadAvg = strings.TrimSpace(output)
	}

	// 获取内存信息
	if output, err := runCmd(client, "free -h 2>/dev/null | awk '/^Mem:/{printf \"%s/%s\", $3, $2}'"); err == nil {
		details.Memory = strings.TrimSpace(output)
	}

	// 获取磁盘信息
	if output, err := runCmd(client, "df -h / 2>/dev/null | awk 'NR==2{printf \"%s/%s (%s)\", $3, $2, $5}'"); err == nil {
		details.Disk = strings.TrimSpace(output)
	}

	return details, nil
}

// runCmd 在远程服务器上执行命令并返回输出。
func runCmd(client *asshssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

// classifyError 根据错误类型和延迟时间判断健康状态。
func classifyError(err error, latency time.Duration) port.HealthStatus {
	errStr := err.Error()
	// 超时判断：连接延迟超过 30 秒 或 错误信息包含 timeout
	if latency > 30*time.Second || strings.Contains(strings.ToLower(errStr), "timeout") {
		return port.HealthStatusTimeout
	}
	return port.HealthStatusError
}
