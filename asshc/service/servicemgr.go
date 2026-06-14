// Package service 实现服务编排层，管理代理和隧道服务的生命周期。
//
// ServiceManagerImpl 实现 port.ServiceManager 接口，
// 统一管理代理和隧道服务的启动、停止、重启和状态查询。
package service

import (
	"fmt"
	"sync"
	"time"

	"assh/asshc/port"
	"assh/log"
)

// ServiceManagerImpl 实现 port.ServiceManager 接口。
// 代理和隧道服务通过此实现进行统一管理。
type ServiceManagerImpl struct {
	proxyRepo  port.ProxyServiceRepository
	tunnelRepo port.TunnelServiceRepository
	proxySvc   *ProxyService
	mu         sync.Mutex
}

// NewServiceManagerImpl 创建 ServiceManagerImpl 实例。
func NewServiceManagerImpl(
	proxyRepo port.ProxyServiceRepository,
	tunnelRepo port.TunnelServiceRepository,
	proxySvc *ProxyService,
) *ServiceManagerImpl {
	return &ServiceManagerImpl{
		proxyRepo:  proxyRepo,
		tunnelRepo: tunnelRepo,
		proxySvc:   proxySvc,
	}
}

// 编译期接口合规检查
var _ port.ServiceManager = (*ServiceManagerImpl)(nil)

// Start 启动指定服务实例。
func (m *ServiceManagerImpl) Start(instance *port.ServiceInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch instance.Type {
	case port.ServiceTypeProxy:
		return m.startProxy(instance)
	case port.ServiceTypeTunnel:
		return m.startTunnel(instance)
	default:
		return fmt.Errorf("unknown service type: %s", instance.Type)
	}
}

// startProxy 启动代理服务。
func (m *ServiceManagerImpl) startProxy(instance *port.ServiceInstance) error {
	// 从存储获取完整配置
	svc, err := m.proxyRepo.GetByID(instance.ID)
	if err != nil {
		return fmt.Errorf("get proxy service: %w", err)
	}

	// 构建代理选项
	opts := ProxyOptions{
		SOCKS5Addr:    svc.SOCKS5Addr,
		HTTPAddr:      svc.HTTPAddr,
		TCPAddr:       svc.TCPAddr,
		Reverse:       svc.Reverse,
		ListenType:    svc.ListenType,
		Auth:          svc.Auth,
		RuleFile:      svc.RuleFile,
		LogDir:        svc.LogDir,
		Daemon:        svc.Daemon,
		AutoReconnect: svc.AutoReconnect,
	}

	// 启动代理
	if err := m.proxySvc.StartProxy(svc.ServerName, opts); err != nil {
		return fmt.Errorf("start proxy: %w", err)
	}

	// 记录使用
	if err := m.proxyRepo.RecordUsage(svc.ID); err != nil {
		// 非致命错误，仅记录日志
		fmt.Printf("warning: record proxy usage: %v\n", err)
	}

	return nil
}

// startTunnel 启动隧道服务。
func (m *ServiceManagerImpl) startTunnel(instance *port.ServiceInstance) error {
	// 从存储获取完整配置
	svc, err := m.tunnelRepo.GetByID(instance.ID)
	if err != nil {
		return fmt.Errorf("get tunnel service: %w", err)
	}

	// 构建转发规格
	forwards := make([]ForwardSpec, len(svc.Forwards))
	for i, fwd := range svc.Forwards {
		forwards[i] = ForwardSpec{
			LocalAddr:  fwd.LocalAddr,
			RemoteAddr: fwd.RemoteAddr,
			Reverse:    fwd.Reverse,
		}
	}

	// 构建代理选项
	opts := ProxyOptions{
		Daemon:        svc.Daemon,
		AutoReconnect: svc.AutoReconnect,
	}

	// 启动隧道
	if err := m.proxySvc.StartTunnel(svc.ServerName, forwards, opts); err != nil {
		return fmt.Errorf("start tunnel: %w", err)
	}

	// 记录使用
	if err := m.tunnelRepo.RecordUsage(svc.ID); err != nil {
		// 非致命错误，仅记录日志
		fmt.Printf("warning: record tunnel usage: %v\n", err)
	}

	return nil
}

// Stop 停止指定服务实例。
func (m *ServiceManagerImpl) Stop(id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 尝试作为代理服务停止
	if err := m.proxySvc.StopProxy(); err == nil {
		return nil
	}

	// 尝试作为隧道服务停止
	if err := m.proxySvc.StopTunnel(fmt.Sprintf("%d", id)); err == nil {
		return nil
	}

	return fmt.Errorf("service %d not found or not running", id)
}

// Restart 重启指定服务实例。
func (m *ServiceManagerImpl) Restart(id int64) error {
	// 先停止
	if err := m.Stop(id); err != nil {
		// 可能服务未运行，继续尝试启动
	}

	// 获取服务实例信息
	instance, err := m.getStatus(id)
	if err != nil {
		return fmt.Errorf("get service instance: %w", err)
	}

	// 启动
	return m.Start(instance)
}

// Status 查询指定服务实例的状态。
func (m *ServiceManagerImpl) Status(id int64) (port.ServiceStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, err := m.getStatus(id)
	if err != nil {
		return port.ServiceStatusStopped, err
	}

	return instance.Status, nil
}

// getStatus 获取服务实例信息（内部方法，不加锁）。
func (m *ServiceManagerImpl) getStatus(id int64) (*port.ServiceInstance, error) {
	// 尝试作为代理服务查询
	proxySvc, err := m.proxyRepo.GetByID(id)
	if err == nil {
		return &port.ServiceInstance{
			ID:         proxySvc.ID,
			Name:       proxySvc.Name,
			Type:       port.ServiceTypeProxy,
			ServerName: proxySvc.ServerName,
			Status:     port.ServiceStatusStopped, // 默认状态，实际状态需要运行时跟踪
			Remark:     proxySvc.Remark,
			CreatedAt:  proxySvc.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:  proxySvc.UpdatedAt.Format("2006-01-02 15:04:05"),
		}, nil
	}

	// 尝试作为隧道服务查询
	tunnelSvc, err := m.tunnelRepo.GetByID(id)
	if err == nil {
		return &port.ServiceInstance{
			ID:         tunnelSvc.ID,
			Name:       tunnelSvc.Name,
			Type:       port.ServiceTypeTunnel,
			ServerName: tunnelSvc.ServerName,
			Status:     port.ServiceStatusStopped, // 默认状态
			Remark:     tunnelSvc.Remark,
			CreatedAt:  tunnelSvc.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:  tunnelSvc.UpdatedAt.Format("2006-01-02 15:04:05"),
		}, nil
	}

	return nil, fmt.Errorf("service %d not found", id)
}

// List 列出符合条件的服务实例。
func (m *ServiceManagerImpl) List(filter port.ServiceFilter) ([]*port.ServiceInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*port.ServiceInstance

	// 查询代理服务
	if filter.Type == "" || filter.Type == port.ServiceTypeProxy {
		proxies, err := m.proxyRepo.List("")
		if err != nil {
			return nil, fmt.Errorf("list proxy services: %w", err)
		}
		for _, svc := range proxies {
			if filter.Server != "" && svc.ServerName != filter.Server {
				continue
			}
			instance := &port.ServiceInstance{
				ID:         svc.ID,
				Name:       svc.Name,
				Type:       port.ServiceTypeProxy,
				ServerName: svc.ServerName,
				Status:     port.ServiceStatusStopped, // 默认状态
				Remark:     svc.Remark,
				CreatedAt:  svc.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdatedAt:  svc.UpdatedAt.Format("2006-01-02 15:04:05"),
			}
			if filter.Status == "" || instance.Status == filter.Status {
				result = append(result, instance)
			}
		}
	}

	// 查询隧道服务
	if filter.Type == "" || filter.Type == port.ServiceTypeTunnel {
		tunnels, err := m.tunnelRepo.List("")
		if err != nil {
			return nil, fmt.Errorf("list tunnel services: %w", err)
		}
		for _, svc := range tunnels {
			if filter.Server != "" && svc.ServerName != filter.Server {
				continue
			}
			instance := &port.ServiceInstance{
				ID:         svc.ID,
				Name:       svc.Name,
				Type:       port.ServiceTypeTunnel,
				ServerName: svc.ServerName,
				Status:     port.ServiceStatusStopped, // 默认状态
				Remark:     svc.Remark,
				CreatedAt:  svc.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdatedAt:  svc.UpdatedAt.Format("2006-01-02 15:04:05"),
			}
			if filter.Status == "" || instance.Status == filter.Status {
				result = append(result, instance)
			}
		}
	}

	return result, nil
}

// RestoreAll 尝试恢复所有已启用的服务。
// 遍历 proxy_services 和 tunnel_services 仓库，对 Enabled == true 的服务
// 执行自动恢复启动，支持指数退避重试。
func (m *ServiceManagerImpl) RestoreAll(cfg port.RestoreConfig) port.RestoreResult {
	result := port.RestoreResult{
		Errors: make(map[int64]error),
	}

	// 恢复代理服务
	proxies, err := m.proxyRepo.List("")
	if err != nil {
		log.Warnf("restore: list proxy services failed: %v", err)
	} else {
		for _, svc := range proxies {
			if !svc.Enabled {
				continue
			}
			result.Total++
			instance := &port.ServiceInstance{
				ID:   svc.ID,
				Name: svc.Name,
				Type: port.ServiceTypeProxy,
			}
			if err := m.restoreWithRetry(instance, cfg); err != nil {
				result.Failed++
				result.Errors[svc.ID] = err
				log.Warnf("restore proxy service %q (id=%d) failed: %v", svc.Name, svc.ID, err)
			} else {
				result.Succeeded++
				log.Infof("restore proxy service %q (id=%d) succeeded", svc.Name, svc.ID)
			}
		}
	}

	// 恢复隧道服务
	tunnels, err := m.tunnelRepo.List("")
	if err != nil {
		log.Warnf("restore: list tunnel services failed: %v", err)
	} else {
		for _, svc := range tunnels {
			if !svc.Enabled {
				continue
			}
			result.Total++
			instance := &port.ServiceInstance{
				ID:   svc.ID,
				Name: svc.Name,
				Type: port.ServiceTypeTunnel,
			}
			if err := m.restoreWithRetry(instance, cfg); err != nil {
				result.Failed++
				result.Errors[svc.ID] = err
				log.Warnf("restore tunnel service %q (id=%d) failed: %v", svc.Name, svc.ID, err)
			} else {
				result.Succeeded++
				log.Infof("restore tunnel service %q (id=%d) succeeded", svc.Name, svc.ID)
			}
		}
	}

	return result
}

// restoreWithRetry 使用指数退避重试来启动单个服务实例。
func (m *ServiceManagerImpl) restoreWithRetry(instance *port.ServiceInstance, cfg port.RestoreConfig) error {
	var lastErr error
	interval := cfg.InitialInterval
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1 // 至少尝试一次
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// 退避等待
			time.Sleep(interval)
			interval = time.Duration(float64(interval) * cfg.BackoffFactor)
		}

		if err := m.Start(instance); err != nil {
			lastErr = err
			log.Debugf("restore service %d attempt %d/%d failed: %v",
				instance.ID, attempt+1, maxRetries, err)
			continue
		}

		// 成功启动
		return nil
	}

	return fmt.Errorf("restore service %d after %d attempts: %w", instance.ID, maxRetries, lastErr)
}
