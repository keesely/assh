// Package port 定义核心业务接口（端口），实现依赖倒置。
//
// 本包只定义接口和领域类型引用，不包含任何实现。
// service 层依赖本包的接口，infra 层提供具体实现，
// 从而实现依赖倒置原则——高层模块不依赖低层模块的具体实现。
package port

// ServiceType 服务类型枚举
type ServiceType string

const (
	ServiceTypeProxy  ServiceType = "proxy"
	ServiceTypeTunnel ServiceType = "tunnel"
)

// ServiceStatus 服务状态枚举
type ServiceStatus string

const (
	ServiceStatusRunning ServiceStatus = "running"
	ServiceStatusStopped ServiceStatus = "stopped"
	ServiceStatusFailed  ServiceStatus = "failed"
)

// ServiceInstance 统一的服务实例表示。
// 用于 ServiceManager 管理代理和隧道服务的生命周期。
type ServiceInstance struct {
	ID         int64         `json:"id"`          // 数据库主键
	Name       string        `json:"name"`         // 服务名称（唯一）
	Type       ServiceType   `json:"type"`         // proxy / tunnel
	ServerName string        `json:"server_name"`  // 关联服务器名称
	Status     ServiceStatus `json:"status"`       // 运行状态
	PID        int           `json:"pid,omitempty"` // 守护进程 PID
	Remark     string        `json:"remark,omitempty"`
	CreatedAt  string        `json:"created_at"`
	UpdatedAt  string        `json:"updated_at"`
}

// ServiceFilter 服务列表过滤条件
type ServiceFilter struct {
	Type   ServiceType   // 按服务类型过滤（空=不过滤）
	Status ServiceStatus // 按状态过滤（空=不过滤）
	Server string        // 按服务器名称过滤（空=不过滤）
}

// ServiceManager 定义统一的服务生命周期管理接口。
// 代理和隧道服务通过此接口进行启动、停止、重启和状态查询。
// 实现层负责将代理和隧道服务的差异封装在统一接口之后。
type ServiceManager interface {
	// Start 启动指定服务实例。
	// 对于代理服务，启动 SOCKS5/HTTP/TCP 代理。
	// 对于隧道服务，启动端口转发。
	Start(instance *ServiceInstance) error

	// Stop 停止指定服务实例。
	// 对于守护进程模式，发送 SIGTERM 信号。
	Stop(id int64) error

	// Restart 重启指定服务实例。
	// 等价于 Stop + Start。
	Restart(id int64) error

	// Status 查询指定服务实例的状态。
	Status(id int64) (ServiceStatus, error)

	// List 列出符合条件的服务实例。
	List(filter ServiceFilter) ([]*ServiceInstance, error)
}
