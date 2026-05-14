# Phase 7: 代理 & 隧道设计文档

## 概述

实现 SSH 代理和隧道功能，包括 SOCKS5 代理、本地端口转发、远程端口转发和 SSH 隧道管理。用户可通过 `assh proxy` 和 `assh tunnel` 命令使用。

## 设计目标

1. **SOCKS5 代理** — 支持标准 SOCKS5 协议（RFC 1928），可通过 SSH 建立加密代理通道
2. **端口转发** — 支持 `-L` 本地转发和 `-R` 远程转发（与 ssh 命令对齐）
3. **隧道管理** — 支持多隧道生命周期管理（启动/停止/列表/重连）
4. **后台驻留** — 隧道进程需持续驻留，不能阻塞 CLI 退出

## 架构设计

### port/ 接口层

```go
// asshc/port/proxy.go

// Proxy 定义 SOCKS5 代理接口
type Proxy interface {
    Start(localAddr string) error    // 启动 SOCKS5 代理
    Stop() error                      // 停止代理
    Addr() string                     // 返回监听地址
}

// PortForward 定义端口转发接口
type PortForward interface {
    Start() error                     // 启动转发
    Stop() error                      // 停止转发
    Type() string                     // "local" 或 "remote"
}

// TunnelManager 定义隧道管理器接口
type TunnelManager interface {
    Add(forward PortForward) (id string, err error)  // 添加隧道
    Remove(id string) error                           // 移除隧道
    List() []PortForward                              // 列出所有隧道
    StartAll() error                                  // 启动所有隧道
    StopAll() error                                   // 停止所有隧道
}
```

### infra/ 实现层

```
asshc/infra/proxy/
├── socks5.go         # SOCKS5 协议实现（RFC 1928）
│   ├── SOCKS5Proxy struct
│   ├── NewSOCKS5Proxy(sshClient) 
│   ├── Start(addr) / Stop()
│   └── handleConnection(net.Conn)  // SOCKS5 握手 + 转发
├── port_forward.go   # 本地/远程端口转发
│   ├── LocalForward struct
│   ├── RemoteForward struct
│   └── dial/accept + forwarding loop
└── tunnel_manager.go # 隧道管理器
    ├── TunnelManager struct
    ├── Add / Remove / List
    └── StartAll / StopAll
```

### infra/tunnel/ 跳板支持

```
asshc/infra/tunnel/
├── tunnel.go         # SSH 隧道管理（跳板机/堡垒机支持，Phase 10）
```

### service/ 编排层

```go
// asshc/service/proxy.go
type ProxyService struct {
    repo     port.ServerRepository
    ssh      port.SSHConnector
    proxy    port.Proxy
    tunnels  port.TunnelManager
}

func (s *ProxyService) StartSOCKS5(name string, localAddr string) error
func (s *ProxyService) StopSOCKS5() error
func (s *ProxyService) AddLocalForward(name string, local, remote string) error
func (s *ProxyService) AddRemoteForward(name string, remote, local string) error
func (s *ProxyService) RemoveTunnel(id string) error
func (s *ProxyService) ListTunnels() []port.PortForward
```

## SOCKS5 协议实现

### 握手流程

```
Client → Server:  | ver=5 | nmethods | methods... |
Server → Client:  | ver=5 | method=0 (NO AUTH) |

Client → Server:  | ver=5 | cmd=1(CONNECT) | rsv=0 | atyp | dst.addr | dst.port |
Server → Client:  | ver=5 | rep=0(SUCCESS) | rsv=0 | atyp | bnd.addr | bnd.port |
```

- 当前实现仅支持无认证模式（NO AUTH）
- 仅支持 CONNECT 命令（TCP 转发）
- 后续可扩展 UDP ASSOCIATE（Phase 7 迭代）

### 数据转发

SOCKS5 握手完成后，建立从本地连接到远程目标的 TCP 转发通道：

```
[Client App] ←→ [Local Port :1080] ←SSH→ [Remote] ←→ [Target Server:port]
```

数据流通过 SSH 连接双向拷贝（io.Copy 双向 goroutine）。

## 端口转发

### 本地转发 (-L)

```
ssh -L [local_addr:]local_port:remote_host:remote_port user@server
```

实现：
1. 在本地监听 local_port
2. 收到连接后，通过 SSH 连接到 remote_host:remote_port
3. 双向拷贝数据

```go
type LocalForward struct {
    localAddr  string  // 本地监听地址（如 "127.0.0.1:8080"）
    remoteAddr string  // 远程目标地址（如 "localhost:80"）
    listener   net.Listener
    sshClient  *ssh.Client
}
```

### 远程转发 (-R)

```
ssh -R [remote_addr:]remote_port:local_host:local_port user@server
```

实现：
1. 通过 SSH 请求服务器端在 remote_port 上监听
2. 收到连接后，通过 SSH 通道连接回本地 local_host:local_port
3. 双向拷贝数据

```go
type RemoteForward struct {
    remoteAddr string  // 服务器端监听地址
    localAddr  string  // 本地服务地址
    sshClient  *ssh.Client
}
```

## CLI 命令设计

### proxy 子命令

```
assh proxy <server>              # 启动 SOCKS5 代理
assh proxy <server> -p 8080     # 指定端口
```

### tunnel 子命令

```
assh tunnel start <server> -L [local:]remote  # 本地转发
assh tunnel start <server> -R [remote:]local  # 远程转发
assh tunnel stop <id>                          # 停止隧道
assh tunnel list                               # 列出隧道
```

## 数据流图

```
┌─────────────────────────────────────┐
│           CLI Layer (cmd/)          │
│  ┌────────────┐  ┌───────────────┐  │
│  │ proxy.go   │  │ tunnel.go     │  │
│  └─────┬──────┘  └───────┬───────┘  │
└────────┼──────────────────┼──────────┘
         │                  │
┌────────▼──────────────────▼──────────┐
│        Service Layer (asshc)         │
│  ┌───────────────────────────────┐   │
│  │ ProxyService                  │   │
│  │  - StartSOCKS5()              │   │
│  │  - AddLocalForward()          │   │
│  │  - AddRemoteForward()         │   │
│  │  - RemoveTunnel()             │   │
│  └─────────┬─────────────────────┘   │
└────────────┼──────────────────────────┘
             │
┌────────────▼──────────────────────────┐
│         Infra Layer (asshc)           │
│  ┌──────────┐ ┌───────────────┐       │
│  │ proxy/   │ │ tunnel/       │       │
│  │ socks5   │ │ tunnel_mgr    │       │
│  │ port_fwd │ └───────────────┘       │
│  └──────────┘                         │
│            SSH Connector (client.go)  │
└───────────────────────────────────────┘
```

## 依赖关系

- `port.Proxy` / `port.PortForward` / `port.TunnelManager` — 无依赖
- `infra/proxy/` — 依赖 `port.Proxy` + `golang.org/x/crypto/ssh`
- `service/proxy.go` — 依赖 `port.ServerRepository` + `port.SSHConnector` + `port.Proxy` + `port.TunnelManager`
- `cmd/tunnel.go` — 依赖 `service.ProxyService`

## 错误处理策略

- **监听端口占用** → 返回友好错误 "port 1080 already in use"
- **SSH 连接断开** → 自动停止所有活跃隧道，记录日志
- **数据转发中断** → 单个连接断开不影响其他连接
- **SOCKS5 协议错误** → 关闭客户端连接，记录错误日志

## 待处理问题

1. **后台驻留**：当前 CLI 模型为同步执行，隧道需要后台 goroutine 运行
   - 方案：隧道在单独的 goroutine 中运行，CLI 等待信号（Ctrl+C）后清理退出
   - 改进：未来可考虑 daemon 模式（Phase 10+）
2. **重连策略**：SSH 连接断开后是否自动重连？
   - Phase 7 初版：不自动重连，记录日志后停止
   - 后续迭代：可选自动重连（--auto-reconnect）
3. **多隧道并发**：单个 SSH 连接上同时运行多个转发的并发安全
   - 使用 sync.Mutex 保护隧道列表
   - 每个转发独立的 goroutine