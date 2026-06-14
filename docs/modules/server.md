# SSH 连接模块

**文件**: `asshv2/asshc/infra/ssh/`
**接口**: `asshc/port/connector.go` + `asshc/port/session.go` + `asshc/port/hostkey.go`

---

## 功能

基于 `golang.org/x/crypto/ssh` 实现 SSH 连接和会话管理：

| 功能 | 说明 |
|------|------|
| **SSH 连接** | 密码认证 + 私钥认证 + SSH Agent 认证 |
| **交互式终端** | pty 分配（xterm-256color）、SIGWINCH 窗口自适应 |
| **远程命令执行** | `Run()` 直接输出、`RunWithOutput()` 返回字符串 |
| **HostKey 验证** | 默认 known_hosts 检查，支持 `insecure_skip_hostkey` 跳过 |
| **Keepalive** | 从 Server.Options["keepalive"] 读取心跳间隔 |
| **连接关闭** | 安全关闭 SSH 连接 |

---

## 接口定义

### SSHConnector（连接管理）

```go
type SSHConnector interface {
    Connect(server *domain.Server) (*ssh.Client, error)
    Close(client *ssh.Client) error
}
```

### SSHSession（会话操作）

```go
type SSHSession interface {
    Shell(client *ssh.Client) error
    Run(client *ssh.Client, cmd string) error
    RunWithOutput(client *ssh.Client, cmd string) (string, error)
}
```

### HostKeyChecker（预留）

```go
type HostKeyChecker interface {
    Check(hostname string, remote net.Addr, key ssh.PublicKey) error
}
```

---

## 连接流程

```
Connect(server)
  ├── 1. 构建认证方法链
  │     ├── SSH Agent (SSH_AUTH_SOCK)
  │     ├── 私钥文件 (KeyFile)
  │     └── 密码 (Password)
  ├── 2. 确定 HostKey 策略
  │     ├── insecure_skip_hostkey=true → 跳过
  │     └── 默认 → known_hosts 检查
  ├── 3. 读取 Keepalive 配置
  │     └── Options["keepalive"] → 心跳间隔
  └── 4. ssh.Dial("tcp", addr, cfg)
```

---

## 会话操作

### Shell 交互终端
- `term.MakeRaw` → 原始模式
- `session.RequestPty("xterm-256color", h, w, modes)` → pty
- `signal.Notify(sigCh, syscall.SIGWINCH)` → 窗口变化

### Run 命令执行
- `session.Run(cmd)` → 直接输出到 stdout/stderr
- `session.CombinedOutput(cmd)` → 返回字符串

---

## 配置项

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `insecure_skip_hostkey` | bool | false | 跳过 host key 验证 |
| `keepalive` | int | 0 | 心跳间隔（秒），0 表示不启用 |

---

## 依赖

- `golang.org/x/crypto` v0.25.0 — SSH 协议库
- `golang.org/x/term` — 终端原始模式设置

## 相关文档

- [Phase 3 设计方案](../Phase3-Design.md)
- [ADR-011: SSH 连接层架构设计](../adrs/017-SSH连接层架构设计.md)
