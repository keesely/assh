# Phase 7: 代理 & 隧道设计文档

## 概述

实现 SSH 代理和隧道功能，包括 SOCKS5 代理、本地端口转发、远程端口转发和 SSH 隧道管理。用户可通过 `assh proxy` 和 `assh tunnel` 两个独立命令组使用。

## 设计目标

1. **SOCKS5 代理** — 支持标准 SOCKS5 协议（RFC 1928），可通过 SSH 建立加密代理通道
2. **端口转发** — 支持 `-L` 本地转发和 `-R` 远程转发（与 ssh 命令对齐）
3. **隧道管理** — 支持多隧道生命周期管理（启动/停止/列表/重连）
4. **后台驻留** — 支持 `--daemon` 参数，Fork + PID 文件方式后台运行
5. **智能重连** — 支持 `--auto-reconnect` 参数或服务器配置控制 SSH 断开重连

## 接口层 (`asshc/port/proxy.go`)

```go
// Proxy 定义 SOCKS5 代理接口（RFC 1928）
type Proxy interface {
    Start(client *ssh.Client, localAddr string) error  // 启动 SOCKS5 代理
    Stop() error                                         // 停止代理
    Addr() string                                        // 返回监听地址
}

// PortForward 定义端口转发接口（local 和 remote）
type PortForward interface {
    ID() string                // 唯一标识符
    Type() string              // "local" 或 "remote"
    LocalAddr() string         // 本地地址
    RemoteAddr() string        // 远程地址
    Start(client *ssh.Client) error  // 启动转发
    Stop() error                     // 停止转发
}

// TunnelManager 定义隧道管理器接口
type TunnelManager interface {
    Add(forward PortForward) error         // 添加端口转发
    Remove(id string) error                // 移除并停止
    Get(id string) (PortForward, error)    // 获取指定转发
    List() []PortForward                    // 列出所有转发
    StopAll() error                         // 停止所有转发
}
```

### 设计决策

- **SSH 客户端生命周期**：`Start` 方法接收 `*ssh.Client` 参数而非构造时注入，SSH 客户端由调用方（Service 层）管理。这使得：
  - 同一个 SOCKS5Proxy 实例可以复用不同 SSH 连接
  - SSH 连接的创建、重连、关闭由 Service 层统一控制
  - 重连时无需重建 Proxy 实例

## 基础设施实现 (`asshc/infra/proxy/`)

```
asshc/infra/proxy/
├── socks5.go          # SOCKS5 协议实现（RFC 1928）
├── port.go            # LocalForward / RemoteForward 端口转发
└── tunnel_manager.go  # TunnelManager 实现（待实现 P7.4）
```

### SOCKS5 代理 (`socks5.go`)

遵循 RFC 1928 标准实现：

#### 握手流程

```
Client → Server:  | ver=5 | nmethods | methods... |
Server → Client:  | ver=5 | method=X |
```

支持的认证方式：
| 方法值 | 认证方式 | 状态 |
|--------|----------|:----:|
| 0x00 | NO AUTH（无认证） | ✅ 已实现 |
| 0x02 | USERNAME/PASSWORD | ⏳ 待实现（P7.2-a 迭代） |

#### 请求与转发

```
Client → Server:  | ver=5 | cmd=1(CONNECT) | rsv=0 | atyp | dst.addr | dst.port |
Server → Client:  | ver=5 | rep=0(SUCCESS) | rsv=0 | atyp | bnd.addr | bnd.port |
```

支持的地址类型：
| ATYP | 类型 | 状态 |
|------|------|:----:|
| 0x01 | IPv4 | ✅ 已实现 |
| 0x03 | 域名 | ✅ 已实现 |
| 0x04 | IPv6 | ✅ 已实现 |

仅支持 CONNECT 命令（TCP 转发），不支持 BIND 和 UDP ASSOCIATE。

#### 数据转发

SOCKS5 握手完成后，建立从本地连接到远程目标的 TCP 转发通道：

```
[Client App] ←→ [Local :1080] ←SSH→ [Remote] ←→ [Target Host:Port]
```

数据流通过 SSH 双向拷贝（`bidirectionalCopy` — 对称 `io.Copy` goroutine）。

#### USERNAME/PASSWORD 认证（待实现）

将现有的 `socksHandshake()` 从仅 NO AUTH 扩展为支持协商选择认证方式：

```go
func (p *SOCKS5Proxy) socksHandshake(conn net.Conn) error {
    // 1. 读取客户端支持的认证方法列表
    // 2. 按优先级选择认证方法：USERNAME/PASSWORD(0x02) > NO AUTH(0x00)
    // 3. 若选择 USERNAME/PASSWORD：
    //    a. 发送 0x02 选择
    //    b. 执行子协商（RFC 1929）：读取用户名密码 → 验证
    //    c. 验证失败返回 0xFF，断开连接
    // 4. 若客户端不支持任一方法，返回 0xFF 拒绝
}
```

认证回调接口将通过 `SOCKS5Proxy` 字段注入：
```go
type SOCKS5Proxy struct {
    // ... 现有字段
    authFunc func(username, password string) bool  // 认证回调（nil=仅 NO AUTH）
}
```

### 本地端口转发 — LocalForward (`port.go`)

等价于 `ssh -L [local_addr:]local_port:remote_host:remote_port`

```
本地监听: net.Listen("tcp", localAddr)
   ↓ 新连接到达
SSH Dial: client.Dial("tcp", remoteAddr)
   ↓
双向拷贝: bidirectionalCopy(localConn, remoteConn)
```

### 远程端口转发 — RemoteForward (`port.go`)

等价于 `ssh -R [remote_addr:]remote_port:local_host:local_port`

```
SSH 请求: client.Listen("tcp", remoteAddr)  // 发送 tcpip-forward 全局请求
   ↓ 远程连接到达
本地 Dial: net.Dial("tcp", localAddr)
   ↓
双向拷贝: bidirectionalCopy(localConn, remoteConn)
```

### 隧道管理器 — TunnelManager（待实现 P7.4）

```go
type tunnelManager struct {
    mu       sync.Mutex
    forwards map[string]PortForward
    sshClient *ssh.Client        // 当前 SSH 连接
    reconnect bool               // 是否启用自动重连
    reconnectCfg *ReconnectConfig // 重连配置
}

type ReconnectConfig struct {
    MaxRetries int           // 最大重试次数（0=不限制）
    Interval   time.Duration // 重试间隔（默认 5s）
    Backoff    time.Duration // 退避增量（默认 5s）
}
```

## 服务编排层 (`asshc/service/proxy.go` — 待实现 P7.5)

```go
type ProxyService struct {
    repo        port.ServerRepository
    connector   port.SSHConnector
    socks5Proxy port.Proxy
    tunnelMgr   port.TunnelManager
}

// SOCKS5 代理
func (s *ProxyService) StartSOCKS5(name string, localAddr string, opts ProxyOptions) error
func (s *ProxyService) StopSOCKS5() error

// 端口转发
func (s *ProxyService) AddLocalForward(name string, localAddr, remoteAddr string) (id string, error)
func (s *ProxyService) AddRemoteForward(name string, remoteAddr, localAddr string) (id string, error)
func (s *ProxyService) RemoveForward(id string) error
func (s *ProxyService) ListForwards() []port.PortForward

// 批量操作
func (s *ProxyService) StartBatchForward(name string, forwards []ForwardSpec, opts ProxyOptions) error
```

### 后台驻留策略

| 模式 | 触发条件 | 行为 |
|------|----------|------|
| **前台阻塞** | 默认/无 `--daemon` | 启动后主 goroutine 等待信号（SIGINT/SIGTERM），收到后清理退出 |
| **后台守护** | 指定 `--daemon` | Fork 子进程 → 写入 PID 文件 → 父进程退出；`assh tunnel stop` 读取 PID kill |
| **无驻留** | 指定 `--oneshot` | 启动后立即返回（仅适用于无需持续连接的操作） |

PID 文件路径：`~/.assh/v2/tunnels/{id}.pid`

### 重连策略

| 控制方式 | 优先级 | 说明 |
|----------|--------|------|
| `--auto-reconnect` CLI 参数 | 最高 | 临时覆盖配置，格式 `--auto-reconnect=3/5s`（重试3次，间隔5s） |
| Server.Options["auto_reconnect"] | 中等 | 按服务器配置执行重连 |
| 默认 | 最低 | 不重连，断开即停 |

重连流程：
1. SSH 连接断开 → 记录日志
2. 若 `reconnect` 启用 → 进入重连循环
3. 每次重连前检查 `ctx.Done()`（主进程退出标记）
4. 重连成功 → 自动重启所有隧道/代理
5. 达到重试上限 → 停止所有隧道，记录错误

### 直连支持

| 命令 | 直连支持 | 说明 |
|------|:--------:|------|
| `proxy` | ✅ | 支持 `-H/-u/-P/-i/-k` 参数，无需提前保存服务器 |
| `tunnel start` | ❌ | 仅操作已保存在数据库中的服务器 |

## CLI 命令设计

### `proxy` 命令

智能代理入口，支持正向和反向两种模式。

#### 正向模式（默认）

```bash
assh proxy myserver                                          # SOCKS5 :1080（默认）
assh proxy myserver --socks5 :1080                           # 显式指定 SOCKS5 地址
assh proxy myserver --http :8080                             # 叠加 HTTP CONNECT
assh proxy myserver --auth user:pass                         # SOCKS5 用户名认证
assh proxy myserver --daemon                                 # 后台守护模式
assh proxy myserver --auto-reconnect=5/10s                   # 重连 5 次，间隔 10s

# 直连模式（无需数据库）
assh proxy -H 192.168.1.1 -u root -P mypass                  # 默认 :1080
assh proxy -H example.com --daemon                           # 后台 + 直连

# Smart Proxy 增强（规则+日志+多端口）
assh proxy myserver --rule ./gfwlist.txt                     # 加载 AutoProxy 规则
assh proxy myserver --log-dir ./proxy-logs                   # 启用透明日志
assh proxy myserver --port-range 80-84:81-85                 # 多端口映射规则
assh proxy myserver --port-range 443:443 \
                    --port-range 80-84:81-85                 # 多组规则
```

#### 反向模式

```bash
assh proxy myserver --reverse                                # 反向代理模式
assh proxy myserver --reverse \
  --port-range 9090:localhost:3000                           # server:9090 → 本地:3000
assh proxy myserver --reverse \
  --port-range 80-84:81-85                                   # 多组反向映射
assh proxy myserver --reverse \
  --rule ./gfwlist.txt --log-dir ./logs                      # 反向+规则+日志
```

#### 管理子命令

```bash
assh proxy rule reload                                       # 热重载规则
assh proxy rule status                                       # 查看规则状态
assh proxy log <id>                                          # 查看会话日志
```

参数规格：

| 参数 | 别名 | 类型 | 默认值 | 说明 |
|------|------|------|--------|------|
| `--socks5` | | string | `:1080` | SOCKS5 监听地址（空=不启用） |
| `--http` | | string | - | HTTP CONNECT 监听地址（空=不启用） |
| `--reverse` | | bool | false | 反向代理模式 |
| `--port-range` | | string(s) | - | 端口映射规则，可多次指定 |
| `--rule` | | string | - | AutoProxy 规则文件路径 |
| `--log-dir` | | string | - | 代理日志目录 |
| `--auth` | | string | - | 用户名密码认证（格式 `user:pass`） |
| `--daemon` | `-d` | bool | false | 后台守护模式 |
| `--auto-reconnect` | | string | - | 自动重连配置，格式 `[retries/]interval` |
| `--host` | `-H` | string | - | 直连：服务器地址 |
| `--user` | `-u`/`-l` | string | - | 直连：用户名 |
| `--password` | `-P` | string | - | 直连：密码 |
| `--identity-file` | `-i`/`-k` | string | - | 直连：密钥文件 |

### `tunnel` 命令组

管理端口转发隧道：

```bash
# 启动本地端口转发（支持多个 -L）
assh tunnel start myserver -L 8080:localhost:80
assh tunnel start myserver -L 0.0.0.0:8080:web:80
assh tunnel start myserver -L 8080:svc1:80 -L 9090:svc2:90   # 多转发

# 启动远程端口转发（支持多个 -R）
assh tunnel start myserver -R 8080:localhost:3000
assh tunnel start myserver -R 0.0.0.0:8080:local:3000

# 混合 -L 和 -R
assh tunnel start myserver -L 8080:localhost:80 -R 9090:localhost:3000

# 带守护和重连参数
assh tunnel start myserver -L 8080:localhost:80 --daemon
assh tunnel start myserver -L 8080:localhost:80 --auto-reconnect=3/5s

# 停止隧道
assh tunnel stop <id>

# 列出活跃隧道
assh tunnel list
```

参数规格：

| 参数 | 别名 | 类型 | 默认值 | 说明 |
|------|------|------|--------|------|
| `-L` | | string(s) | - | 本地转发规格 `[bind_addr:]port:host:hostport`，可多次指定 |
| `-R` | | string(s) | - | 远程转发规格 `[bind_addr:]port:host:hostport`，可多次指定 |
| `--daemon` | `-d` | bool | false | 后台守护模式 |
| `--auto-reconnect` | | string | - | 自动重连配置，与 proxy 一致 |

### 端口转发规格解析

`[bind_addr:]port:host:hostport` 格式：

| 写法 | bind_addr | port | host | hostport |
|------|-----------|------|------|----------|
| `8080:localhost:80` | 127.0.0.1 | 8080 | localhost | 80 |
| `0.0.0.0:8080:web:80` | 0.0.0.0 | 8080 | web | 80 |
| `127.0.0.1:8443:svc:443` | 127.0.0.1 | 8443 | svc | 443 |

## 数据流图

```
┌─────────────────────────────────────────┐
│            CLI Layer (cmd/)             │
│  ┌──────────────────┐  ┌─────────────┐  │
│  │ cmd/proxy.go     │  │ cmd/tunnel.go│  │
│  │  proxy subcmd    │  │  tunnel     │  │
│  └────────┬─────────┘  │  ├─ start   │  │
│           │            │  ├─ stop    │  │
│           │            │  └─ list    │  │
│           │            └──────┬──────┘  │
└───────────┼───────────────────┼─────────┘
            │                   │
┌───────────▼───────────────────▼─────────┐
│         Service Layer (asshc)           │
│  ┌────────────────────────────────┐     │
│  │ ProxyService                   │     │
│  │  - StartSOCKS5                 │     │
│  │  - StopSOCKS5                  │     │
│  │  - AddLocalForward             │     │
│  │  - AddRemoteForward            │     │
│  │  - RemoveForward               │     │
│  │  - ListForwards                │     │
│  │  - StartBatchForward           │     │
│  └──────────┬─────────────────────┘     │
└─────────────┼───────────────────────────┘
              │
┌─────────────▼───────────────────────────┐
│         Infra Layer (asshc)             │
│  ┌──────────────────────────────┐       │
│  │ infra/proxy/                 │       │
│  │  ├── socks5.go               │       │
│  │  │   SOCKS5Proxy             │       │
│  │  ├── port.go                 │       │
│  │  │   LocalForward            │       │
│  │  │   RemoteForward           │       │
│  │  └── tunnel_manager.go       │       │
│  │      TunnelManager           │       │
│  └──────────┬───────────────────┘       │
│             │                            │
│  ┌──────────▼───────────────────┐       │
│  │ infra/ssh/client.go          │       │
│  │ SSHConnector (连接管理)       │       │
│  └──────────────────────────────┘       │
└─────────────────────────────────────────┘
```

## 生命周期管理

```
User executes:
  assh proxy myserver --daemon --auto-reconnect=3/5s
                        │
                        ▼
  1. ProxyService.StartSOCKS5("myserver", ":1080", opts)
                        │
                        ├── 2. repo.Get("myserver") → domain.Server
                        ├── 3. connector.Connect(server) → *ssh.Client
                        ├── 4. socks5Proxy.Start(client, ":1080")
                        │       └── go acceptLoop(client)
                        │
                        ├── 5. [--daemon] Fork + write PID file → exit(0)
                        │
                        └── 6. [前台模式] Wait for signal (SIGINT/SIGTERM)
                           7. [重连]  SSH 断开 → reconnect loop
                           8. 信号到达 → StopSOCKS5 → StopProxy → cleanup
```

## 错误处理策略

| 场景 | 处理方式 |
|------|----------|
| 监听端口占用 | 返回友好错误 "port 1080 already in use" |
| SSH 连接断开（无重连） | 停止所有活跃隧道，打印错误，进程退出 |
| SSH 连接断开（有重连） | 进入重连循环，达上限后停止 |
| 数据转发中断 | 单个连接断开不影响其他连接 |
| SOCKS5 协议错误 | 关闭客户端连接，记录 Debug 日志 |
| SOCKS5 认证失败 | 返回 0xFF 拒绝，关闭连接 |
| 多个 -L/-R 中个别失败 | 继续启动其余转发，汇总报告 |

## 依赖关系

```
port.Proxy / port.PortForward / port.TunnelManager — 零依赖（仅 ssh 类型引用）
    ↑
infra/proxy/ — 依赖 port.* 接口 + golang.org/x/crypto/ssh
    ↑
service/proxy.go — 依赖 port.ServerRepository + port.SSHConnector + port.Proxy + port.TunnelManager
    ↑
cmd/proxy.go + cmd/tunnel.go — 依赖 service.ProxyService
    ↑
main.go (DI 组合根) — 创建所有依赖并注入
```

## 待实现功能摘要

| 编号 | 任务 | 文件 | 优先级 |
|------|------|------|:------:|
| P7.2-a-2 | SOCKS5 USERNAME/PASSWORD 认证扩展 | `infra/proxy/socks5.go` | 🟡 中 |
| P7.4 | TunnelManager 实现 | `infra/proxy/tunnel_manager.go` | 🔴 高 |
| P7.5 | ProxyService 编排层 | `service/proxy.go` | 🔴 高 |
| P7.2-b | proxy CLI 命令 | `cmd/proxy.go` | 🔴 高 |
| P7.6 | tunnel CLI 命令 | `cmd/tunnel.go` | 🔴 高 |
| P7.7 | 单元测试 | `*_test.go` | 🔴 高 |
| P7.8 | DI 注入 | `main.go` + `cmd/app.go` | 🔴 高 |
| - | Fork + PID 文件 daemon 模式 | `service/proxy.go` + 工具函数 | 🟡 中 |
| - | 重连逻辑 | `infra/proxy/tunnel_manager.go` | 🟡 中 |

## 参考资料

- [RFC 1928 — SOCKS Protocol Version 5](https://datatracker.ietf.org/doc/html/rfc1928)
- [RFC 1929 — Username/Password Authentication for SOCKS V5](https://datatracker.ietf.org/doc/html/rfc1929)
- [ADR-028: 代理 & 隧道架构设计](../adrs/028-proxy-tunnel-architecture.md)
- Phase 7 初始化工作日志：`../../worklogs/2026-05-13-Phase7-Init.md`
