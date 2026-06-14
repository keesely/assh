# 设计原则与约束

## 1. 概述

本文档定义了 ASSH 项目的设计原则、架构约束和开发规范。所有开发工作必须遵循这些原则和约束。

## 2. 架构原则

### 2.1 SOLID 原则

| 原则 | 应用方式 | 示例 |
|------|----------|------|
| **单一职责 (SRP)** | 每个服务只负责一个业务领域 | `ServerService` 只管理服务器配置 |
| **开闭原则 (OCP)** | 通过接口扩展，不修改现有实现 | 新增存储实现只需实现接口 |
| **里氏替换 (LSP)** | 所有接口实现可互换 | `RSACryptor` 和 `AESCryptor` 可互换 |
| **接口隔离 (ISP)** | 端口接口小而专注 | `ServerRepository` 只定义必要方法 |
| **依赖倒置 (DIP)** | 服务依赖接口，不依赖实现 | `service` 依赖 `port`，不依赖 `infra` |

### 2.2 DDD 原则

| 原则 | 应用方式 |
|------|----------|
| **领域模型为核心** | 所有业务逻辑围绕领域模型展开 |
| **限界上下文** | 通过包结构划分不同领域 |
| **聚合设计** | Server 是聚合根，管理 Auth 生命周期 |
| **仓储模式** | 通过 Repository 接口访问持久化 |
| **领域事件** | ChangelogEntry 记录变更历史 |

### 2.3 六边形架构原则

```
┌─────────────────────────────────────────────────────────────┐
│                    六边形架构核心思想                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              端口 (Ports) 定义边界                     │  │
│  │  • 驱动端口：外部调用应用的入口                        │  │
│  │  • 被驱动端口：应用调用外部的出口                      │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              适配器 (Adapters) 实现接口                │  │
│  │  • 驱动适配器：CLI、API、GUI                          │  │
│  │  • 被驱动适配器：数据库、文件系统、外部服务            │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              应用核心 (Application Core)               │  │
│  │  • 领域层：实体、值对象、领域服务                      │  │
│  │  • 应用层：用例编排、事务管理                          │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## 3. 依赖约束

### 3.1 依赖方向

```
┌─────────────────────────────────────────────────────────────┐
│                    依赖方向规则                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   cmd/ ──────────────► service/ ◄────────────── infra/       │
│                             │                      ▲         │
│                             ▼                      │         │
│                          port/ ────────────────────┘         │
│                             │                                │
│                             ▼                                │
│                          domain/                             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 禁止的依赖

| 禁止的依赖 | 原因 |
|------------|------|
| `domain/` → `port/` | 领域层不应依赖接口层 |
| `domain/` → `infra/` | 领域层不应依赖基础设施 |
| `domain/` → `service/` | 领域层不应依赖应用层 |
| `port/` → `infra/` | 接口层不应依赖实现层 |
| `service/` → `infra/` | 服务层不应依赖基础设施 |
| `service/` → `cmd/` | 服务层不应依赖命令层 |

### 3.3 允许的依赖

| 允许的依赖 | 说明 |
|------------|------|
| `port/` → `domain/` | 接口使用领域实体 |
| `service/` → `port/` | 服务调用接口 |
| `service/` → `domain/` | 服务使用领域实体 |
| `infra/` → `port/` | 实现接口 |
| `infra/` → `domain/` | 使用领域实体 |
| `cmd/` → `service/` | 命令调用服务 |

## 4. 包结构约束

### 4.1 包命名规范

| 包 | 职责 | 命名规范 |
|----|------|----------|
| `domain/` | 领域实体、值对象、领域错误 | 小写、单数 |
| `port/` | 接口定义 | 小写、单数 |
| `service/` | 用例编排 | 小写、单数 |
| `infra/` | 基础设施实现 | 小写、复数（如 `store/`） |
| `cmd/` | CLI 命令 | 小写、单数 |

### 4.2 文件命名规范

| 文件类型 | 命名规范 | 示例 |
|----------|----------|------|
| 实体定义 | `{entity}.go` | `server.go` |
| 接口定义 | `{interface}.go` | `repository.go` |
| 服务实现 | `{service}.go` | `server.go` |
| 基础设施 | `{implementation}.go` | `sqlite.go` |
| 测试文件 | `{file}_test.go` | `server_test.go` |

## 5. 接口设计约束

### 5.1 接口命名规范

| 模式 | 示例 | 说明 |
|------|------|------|
| `{名词}Repository` | `ServerRepository` | 存储接口 |
| `{名词}Recorder` | `KnownServerRecorder` | 记录接口 |
| `{动词}er` | `Connector`, `Checker` | 行为接口 |
| `{名词}Manager` | `TunnelManager` | 管理接口 |
| `{名词}Service` | `ProxyService` | 服务接口 |

### 5.2 方法命名规范

| 模式 | 示例 | 说明 |
|------|------|------|
| `Get{Name}` | `GetServer` | 获取单个实体 |
| `List{Names}` | `ListServers` | 获取列表 |
| `Set{Name}` | `SetServer` | 创建或更新 |
| `Delete{Name}` | `DeleteServer` | 删除 |
| `Record{Event}` | `RecordDirectConnect` | 记录事件 |
| `Lookup{By}` | `LookupByAuth` | 按条件查找 |

### 5.3 接口设计原则

1. **接口隔离**：接口小而专注，只定义必要的方法
2. **稳定性**：接口变更需要谨慎评估影响范围
3. **可测试性**：接口易于 mock 测试

## 6. 错误处理约束

### 6.1 错误定义位置

| 错误类型 | 定义位置 | 示例 |
|----------|----------|------|
| 领域错误 | `domain/errors.go` | `ErrNotFound`, `ErrExists` |
| 服务错误 | 服务层 | `fmt.Errorf("failed to get server: %w", err)` |
| 基础设施错误 | 基础设施层 | `sql.ErrNoRows` |

### 6.2 错误处理模式

```go
// 领域层：定义错误
var ErrNotFound = errors.New("server not found")

// 服务层：返回领域错误
func (s *Service) GetServer(name string) (*domain.Server, error) {
    if name == "" {
        return nil, domain.ErrInvalidName
    }
    server, err := s.repo.Get(name)
    if err != nil {
        return nil, err
    }
    if server == nil {
        return nil, domain.ErrNotFound
    }
    return server, nil
}

// 命令层：处理错误
func (c *Command) HandleError(err error) {
    if errors.Is(err, domain.ErrNotFound) {
        fmt.Println("服务器不存在")
    } else {
        fmt.Printf("操作失败: %v\n", err)
    }
}
```

### 6.3 错误包装原则

1. **保留错误链**：使用 `fmt.Errorf("...: %w", err)` 包装
2. **添加上下文**：包装时添加操作上下文
3. **不暴露细节**：对外不暴露内部实现细节

## 7. 测试约束

### 7.1 测试分类

| 测试类型 | 位置 | 依赖 | 速度 |
|----------|------|------|------|
| 单元测试 | `{file}_test.go` | Mock | 快 |
| 集成测试 | `*_integration_test.go` | 真实依赖 | 慢 |

### 7.2 测试命名规范

```go
// 单元测试
func TestServerService_AddServer(t *testing.T) {
    // 测试用例
}

// 集成测试
func TestServerService_Integration_AddServer(t *testing.T) {
    // 集成测试用例
}
```

### 7.3 Mock 策略

```go
// Mock 接口
type MockServerRepository struct {
    mock.Mock
}

func (m *MockServerRepository) Get(name string) (*domain.Server, error) {
    args := m.Called(name)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*domain.Server), args.Error(1)
}

// 使用 Mock
func TestServerService_GetServer(t *testing.T) {
    mockRepo := new(MockServerRepository)
    mockRepo.On("Get", "test.server").Return(&domain.Server{
        Name: "test",
        Host: "1.2.3.4",
    }, nil)
    
    service := NewServerService(mockRepo)
    server, err := service.GetServer("test.server")
    
    assert.NoError(t, err)
    assert.Equal(t, "1.2.3.4", server.Host)
    mockRepo.AssertExpectations(t)
}
```

## 8. 并发约束

### 8.1 并发安全

1. **共享状态**：使用 `sync.Mutex` 或 `sync.RWMutex` 保护
2. **数据库连接**：SQLite 单写多读，使用连接池
3. **SSH 连接**：每个操作独立连接，避免共享

### 8.2 并发模式

```go
// 使用 Mutex 保护共享状态
type TunnelManager struct {
    tunnels map[string]*Tunnel
    mu      sync.RWMutex
}

func (m *TunnelManager) Get(id string) (*Tunnel, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    t, ok := m.tunnels[id]
    return t, ok
}

func (m *TunnelManager) Set(id string, t *Tunnel) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.tunnels[id] = t
}
```

## 9. 安全约束

### 9.1 敏感数据处理

| 数据类型 | 处理方式 |
|----------|----------|
| 密码 | AES-GCM 加密存储 |
| 密钥文件 | 文件路径存储，不存储内容 |
| 云账户凭据 | AES-GCM 加密存储 |
| SSH 会话 | 内存中，不持久化 |

### 9.2 加密规范

1. **存储加密**：AES-GCM 模式
2. **传输加密**：SSH 协议内置加密
3. **密钥管理**：通过环境变量或配置文件

### 9.3 输入校验

1. **所有输入必须校验**：包括用户输入和外部数据
2. **使用白名单**：只允许已知安全的输入
3. **长度限制**：防止缓冲区溢出

## 10. 性能约束

### 10.1 性能目标

| 操作 | 目标时间 |
|------|----------|
| 服务器列表 | < 100ms |
| SSH 连接 | < 5s |
| 文件传输 | 取决于网络 |
| 健康检查 | < 30s |

### 10.2 优化策略

1. **连接复用**：SSH 连接池
2. **并发操作**：健康检查并发执行
3. **缓存**：服务器配置缓存
4. **懒加载**：按需创建连接

## 11. 日志约束

### 11.1 日志级别

| 级别 | 使用场景 |
|------|----------|
| `DEBUG` | 调试信息、详细流程 |
| `INFO` | 正常操作、状态变更 |
| `WARN` | 警告信息、降级处理 |
| `ERROR` | 错误信息、异常情况 |

### 11.2 日志格式

```go
// 使用 zerolog
log.Info().
    Str("server", name).
    Int("port", port).
    Msg("Connecting to server")

log.Error().
    Err(err).
    Str("server", name).
    Msg("Failed to connect")
```

## 12. 文档约束

### 12.1 代码文档

1. **包注释**：每个包必须有包注释
2. **导出函数**：所有导出函数必须有注释
3. **复杂逻辑**：关键算法必须有注释

### 12.2 文档更新

1. **接口变更**：必须更新端口层文档
2. **架构变更**：必须更新架构文档
3. **新功能**：必须更新用户文档

## 13. 版本约束

### 13.1 版本号规范

遵循语义化版本（SemVer）：`v主版本.次版本.补丁`

| 版本号 | 含义 | 触发条件 |
|--------|------|----------|
| `v1.0.0` | 当前稳定版本 | Phase 完成 |
| `v1.0.1` | 补丁版本 | Bug 修复 |
| `v1.1.0` | 次版本 | 新功能（非破坏性） |
| `v2.0.0` | 主版本 | 破坏性变更 |

### 13.2 分支规范

| 分支类型 | 命名格式 | 示例 |
|----------|----------|------|
| 稳定分支 | `master` | `master` |
| 开发分支 | `dev/vx.0.0-phase-{N}` | `dev/v2.0.0-phase-12` |
| 特性分支 | `feat/{feature}` | `feat/config` |
| Bug 修复 | `bugfix/{issue}` | `bugfix/log-init-race` |

## 14. 相关文档

- [DDD 架构概览](./overview.md)
- [领域层设计](./domain-layer.md)
- [端口层设计](./port-layer.md)
- [服务层设计](./service-layer.md)
- [基础设施层设计](./infra-layer.md)
