package port

import (
	"time"

	"assh/asshc/domain"
)

// HealthStatus 表示服务器健康状态。
type HealthStatus int

const (
	HealthStatusHealthy   HealthStatus = iota // 健康：SSH 连接成功
	HealthStatusUnhealthy                     // 不健康：连接成功但系统异常
	HealthStatusTimeout                       // 超时：连接超时
	HealthStatusError                         // 错误：连接失败
)

// String 返回 HealthStatus 的可读字符串。
func (s HealthStatus) String() string {
	switch s {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusUnhealthy:
		return "unhealthy"
	case HealthStatusTimeout:
		return "timeout"
	case HealthStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// HealthResult 表示单台服务器的健康检查结果。
type HealthResult struct {
	Server    string         `json:"server"`     // 服务器名称
	Host      string         `json:"host"`       // 主机地址
	Port      int            `json:"port"`       // SSH 端口
	Status    HealthStatus   `json:"status"`     // 健康状态
	Latency   time.Duration  `json:"latency"`    // 连接延迟
	Error     string         `json:"error"`      // 错误信息（如果有）
	CheckedAt time.Time      `json:"checked_at"` // 检查时间
	Details   *HealthDetails `json:"details"`    // 系统详细信息（可选）
}

// HealthDetails 表示远程服务器的系统信息。
type HealthDetails struct {
	Uptime  string `json:"uptime"`   // 系统运行时间
	LoadAvg string `json:"load_avg"` // 负载均值
	Memory  string `json:"memory"`   // 内存使用情况
	Disk    string `json:"disk"`     // 磁盘使用情况
}

// HealthChecker 定义服务器健康检查接口。
// 通过 SSH 连接检测服务器可达性，并采集系统信息。
type HealthChecker interface {
	// Check 对单台服务器执行健康检查，返回检查结果。
	// 如果 detail 为 true，会采集远程系统信息（uptime/load/memory/disk）。
	Check(server *domain.Server, detail bool) (*HealthResult, error)
}
