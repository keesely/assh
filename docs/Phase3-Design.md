# Phase 3: SSH 连接层设计方案

> 日期: 2026-05-09
> 状态: 定稿
> ADR: ADR-011 (SSH 连接层架构设计)

---

## 1. 设计目标

实现 SSH 连接层，支持：
- SSH 连接建立与关闭（密码/私钥/Agent 三种认证方式）
- 交互式 Shell 终端（pty + SIGWINCH）
- 远程命令执行
- HostKey 可配置验证策略
- Keepalive 心跳可配置

---

## 2. 接口设计

### 2.1 port/connector.go — SSHConnector

```go
package port

import "assh/asshc/domain"

// SSHConnector 定义 SSH 连接管理能力
type SSHConnector interface {
    // Connect 根据服务器信息建立 SSH 连接
    // 认证顺序：Agent → 私钥 → 密码
    Connect(server *domain.Server) (*ssh.Client, error)
    
    // Close 关闭 SSH 连接
    Close(client *ssh.Client) error
}
```

### 2.2 port/session.go — SSHSession

```go
package port

// SSHSession 定义 SSH 会话操作
type SSHSession interface {
    // Shell 启动交互式终端
    // - 分配 pty（xterm-256color）
    // - 处理 SIGWINCH 窗口大小变化
    Shell(client *ssh.Client) error

    // Run 执行远程命令，输出到 stdout/stderr
    Run(client *ssh.Client, cmd string) error

    // RunWithOutput 执行远程命令，返回输出字符串
    RunWithOutput(client *ssh.Client, cmd string) (string, error)
}
```

### 2.3 port/hostkey.go — HostKeyChecker（预留）

```go
package port

import "golang.org/x/crypto/ssh"

// HostKeyCallback 定义主机密钥验证回调类型
// 与 golang.org/x/crypto/ssh 的 HostKeyCallback 兼容
type HostKeyCallback func(hostname string, remote net.Addr, key ssh.PublicKey) error

// HostKeyChecker 定义主机密钥安全检查接口
// Phase 3 仅定义接口，Phase 3 实现支持 known_hosts 和 skip 两种策略
// 后续 Phase 可通过此接口扩展自定义安全策略
type HostKeyChecker interface {
    // Check 检查主机密钥
    // 返回 nil 表示通过，返回 error 表示拒绝连接
    Check(hostname string, remote net.Addr, key ssh.PublicKey) error
}
```

---

## 3. 基础设施层实现

### 3.1 infra/ssh/client.go — SSH 客户端

实现 `port.SSHConnector` 接口，核心功能：

**认证方法链（按优先级从高到低）：**
1. **SSH Agent**: 检测 `SSH_AUTH_SOCK` 环境变量，优先尝试 agent 签名
2. **私钥文件**: 如果 `server.Auth.KeyFile` 非空，读取并解析私钥
3. **密码认证**: 如果 `server.Auth.Password` 非空，使用密码

**HostKey 策略：**
```go
func getHostKeyCallback(server *domain.Server) ssh.HostKeyCallback {
    if skip, _ := server.Options["insecure_skip_hostkey"]; skip == true {
        return ssh.InsecureIgnoreHostKey()
    }
    // TODO: known_hosts 检查 (Phase 3 迭代增强)
    return ssh.InsecureIgnoreHostKey() // Phase 3 初始版本默认跳过
}
```

**Keepalive 配置：**
```go
func getKeepaliveConfig(server *domain.Server) *KeepaliveConfig {
    interval := 0
    if v, ok := server.Options["keepalive"].(float64); ok {
        interval = int(v)
    }
    return &KeepaliveConfig{Interval: interval}
}
```

### 3.2 infra/ssh/session.go — 会话管理

实现 `port.SSHSession` 接口，核心功能：

**Shell 交互式终端：**
- `term.MakeRaw` 设置终端为原始模式
- `session.RequestPty` 请求 pty（xterm-256color, 14400 baud）
- `signal.Notify` 监听 `SIGWINCH`，更新窗口大小
- 退出时 `term.Restore` 恢复终端

**Run 远程命令：**
- `session.Run(cmd)` 执行命令
- `session.CombinedOutput(cmd)` 获取命令输出

### 3.3 infra/ssh/hostkey.go — HostKey 验证（预留）

实现 `port.HostKeyChecker` 接口。

Phase 3 初始版本提供两种策略：
1. **KnownHostsChecker**: 读取 `~/.ssh/known_hosts` 进行验证
2. **InsecureChecker**: 总是跳过验证

---

## 4. 服务层设计

### 4.1 service/connect.go — ConnectService

```go
package service

type ConnectService struct {
    connector port.SSHConnector
    session   port.SSHSession
    repo      port.ServerRepository
}

func NewConnectService(
    connector port.SSHConnector,
    session   port.SSHSession,
    repo      port.ServerRepository,
) *ConnectService

// 连接能力
func (s *ConnectService) ConnectByName(name string) (*ssh.Client, error)
func (s *ConnectService) ConnectDirect(host string, port int, user string, password string, keyFile string) (*ssh.Client, error)

// 会话能力（委托给 port.SSHSession）
func (s *ConnectService) Shell(client *ssh.Client) error
func (s *ConnectService) Run(client *ssh.Client, cmd string) error
func (s *ConnectService) RunWithOutput(client *ssh.Client, cmd string) (string, error)
```

**ConnectByName 流程：**
```
1. repo.Get(name) 获取 Server 实体
2. 如果未找到，返回 ErrNotFound
3. 调用 connector.Connect(server) 建立连接
4. 返回 *ssh.Client
```

**ConnectDirect 流程：**
```
1. 创建 domain.Server 对象（Host/Port/User/Auth）
2. 调用 connector.Connect(server) 建立连接
3. 返回 *ssh.Client
```

---

## 5. 连接流程

```
用户请求
  │
  ├── 按名称 → ConnectService.ConnectByName(name)
  │               ├── repo.Get(name) → domain.Server
  │               └── connector.Connect(server) → *ssh.Client
  │
  ├── 直连 → ConnectService.ConnectDirect(host, port, user, pass, key)
  │               └── connector.Connect(server) → *ssh.Client
  │
  └── 连接建立后
        ├── Shell(client)      → 交互终端
        ├── Run(client, cmd)   → 执行命令
        └── RunWithOutput(...) → 获取输出
```

**认证流程：**
```
Connect(server)
  │
  ├── 检查 SSH_AUTH_SOCK → 添加 Agent 认证
  ├── 检查 Auth.KeyFile  → 添加私钥认证
  ├── 检查 Auth.Password → 添加密码认证
  │
  ├── 读取 HostKey 策略
  │     ├── Options["insecure_skip_hostkey"]=true → InsecureIgnoreHostKey
  │     └── 默认 → KnownHosts 检查
  │
  ├── 读取 Keepalive 配置
  │     └── Options["keepalive"] → 设置心跳间隔
  │
  └── ssh.Dial("tcp", addr, sshCfg)
```

---

## 6. 配置项说明

### Server.Options 配置项

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `insecure_skip_hostkey` | bool | false | 跳过 host key 验证 |
| `keepalive` | int | 0 | 心跳间隔（秒），0 表示不启用 |

---

## 7. 文件清单与任务分解

### P3.1 接口层（port/）

| 任务 | 文件 | 说明 |
|------|------|------|
| P3.1-a | `asshc/port/connector.go` | SSHConnector 接口定义 |
| P3.1-b | `asshc/port/session.go` | SSHSession 接口定义 |
| P3.1-c | `asshc/port/hostkey.go` | HostKeyChecker 接口（预留） |

### P3.2 SSH 客户端（infra/ssh/）

| 任务 | 文件 | 说明 |
|------|------|------|
| P3.2-a | `asshc/infra/ssh/client.go` | 实现 SSHConnector（认证/连接/HostKey/Keepalive） |
| P3.2-b | `asshc/infra/ssh/session.go` | 实现 SSHSession（Shell/Run/RunWithOutput） |
| P3.2-c | `asshc/infra/ssh/hostkey.go` | HostKey 检查实现（known_hosts + insecure） |

### P3.3 测试文件

| 任务 | 文件 | 说明 |
|------|------|------|
| P3.3-a | `asshc/infra/ssh/client_test.go` | SSH 连接测试（mock ssh server） |
| P3.3-b | `asshc/infra/ssh/session_test.go` | 会话测试 |
| P3.3-c | `asshc/infra/ssh/hostkey_test.go` | HostKey 验证测试 |

### P3.4 服务层

| 任务 | 文件 | 说明 |
|------|------|------|
| P3.4-a | `asshc/service/connect.go` | ConnectService（连接 + 会话） |
| P3.4-b | `asshc/service/connect_test.go` | 服务层测试（mock） |

### P3.5 DI 注入

| 任务 | 文件 | 说明 |
|------|------|------|
| P3.5-a | `main.go` | 添加 SSHConnector/SSHSession/ConnectService 的 DI 注入 |

---

## 8. 验证策略

```
Phase 3 完成后执行：

# 编译验证
cd asshv2 && go build ./...

# 静态检查
cd asshv2 && go vet ./...

# 全量测试（跳过需要 SSH 连接的集成测试）
cd asshv2 && go test ./... -v -count=1

# 特定测试（含集成测试，需要 SSH server）
cd asshv2 && go test ./asshc/infra/ssh/... -v -count=1 -tags=integration
```

### 单元测试覆盖

- 认证方法链测试：Agent/私钥/密码三种认证方式的构建
- HostKey 策略测试：默认/跳过两种模式
- Shell pty 测试：终端参数设置
- Service 层 mock 测试：ConnectByName/ConnectDirect/Shell/Run
- 错误处理测试：连接失败、认证失败、超时等

---

## 9. 依赖情况

| 依赖 | 用途 | 状态 |
|------|------|------|
| `golang.org/x/crypto` v0.25.0 | SSH 协议（已在 go.mod 中） | ✅ 已有 |
| `golang.org/x/term` | 终端原始模式设置 | 📝 需确认在 go.mod 中 |
