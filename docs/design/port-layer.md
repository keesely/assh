# 端口层设计 (Port Layer)

## 1. 概述

端口层定义了应用的业务接口，是实现**依赖倒置原则**的关键。该层只包含接口定义和类型引用，不包含任何实现代码。

### 1.1 设计原则

- **接口隔离**：每个接口小而专注，只定义必要的方法
- **依赖倒置**：高层模块定义接口，低层模块实现接口
- **稳定性**：接口变更需要谨慎评估影响范围

## 2. 端口分类

### 2.1 驱动端口 (Driving Ports)

驱动端口由外部调用，定义了应用的入口点。

```
┌─────────────────────────────────────────────────────────────┐
│                    驱动端口                                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              连接管理端口                              │  │
│  │  • SSHConnector    - 建立 SSH 连接                     │  │
│  │  • SSHSession      - 管理 SSH 会话                     │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              文件操作端口                              │  │
│  │  • FileTransfer    - 文件传输                          │  │
│  │  • FileMounter     - 文件系统挂载                      │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              服务管理端口                              │  │
│  │  • ServiceManager  - 后台服务管理                      │  │
│  │  • TunnelManager   - 隧道管理                          │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 被驱动端口 (Driven Ports)

被驱动端口由应用调用，定义了应用对基础设施的需求。

```
┌─────────────────────────────────────────────────────────────┐
│                    被驱动端口                                │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              存储端口                                  │  │
│  │  • ServerRepository      - 服务器配置存储              │  │
│  │  • KnownServerRecorder   - 连接历史存储                │  │
│  │  • MountStore            - 挂载配置存储                │  │
│  │  • SyncStore             - 同步状态存储                │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              协议端口                                  │  │
│  │  • Syncer            - 云同步服务                      │  │
│  │  • HealthChecker     - 健康检查服务                    │  │
│  │  • RuleEngine        - 代理规则引擎                    │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              加密端口                                  │  │
│  │  • Cryptor           - 加解密服务                      │  │
│  │  • HostKeyChecker    - 主机密钥校验                    │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## 3. 核心端口接口

### 3.1 ServerRepository

**位置**: `asshc/port/repository.go`

服务器配置的持久化存储接口。

```go
type ServerRepository interface {
    // 基础 CRUD
    List() (map[string]map[string]*domain.Server, error)
    Get(name string) (*domain.Server, error)
    Set(name string, server *domain.Server) error
    Delete(name string) error
    Move(from, to string) error
    
    // 查询
    Search(keyword string) (map[string]map[string]*domain.Server, error)
    GetGroup(group string) (map[string]*domain.Server, error)
    
    // 版本管理
    GetChangelog(name string) ([]domain.ChangelogEntry, error)
    RollbackTo(name string, version int) error
    
    // 生命周期
    Close() error
}
```

**设计要点**:
- 返回类型使用领域实体，不使用 DTO
- `Set` 方法实现 upsert 语义
- `List` 返回二级映射结构（分组 -> 名称 -> Server）
- 版本管理支持变更历史和回滚

### 3.2 KnownServerRecorder

**位置**: `asshc/port/repository.go`

直连历史记录接口。

```go
type KnownServerRecorder interface {
    RecordDirectConnect(ks *domain.KnownServer) error
    LookupKnownServer(id string) (*domain.KnownServer, error)
    LookupKnownServerByAuth(user, host string, port int, authFingerprint string) (*domain.KnownServer, error)
    UpdateKeyBackupPath(id, keyBackupPath string) error
    DeleteKnownServer(id string) error
    ListKnownServers() ([]*domain.KnownServer, error)
}
```

**设计要点**:
- 支持多种查找方式（ID、认证指纹）
- `RecordDirectConnect` 实现 upsert 语义（存在则递增计数）

### 3.3 SSHConnector

**位置**: `asshc/port/connector.go`

SSH 连接管理接口。

```go
type SSHConnector interface {
    Connect(server *domain.Server) (*ssh.Client, error)
    Close(client *ssh.Client) error
}
```

**设计要点**:
- 接收领域实体作为参数
- 返回标准库 `ssh.Client`
- 连接过程包括认证、HostKey 校验、Keepalive

### 3.4 SSHSession

**位置**: `asshc/port/session.go`

SSH 会话操作接口。

```go
type SSHSession interface {
    Shell(client *ssh.Client) error
    Run(client *ssh.Client, cmd string) ([]byte, []byte, error)
    Close() error
}
```

**设计要点**:
- `Run` 返回 stdout 和 stderr
- `Shell` 启动交互式会话

### 3.5 FileTransfer

**位置**: `asshc/port/transfer.go`

文件传输接口。

```go
type FileTransfer interface {
    Push(client *ssh.Client, localPath, remotePath string) error
    Pull(client *ssh.Client, remotePath, localPath string) error
    List(client *ssh.Client, remotePath string) ([]FileInfo, error)
    Remove(client *ssh.Client, remotePath string) error
    Mkdir(client *ssh.Client, remotePath string) error
}
```

**设计要点**:
- 所有操作需要 SSH 客户端参数
- 返回自定义 `FileInfo` 类型

### 3.6 Cryptor

**位置**: `asshc/port/cryptor.go`

加解密服务接口。

```go
type Cryptor interface {
    Encrypt(plaintext []byte) ([]byte, error)
    Decrypt(ciphertext []byte) ([]byte, error)
}
```

**设计要点**:
- 简单的字节数组操作
- 实现可选择不同算法（RSA/AES）

### 3.7 TunnelManager

**位置**: `asshc/port/proxy.go`

SSH 隧道管理接口。

```go
type TunnelManager interface {
    Start(tunnel *domain.Tunnel) error
    Stop(id string) error
    List() ([]*domain.Tunnel, error)
    RestoreAll() error
}
```

**设计要点**:
- 支持隧道的完整生命周期管理
- `RestoreAll` 用于应用启动时恢复所有隧道

### 3.8 ServiceManager

**位置**: `asshc/port/servicemgr.go`

后台服务管理接口。

```go
type ServiceManager interface {
    Start(service *domain.Service) error
    Stop(id string) error
    List() ([]*domain.Service, error)
    RestoreAll() error
}
```

**设计要点**:
- 统一管理各类后台服务（代理、隧道等）
- 支持服务状态持久化和恢复

## 4. 端口设计模式

### 4.1 接口命名规范

| 模式 | 示例 | 说明 |
|------|------|------|
| `{名词}Repository` | `ServerRepository` | 存储接口 |
| `{名词}Recorder` | `KnownServerRecorder` | 记录接口 |
| `{动词}er` | `Connector`, `Checker` | 行为接口 |
| `{名词}Manager` | `TunnelManager` | 管理接口 |
| `{名词}Service` | `ProxyService` | 服务接口 |

### 4.2 方法命名规范

| 模式 | 示例 | 说明 |
|------|------|------|
| `Get{Name}` | `GetServer` | 获取单个实体 |
| `List{Names}` | `ListServers` | 获取列表 |
| `Set{Name}` | `SetServer` | 创建或更新 |
| `Delete{Name}` | `DeleteServer` | 删除 |
| `Record{Event}` | `RecordDirectConnect` | 记录事件 |
| `Lookup{By}` | `LookupByAuth` | 按条件查找 |

### 4.3 错误处理

```go
// 返回领域错误
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
```

## 5. 添加新端口

### 5.1 确定端口类型

1. **驱动端口**：定义应用的入口点（如 CLI 命令）
2. **被驱动端口**：定义应用对基础设施的需求（如存储、外部服务）

### 5.2 设计接口

1. 在 `port/` 包中创建新文件
2. 定义接口，保持小而专注
3. 使用领域实体作为参数和返回值
4. 添加清晰的文档注释

### 5.3 示例

```go
// port/notifier.go

package port

import "assh/asshc/domain"

// Notifier 定义通知服务接口。
// 用于在特定事件发生时发送通知（如连接失败、健康检查异常）。
type Notifier interface {
    // Notify 发送通知。
    Notify(event *domain.NotificationEvent) error
    
    // NotifyWithRetry 发送通知，失败时重试。
    NotifyWithRetry(event *domain.NotificationEvent, maxRetries int) error
}
```

## 6. 端口与适配器映射

| 端口接口 | 适配器实现 | 位置 |
|----------|------------|------|
| `ServerRepository` | `SQLiteStore` | `infra/store/` |
| `KnownServerRecorder` | `SQLiteStore` | `infra/store/` |
| `SSHConnector` | `SSHClient` | `infra/ssh/` |
| `SSHSession` | `SSHClient` | `infra/ssh/` |
| `FileTransfer` | `SFTPTransfer` | `infra/sftp/` |
| `Cryptor` | `RSACryptor`, `AESCryptor` | `infra/crypto/` |
| `TunnelManager` | `TunnelManagerImpl` | `infra/tunnel/` |
| `ServiceManager` | `ServiceManagerImpl` | `infra/proxy/` |
| `Syncer` | `QiniuSyncer` | `infra/sync/qiniu/` |
| `FileMounter` | `SSHFSMounter`, `WebDAVMounter` | `infra/mount/` |

## 7. 相关文档

- [DDD 架构概览](./overview.md)
- [领域层设计](./domain-layer.md)
- [服务层设计](./service-layer.md)
- [基础设施层设计](./infra-layer.md)
