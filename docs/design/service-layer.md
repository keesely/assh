# 服务层设计 (Service Layer)

## 1. 概述

服务层是应用的业务用例编排层，负责组合端口接口来完成具体的业务逻辑。该层不包含基础设施实现细节，只依赖端口接口和领域实体。

### 1.1 设计原则

- **用例驱动**：每个服务方法对应一个完整的业务用例
- **依赖倒置**：只依赖端口接口，不依赖基础设施实现
- **事务边界**：协调多个操作的事务一致性
- **错误转换**：将底层错误转换为业务语义错误

## 2. 服务职责

```
┌─────────────────────────────────────────────────────────────┐
│                    服务层职责                                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              用例编排 (Orchestration)                  │  │
│  │  • 组合多个端口完成业务流程                            │  │
│  │  • 定义业务操作的边界                                  │  │
│  │  • 协调事务一致性                                      │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              业务规则 (Business Rules)                 │  │
│  │  • 输入校验和验证                                      │  │
│  │  • 默认值设置                                          │  │
│  │  • 业务约束检查                                        │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              错误处理 (Error Handling)                 │  │
│  │  • 捕获底层异常                                        │  │
│  │  • 转换为业务错误                                      │  │
│  │  • 记录错误日志                                        │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## 3. 核心服务

### 3.1 ServerService

**位置**: `asshc/service/server.go`

服务器配置管理服务，是最核心的服务之一。

```go
type ServerService struct {
    repo port.ServerRepository
}

func NewServerService(repo port.ServerRepository) *ServerService {
    return &ServerService{repo: repo}
}
```

**主要方法**:

| 方法 | 职责 | 业务规则 |
|------|------|----------|
| `ListServers()` | 获取所有服务器 | 返回二级映射结构 |
| `GetServer(name)` | 获取单个服务器 | 名称非空校验 |
| `AddServer(name, server)` | 添加服务器 | 名称唯一性检查、默认值设置 |
| `UpdateServer(name, server)` | 更新服务器 | 服务器必须存在 |
| `SetServer(name, server)` | Upsert 操作 | 创建时校验必填、更新时保留版本 |
| `RemoveServer(name)` | 删除服务器 | 服务器必须存在 |
| `MoveServer(from, to)` | 移动/重命名 | 源必须存在 |
| `SearchServers(keyword)` | 搜索服务器 | 空关键字返回全部 |
| `RollbackServer(name, version)` | 版本回滚 | 版本号 >= 1 |
| `SuggestServer(input)` | 模糊建议 | 使用编辑距离算法 |

**业务流程示例**:

```
AddServer 流程:
┌─────────────────────────────────────────────────────────────┐
│ 1. 校验名称非空                                              │
│ 2. 检查服务器是否已存在                                       │
│ 3. 校验 Host 必填                                            │
│ 4. 设置默认值 (Port=22, User=root)                           │
│ 5. 解析分组信息                                              │
│ 6. 调用 repo.Set() 持久化                                    │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 ConnectService

**位置**: `asshc/service/connect.go`

SSH 连接管理服务。

```go
type ConnectService struct {
    connector port.SSHConnector
    session   port.SSHSession
}

func NewConnectService(connector port.SSHConnector, session port.SSHSession) *ConnectService {
    return &ConnectService{connector: connector, session: session}
}
```

**主要方法**:

| 方法 | 职责 | 业务规则 |
|------|------|----------|
| `Login(server)` | 交互式登录 | 建立连接 + 启动 Shell |
| `Run(server, cmd)` | 执行命令 | 建立连接 + 执行命令 + 返回输出 |
| `BatchRun(server, cmds)` | 批量执行 | 复用连接 + 顺序执行 |

**认证回退策略**:
```
┌─────────────────────────────────────────────────────────────┐
│                    认证回退流程                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. 尝试密钥认证 (KeyFile)                                   │
│     │                                                        │
│     ├── 成功 → 返回连接                                      │
│     │                                                        │
│     └── 失败 ↓                                               │
│                                                              │
│  2. 尝试密码认证 (Password)                                  │
│     │                                                        │
│     ├── 成功 → 返回连接                                      │
│     │                                                        │
│     └── 失败 → 返回错误                                      │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 3.3 TransferService

**位置**: `asshc/service/transfer.go`

文件传输服务。

```go
type TransferService struct {
    transfer port.FileTransfer
    connector port.SSHConnector
}

func NewTransferService(transfer port.FileTransfer, connector port.SSHConnector) *TransferService {
    return &TransferService{transfer: transfer, connector: connector}
}
```

**主要方法**:

| 方法 | 职责 | 业务规则 |
|------|------|----------|
| `PushFile(server, local, remote)` | 上传文件 | 建立连接 + 传输 + 关闭连接 |
| `PullFile(server, remote, local)` | 下载文件 | 建立连接 + 传输 + 关闭连接 |
| `ListRemote(server, path)` | 列出远程文件 | 建立连接 + 查询 + 关闭连接 |
| `GlobLocal(pattern)` | 本地文件通配 | 支持 glob 模式 |
| `PushGlob(server, pattern, remote)` | 批量上传 | 通配 + 循环上传 |

### 3.4 ProxyService

**位置**: `asshc/service/proxy.go`

代理服务编排。

```go
type ProxyService struct {
    tunnelMgr port.TunnelManager
    ruleEngine port.RuleEngine
    logger port.ProxyLogger
}

func NewProxyService(tunnelMgr port.TunnelManager, ruleEngine port.RuleEngine, logger port.ProxyLogger) *ProxyService {
    return &ProxyService{tunnelMgr: tunnelMgr, ruleEngine: ruleEngine, logger: logger}
}
```

**主要方法**:

| 方法 | 职责 | 业务规则 |
|------|------|----------|
| `StartSOCKS5(server, addr)` | 启动 SOCKS5 代理 | 建立隧道 + 启动监听 |
| `StartHTTP(server, addr)` | 启动 HTTP 代理 | 建立隧道 + 启动监听 |
| `StartSmartProxy(server, addr)` | 启动智能代理 | 规则匹配 + 自动路由 |
| `StopProxy(id)` | 停止代理 | 关闭隧道 + 释放资源 |

### 3.5 HealthService

**位置**: `asshc/service/health.go`

服务器健康检查服务。

```go
type HealthService struct {
    checker port.HealthChecker
    connector port.SSHConnector
    serverRepo port.ServerRepository
}

func NewHealthService(checker port.HealthChecker, connector port.SSHConnector, serverRepo port.ServerRepository) *HealthService {
    return &HealthService{checker: checker, connector: connector, serverRepo: serverRepo}
}
```

**主要方法**:

| 方法 | 职责 | 业务规则 |
|------|------|----------|
| `CheckServer(name)` | 单机检查 | 建立连接 + 采集信息 |
| `CheckGroup(group)` | 分组检查 | 并发检查 + 聚合结果 |
| `CheckAll()` | 全量检查 | 并发检查所有服务器 |

### 3.6 SyncService

**位置**: `asshc/service/sync.go`

云同步服务。

```go
type SyncService struct {
    syncer port.Syncer
    store port.SyncStore
    serverRepo port.ServerRepository
}

func NewSyncService(syncer port.Syncer, store port.SyncStore, serverRepo port.ServerRepository) *SyncService {
    return &SyncService{syncer: syncer, store: store, serverRepo: serverRepo}
}
```

**主要方法**:

| 方法 | 职责 | 业务规则 |
|------|------|----------|
| `Push()` | 推送本地到云端 | 读取本地 + 合并 + 上传 |
| `Pull()` | 拉取云端到本地 | 下载 + 合并 + 写入本地 |
| `TestConnection()` | 测试连接 | 验证云账户凭据 |

### 3.7 MountService

**位置**: `asshc/service/mount.go`

远程文件系统挂载服务。

```go
type MountService struct {
    mounter port.FileMounter
    mountStore port.MountStore
    connector port.SSHConnector
}

func NewMountService(mounter port.FileMounter, mountStore port.MountStore, connector port.SSHConnector) *MountService {
    return &MountService{mounter: mounter, mountStore: mountStore, connector: connector}
}
```

**主要方法**:

| 方法 | 职责 | 业务规则 |
|------|------|----------|
| `Mount(server, remotePath, localPath)` | 挂载远程目录 | 建立连接 + 挂载 + 记录 |
| `Unmount(mountPoint)` | 卸载 | 卸载 + 清理 |
| `ListMounts()` | 列出挂载点 | 查询持久化记录 |
| `RestoreAll()` | 恢复所有挂载 | 自动重连 + 重新挂载 |

## 4. 服务设计模式

### 4.1 构造函数注入

```go
type Service struct {
    dep1 port.Interface1
    dep2 port.Interface2
}

func NewService(dep1 port.Interface1, dep2 port.Interface2) *Service {
    return &Service{dep1: dep1, dep2: dep2}
}
```

### 4.2 用例编排模式

```go
func (s *Service) BusinessOperation(params) error {
    // 1. 输入校验
    if err := validate(params); err != nil {
        return err
    }
    
    // 2. 调用端口完成操作
    result, err := s.port.DoSomething(params)
    if err != nil {
        return domain.WrapError(err)
    }
    
    // 3. 后续处理
    return s.port.DoSomethingElse(result)
}
```

### 4.3 错误处理模式

```go
func (s *Service) GetServer(name string) (*domain.Server, error) {
    // 1. 领域校验
    if name == "" {
        return nil, domain.ErrInvalidName
    }
    
    // 2. 调用仓储
    server, err := s.repo.Get(name)
    if err != nil {
        // 3. 错误转换
        return nil, fmt.Errorf("failed to get server: %w", err)
    }
    
    // 4. 业务校验
    if server == nil {
        return nil, domain.ErrNotFound
    }
    
    return server, nil
}
```

## 5. 事务管理

### 5.1 当前实现

当前项目使用 SQLite 单连接，事务管理相对简单：

```go
func (s *Service) ComplexOperation() error {
    // SQLite 单连接，操作自动串行化
    s.repo.Set(name1, server1)
    s.repo.Set(name2, server2)
    return nil
}
```

### 5.2 未来扩展

如需支持分布式事务，可考虑：

1. **Saga 模式**：长事务拆分为多个本地事务
2. **TCC 模式**：Try-Confirm-Cancel 三阶段提交
3. **事件溯源**：通过领域事件实现最终一致性

## 6. 测试策略

### 6.1 单元测试

使用 mock 端口接口进行测试：

```go
func TestServerService_AddServer(t *testing.T) {
    // 创建 mock
    mockRepo := &MockServerRepository{}
    
    // 设置期望
    mockRepo.On("Get", "test.server").Return(nil, nil)
    mockRepo.On("Set", "test.server", mock.Anything).Return(nil)
    
    // 创建服务
    service := NewServerService(mockRepo)
    
    // 执行测试
    err := service.AddServer("test.server", &domain.Server{Host: "1.2.3.4"})
    
    // 验证结果
    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
}
```

### 6.2 集成测试

使用真实基础设施进行测试：

```go
func TestServerService_Integration(t *testing.T) {
    // 创建真实存储
    store, _ := infra.NewSQLiteStore(":memory:")
    defer store.Close()
    
    // 创建服务
    service := NewServerService(store)
    
    // 执行测试
    err := service.AddServer("test.server", &domain.Server{Host: "1.2.3.4"})
    assert.NoError(t, err)
    
    // 验证持久化
    server, err := service.GetServer("test.server")
    assert.NoError(t, err)
    assert.Equal(t, "1.2.3.4", server.Host)
}
```

## 7. 添加新服务

### 7.1 确定服务职责

1. 识别业务用例
2. 确定依赖的端口接口
3. 定义服务方法

### 7.2 实现步骤

1. 在 `service/` 包中创建新文件
2. 定义服务结构体和构造函数
3. 实现业务方法
4. 编写单元测试

### 7.3 示例

```go
// service/notification.go

package service

import (
    "assh/asshc/domain"
    "assh/asshc/port"
)

type NotificationService struct {
    notifier port.Notifier
    serverRepo port.ServerRepository
}

func NewNotificationService(notifier port.Notifier, serverRepo port.ServerRepository) *NotificationService {
    return &NotificationService{notifier: notifier, serverRepo: serverRepo}
}

func (s *NotificationService) NotifyServerDown(serverName string) error {
    server, err := s.serverRepo.Get(serverName)
    if err != nil {
        return err
    }
    
    event := &domain.NotificationEvent{
        Type: "server_down",
        Server: server,
        Message: fmt.Sprintf("Server %s is down", serverName),
    }
    
    return s.notifier.Notify(event)
}
```

## 8. 相关文档

- [DDD 架构概览](./overview.md)
- [领域层设计](./domain-layer.md)
- [端口层设计](./port-layer.md)
- [基础设施层设计](./infra-layer.md)
