# Phase 10: 跳板机支持 设计方案

> 日期: 2026-05-19
> 状态: 设计阶段
> ADR: ADR-030 (跳板机多跳架构设计)

---

## 1. 术语表

| 术语 | 英文 | 定义 | 示例 |
|------|------|------|------|
| **跳板机** | Jump Host / Bastion | 作为中间节点转发 SSH 连接的服务器 | `bastion.example.com` |
| **多跳** | Multi-hop | 多个跳板机串联连接 | `A → B → C → D → Target` |
| **跳板链** | Jump Chain | 有序的跳板机列表，从第一个开始依次连接 | `[B, C, D, E]` |
| **ProxyJump** | ProxyJump | SSH 原生跳板机制 (ssh -J) | `ssh -J user@bastion:22 target` |
| **链式客户端** | Chain Client | 通过多跳链建立的 SSH 连接（含全链路 cleanup） | `ssh.Client` wrapper |
| **跳板历史** | Jump History | 存储历史跳板路径及认证信息的数据库记录 | `jump_history` 表 |

---

## 2. 设计目标

实现 SSH 多跳跳板机支持，满足以下场景：

1. **多跳登录** — 通过多个跳板机串联到达目标服务器
2. **文件传输** — assh-fs 的 push/pull 命令支持通过跳板链传输
3. **端口转发** — proxy/tunnel 命令支持通过跳板链建立通道
4. **跳板记录** — 自动记录跳板路径及认证凭据，支持快速复用

---

### 2.1 SSH 标准行为补齐

Phase 10 需要实现以下 SSH `-J` 标准行为，确保兼容性：

| # | 行为 | SSH 标准 | ASSH 实现方式 |
|---|------|----------|---------------|
| 1 | **Config 默认跳板** | `~/.ssh/config` → `Host *.internal → ProxyJump bastion` | `ConnectByName` 检查 `Options["jump_hosts"]`，自动调用 `ConnectChain` |
| 2 | **Config 隔离规则** | CLI flags 只作用于目标，不作用于跳板机 | `parseJumpChain` 不为跳板机应用 CLI `-P`/`-i` 等参数 |
| 3 | **-J none 禁用** | `-J none` 禁用继承的跳板配置 | 显式检测 `none`，跳过多跳逻辑走直连 |
| 4 | **IPv6 地址** | `[::1]:22` 方括号语法 | `parseJumpChain` 识别 `[...]` 包裹的地址 |
| 5 | **每跳 Keepalive** | 跳板机也需要保活 | `dialThrough` 中对每个跳板应用 `setupKeepalive` |

---

## 3. 命令行设计

### 3.1 语法

统一使用 `-J`/`--jump` 标志，在各命令间保持一致：

```bash
# -J 后跟逗号分隔的跳板列表
assh login <target> -J <jump1,jump2,...>

# 每条记录可以是：服务器名、user@host[:port]、@history_id、IPv6 或 none
assh login <target> -J bastion,gateway,root@10.0.1.1:2222

# IPv6 地址使用方括号包裹（与 SSH 标准一致）
assh login <target> -J bastion,root@[::1]:2222

# 复用历史跳板记录
assh login <target> -J @<history_id>

# -J none 禁用跳板（覆盖继承的配置或 Options["jump_hosts"]）
assh login <target> -J none
```

**CLI flags 隔离规则**：`-P`、`-i`、`-u` 等 CLI 认证参数**只应用于目标服务器**，不应用于跳板机。跳板机的认证来自：
- 服务器名引用 → 从 DB 读取服务器配置中的认证信息
- 历史引用 → 从 `jump_history` 读取加密存储的认证信息

### 3.2 登录

```bash
# 通过配置好的跳板机连接
assh login internal-web-01 -J bastion,gateway

# 直连模式 + 跳板
assh login root@10.0.1.5 -J root@bastion:22

# 分离参数模式 + 跳板
assh login -H 10.0.1.5 -u root -J bastion,gateway

# 复用历史跳板
assh login internal-web-01 -J @3
```

### 3.3 命令执行

```bash
# 通过跳板执行命令
assh run internal-web-01 "uptime" -J bastion,gateway

# 批量执行 + 跳板
assh bc "uptime" --servers s1,s2 -J bastion
assh bc "uptime" --group internal -J gateway
```

### 3.4 文件传输

```bash
# 推送文件走跳板
assh-fs push internal-web-01 ./file.txt /tmp/ -J bastion,gateway

# 拉取文件走跳板
assh-fs pull internal-web-01 /remote/file.txt ./ -J bastion,gateway

# 递归 + 跳板
assh-fs push -r internal-web-01 ./dir/ /tmp/ -J bastion
```

### 3.5 代理/隧道

```bash
# SOCKS5 代理通过跳板
assh proxy internal-web-01 --socks5 :1080 -J bastion

# HTTP CONNECT 通过跳板
assh proxy internal-web-01 --http :8080 -J bastion

# 本地端口转发通过跳板
assh tunnel start internal-web-01 -L 8080:localhost:80 -J bastion

# 远程端口转发通过跳板
assh tunnel start internal-web-01 -R 8080:localhost:3000 -J bastion
```

### 3.6 跳板历史管理

```bash
# 查看历史跳板记录
assh jump history
# 输出:
# ID  PATH                         CHAIN  USE  LAST_USED
# 3   bastion,gateway              2      5    2026-05-19 15:04
# 4   root@10.0.1.1:2222,gateway  2      3    2026-05-19 14:30

# 复用历史记录（自动使用存储的认证信息）
assh login internal-web-01 -J @3

# 通过历史 + 覆盖认证
assh login internal-web-01 -J @3 -P newpass

# 清理历史
assh jump history --clear
assh jump history --delete <id>
```

---

## 4. 核心架构

### 4.1 多跳连接流程

```
                  ┌──────────────────────┐
                  │  目标: internal-web-01 │
                  │  -J bastion,gateway   │
                  └───────┬──────────────┘
                          │
                          ▼
            ┌─────────────────────────┐
            │   解析跳板链             │
            │   [bastion, gateway]     │
            │   (从 DB/直连参数解析)    │
            └──────────┬──────────────┘
                       │
                       ▼
            ┌─────────────────────────┐
            │   Dial bastion           │
            │   ssh.Dial("tcp", addr)  │
            │   └── 得到 client1       │
            └──────────┬──────────────┘
                       │
                       ▼
            ┌─────────────────────────┐
            │   Dial through client1   │
            │   client1.Dial("tcp",    │
            │     gateway_addr)        │
            │   → ssh.NewClientConn    │
            │   └── 得到 client2       │
            └──────────┬──────────────┘
                       │
                       ▼
            ┌─────────────────────────┐
            │   Dial through client2   │
            │   → target               │
            │   └── 得到 targetClient  │
            └──────────┬──────────────┘
                       │
                       ▼
            ┌─────────────────────────┐
            │   返回 chainClient       │
            │   (包装 targetClient,    │
            │    Close() 时级联      │
            │    关闭全链 client)      │
            └─────────────────────────┘
```

### 4.2 链式客户端 (Chain Client)

```go
// chainClient 包装最终的 SSH 连接，确保 Close() 时级联关闭所有跳板连接
type chainClient struct {
    *ssh.Client                 // 目标服务器连接
    parents []*ssh.Client       // 跳板机连接链（从远到近）
}

func (c *chainClient) Close() error {
    // 1. 关闭目标连接
    err := c.Client.Close()
    // 2. 从近到远关闭跳板连接
    for i := len(c.parents) - 1; i >= 0; i-- {
        c.parents[i].Close()
    }
    return err
}
```

### 4.3 与既有组件的交互

```
             ┌──────────────┐
             │  SSHConnector │  ← 接口层 (port/connector.go)
             │              │     Connect(server) → *ssh.Client
             │              │     +ConnectChain(target, chain) → *ssh.Client  
             └──────┬───────┘
                    │
          ┌─────────┴──────────┐
          ▼                    ▼
┌──────────────────┐  ┌──────────────────┐
│  Connector       │  │  Connector       │
│  .Connect()      │  │  .ConnectChain() │
│  直连实现         │  │  多跳实现         │
└──────────────────┘  └──────────────────┘
          │                    │
          ▼                    ▼
┌─────────────────────────────────────────┐
│            ConnectService               │
│  (service/connect.go)                   │
│  - ConnectByName(name)                  │
│  - ConnectDirect(...)                   │
│  + ConnectChain(name, chainExpr)        │
│  + parseJumpChain(expr) → []*Server     │
└────────────────┬────────────────────────┘
                 │
        ┌────────┴────────┐
        ▼                 ▼
┌──────────────┐  ┌──────────────┐
│  CLI 命令     │  │  assh-fs     │
│  login/run/  │  │  push/pull   │
│  bc/proxy/   │  │              │
│  tunnel      │  │              │
└──────────────┘  └──────────────┘
```

---

## 5. 接口变更

### 5.1 port/connector.go — SSHConnector

```go
type SSHConnector interface {
    // Connect 直连（现有）
    Connect(server *domain.Server) (*ssh.Client, error)
    
    // ConnectChain 通过跳板链连接
    // chain 参数为有序的跳板服务器列表，从第一个依次连接
    ConnectChain(target *domain.Server, chain []*domain.Server) (*ssh.Client, error)
    
    // Close 关闭连接（现有）
    Close(client *ssh.Client) error
}
```

### 5.2 port/jump.go — JumpHistoryRecorder（新增）

```go
package port

import "assh/asshc/domain"

// JumpHopAuth 记录一跳的认证信息
type JumpHopAuth struct {
    Type       string `json:"type"`                  // "server" 或 "direct"
    ServerRef  string `json:"server_ref,omitempty"`  // 服务器名引用（type=server）
    Host       string `json:"host,omitempty"`         // 直接主机地址（type=direct）
    Port       int    `json:"port,omitempty"`
    User       string `json:"user,omitempty"`
    Password   string `json:"password,omitempty"`     // 明文（持久化时加密）
    KeyFile    string `json:"keyfile,omitempty"`
}

// JumpHistoryRecorder 定义跳板历史记录存储接口
type JumpHistoryRecorder interface {
    // Record 记录一次跳板跳使用
    Record(jh *domain.JumpHistory) error
    
    // List 返回历史记录，按 last_used 降序
    List(limit int) ([]*domain.JumpHistory, error)
    
    // Get 根据 ID 获取历史记录
    Get(id int64) (*domain.JumpHistory, error)
    
    // Delete 删除指定历史记录
    Delete(id int64) error
    
    // Clear 清空所有历史记录
    Clear() error
    
    // IncrementUse 递增使用次数并更新 last_used
    IncrementUse(id int64) error
}
```

---

## 6. 领域层变更

### 6.1 asshc/domain/jump.go（新增）

```go
package domain

// JumpHistory 表示一次跳板连接的历史记录
type JumpHistory struct {
    ID           int64           `json:"id"`
    TargetExpr   string          `json:"target_expr"`   // 目标表达式（服务器名或 "user@host:port"）
    PathText     string          `json:"path_text"`     // 人类可读："bastion,gateway,root@10.0.1.1"
    PathData     []JumpHopAuth   `json:"-"`             // 加密持久化的每跳认证信息
    ChainCount   int             `json:"chain_count"`
    LastUsed     string          `json:"last_used"`
    UseCount     int             `json:"use_count"`
    CreatedAt    string          `json:"created_at"`
    UpdatedAt    string          `json:"updated_at"`
}
```

---

## 7. 基础设施层变更

### 7.1 asshc/infra/ssh/client.go — 多跳实现

核心逻辑 `ConnectChain`：

```go
func (c *Connector) ConnectChain(target *domain.Server, chain []*domain.Server) (*ssh.Client, error) {
    if len(chain) == 0 {
        return c.Connect(target)
    }

    var parents []*ssh.Client
    
    // Step 1: 连接第一个跳板机
    first := chain[0]
    client, err := c.dialServer(first)
    if err != nil {
        return nil, fmt.Errorf("jump[0] %s: %w", first.Host, err)
    }
    parents = append(parents, client)

    // Step 2: 依次串联后续跳板机
    for i := 1; i < len(chain); i++ {
        hop := chain[i]
        client, err = c.dialThrough(client, hop)
        if err != nil {
            c.closeAll(parents)
            return nil, fmt.Errorf("jump[%d] %s: %w", i, hop.Host, err)
        }
        parents = append(parents, client)
    }

    // Step 3: 最后连接目标
    targetClient, err := c.dialThrough(client, target)
    if err != nil {
        c.closeAll(parents)
        return nil, fmt.Errorf("target %s: %w", target.Host, err)
    }

    return &chainClient{Client: targetClient, parents: parents}, nil
}

// dialThrough 通过已有客户端拨号到目标服务器
// 连接建立后自动配置 Keepalive（与直连行为一致）
func (c *Connector) dialThrough(parent *ssh.Client, server *domain.Server) (*ssh.Client, error) {
    addr := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))
    conn, err := parent.Dial("tcp", addr)
    if err != nil {
        return nil, fmt.Errorf("dial %s: %w", addr, err)
    }
    sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, c.buildConfig(server))
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("handshake %s: %w", addr, err)
    }
    client := ssh.NewClient(sshConn, chans, reqs)
    c.setupKeepalive(client, server) // 每跳独立保活
    return client, nil
}
```

### 7.2 辅助方法提取

将 `Connect` 中的 SSH 配置构建逻辑提取为 `buildConfig`，供 `Connect` 和 `ConnectChain` 共用：

```go
func (c *Connector) buildConfig(server *domain.Server) *ssh.ClientConfig {
    return &ssh.ClientConfig{
        User:            server.User,
        Auth:            c.authMethods(server),
        HostKeyCallback: c.getHostKeyCallback(server),
        Timeout:         DefaultTimeout,
    }
}
```

### 7.3 asshc/infra/store/db.go — 数据库迁移

```sql
CREATE TABLE IF NOT EXISTS jump_history (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    target_expr      TEXT NOT NULL,
    path_text        TEXT NOT NULL,
    path_data        BLOB,                    -- AES-GCM 加密的每跳认证 JSON
    chain_count      INTEGER NOT NULL DEFAULT 0,
    last_used        TEXT,
    use_count        INTEGER DEFAULT 1,
    created_at       TEXT DEFAULT (datetime('now')),
    updated_at       TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_jump_history_last_used ON jump_history(last_used DESC);
```

### 7.4 asshc/infra/store/jump.go（新增）

```go
// Record 写入跳板历史（path_data 使用 AES-GCM 加密后存储）
func (s *Store) Record(jh *domain.JumpHistory) error
// List 返回最近使用的 N 条记录
func (s *Store) List(limit int) ([]*domain.JumpHistory, error)
// Get 按 ID 获取（自动解密 path_data）
func (s *Store) Get(id int64) (*domain.JumpHistory, error)
// Delete 删除
func (s *Store) Delete(id int64) error
// Clear 清空
func (s *Store) Clear() error
// IncrementUse 递增使用次数
func (s *Store) IncrementUse(id int64) error
```

path_data 加密格式：
```
AES-256-GCM 加密
  plaintext: JSON([{type, host, port, user, password, key_file, server_ref}, ...])
```

---

## 8. 服务层变更

### 8.1 asshc/service/connect.go

```go
type ConnectService struct {
    connector     port.SSHConnector
    session       port.SSHSession
    repo          port.ServerRepository
}

// ConnectChain 通过指定跳板链连接到目标
// chainExpr 的格式：逗号分隔的跳板列表
//   每条记录可以是：服务器名 或 @history_id
//      服务器名 → 从 DB 读取配置
//      @id      → 从 jump_history 读取并恢复跳板链
//      user@host[:port] → 直连参数构建服务器对象
func (s *ConnectService) ConnectChain(target string, chainExpr string) (*ssh.Client, []*domain.Server, error)

// parseJumpChain 解析跳板链表达式为 *domain.Server 列表
// 返回值还包括是否包含 direct 类型（需要记录历史）
func (s *ConnectService) parseJumpChain(chainExpr string) ([]*domain.Server, []JumpHopMeta, error)
```

### 8.2 Config 默认跳板 (Options["jump_hosts"])

`ConnectByName` 方法扩展为自动检测 `server.Options["jump_hosts"]`：

```go
func (s *ConnectService) ConnectByName(name string) (*ssh.Client, error) {
    server, err := s.repo.Get(name)
    if err != nil {
        return nil, err
    }

    // 检查 Options 中是否配置了默认跳板
    if jumpExpr, ok := getJumpHostsFromOptions(server); ok && jumpExpr != "" {
        // 通过跳板链连接（自动解析 Options 中的跳板列表）
        client, _, err := s.ConnectChain(name, jumpExpr)
        return client, err
    }

    return s.connector.Connect(server)
}

// getJumpHostsFromOptions 从 Options 中读取跳板链配置
// 支持单跳板和多跳板，逗号分隔
// 示例: -o jump_hosts=bastion 或 -o jump_hosts=bastion,gateway
func getJumpHostsFromOptions(server *domain.Server) (string, bool) {
    if server.Options == nil {
        return "", false
    }
    v, ok := server.Options["jump_hosts"]
    if !ok {
        return "", false
    }
    if s, ok := v.(string); ok && s != "" {
        return s, true
    }
    return "", false
}
```

**配置方式**：
```bash
assh set internal-web-01 -o jump_hosts=bastion
```

此后连接自动走跳板：
```bash
assh login internal-web-01  # → bastion → internal-web-01
```

跳板配置可通过 `-J none` 临时覆盖：
```bash
assh login internal-web-01 -J none  # 直连，不走跳板
```

**优先级**（高→低）：
1. CLI `-J` 参数（显式指定）
2. `-J none`（禁用继承）
3. Options 中的 `jump_hosts`
4. 无跳板（直连）

### 8.3 asshc/service/jump.go（新增）

```go
// JumpService 跳板历史服务
type JumpService struct {
    recorder port.JumpHistoryRecorder
    crypto   port.Cryptor
}

// RecordJumpChain 记录一次跳板链使用
func (s *JumpService) RecordJumpChain(targetExpr, pathText string, hops []JumpHopMeta) error

// List 列出历史
func (s *JumpService) List(limit int) ([]*domain.JumpHistory, error)

// Delete 删除
func (s *JumpService) Delete(id int64) error

// Clear 清空
func (s *JumpService) Clear() error
```

---

## 9. CLI 命令变更

### 9.1 各命令的 -J 标志

在 `login`、`run`、`bc`、`push`、`pull`、`proxy`、`tunnel` 命令中统一添加：

```go
cli.StringFlag{Name: "J, jump", Usage: "jump host chain (comma-separated): name,@id,user@host[:port]"}
```

### 9.2 新增子命令 — cmd/jump.go

```go
// 注册 jump 命令组
func (a *App) registerJumpCommands()

// 命令树
assh jump history [--limit N]    // 查看跳板历史
assh jump history --clear        // 清空
assh jump history --delete <id>  // 删除指定
```

### 9.3 处理流程

```go
// loginAction 中（其他命令类似）
func (a *App) loginAction(c *cli.Context) error {
    jumpExpr := c.String("jump")
    
    if jumpExpr != "" {
        // 通过跳板连接
        client, hops, err := a.connectSvc.ConnectChain(target, jumpExpr)
        if err != nil { return err }
        defer a.connectSvc.Close(client)
        
        // 异步记录跳板历史（如果有 direct 类型的跳板）
        if len(hops) > 0 {
            go a.jumpSvc.RecordJumpChain(target, jumpExpr, hops)
        }
        
        return a.connectSvc.Shell(client)
    }
    
    // 无跳板，走原有逻辑
    // ...
}
```

---

## 10. 跳板解析流程

### 10.1 chainExpr 格式

```
"bastion,gateway,root@10.0.1.1:2222,@3"
  ├── "bastion"                      → serverRef, 从 DB 读取
  ├── "gateway"                      → serverRef, 从 DB 读取
  ├── "root@10.0.1.1:2222"          → direct, 直连参数
  └── "@3"                           → historyRef, 从 jump_history 表 ID=3 读取

"[::1]:22"                           → IPv6 direct, 方括号识别
"none"                               → 禁用跳板, 跳过后续解析
```

### 10.2 解析状态图

```
parseJumpChain(expr)
  │
  ├── if expr == "none" → return empty (跳过跳板)
  │
  ├── splitByComma(expr, keepBrackets)
  │    注: 逗号分隔时跳过方括号内的逗号 "prefix,[::1]:22,suffix"
  │        → ["prefix", "[::1]:22", "suffix"]
  │
  ├── for each token:
  │     │
  │     ├── startsWith "@" → historyRef
  │     │     └── jumpStore.Get(id) → 解密 path_data → []JumpHop
  │     │
  │     ├── startsWith "[" → IPv6 direct
  │     │     └── parseIPv6Host(token) → Server{Host, Port}
  │     │         Auth 不应用 CLI flags（Config 隔离规则）
  │     │
  │     ├── contains "@" → direct (user@host)
  │     │     └── parseUserHost(token) → Server{User, Host, Port}
  │     │         Auth 不应用 CLI flags（Config 隔离规则）
  │     │
  │     └── else → serverRef
  │           └── repo.Get(token) → Server
  │               （认证信息从 DB 读取）
  │
  └── return []*domain.Server
```

### 10.3 IPv6 解析

```
parseIPv6Host(token)
  │ 输入: "[::1]:22" 或 "[::1]" 或 "[2001:db8::1]:2222"
  │
  ├── 提取方括号内容 → host = "::1"
  ├── 方括号后跟 ":" → port = "22"（默认 22）
  ├── 验证 net.ParseIP(host) 为 IPv6
  └── 返回 Server{Host: host, Port: port}
```

---

## 11. 历史记录认证存储

### 11.1 记录时机

当 `-J` 参数包含 **direct 类型** 跳板（非纯服务器名引用）时，记录到 jump_history。

纯服务器名引用的跳板不重复存储认证信息（从 servers 表读取）。

### 11.2 存储内容

```json
[
  {"type":"server",  "server_ref":"bastion"},
  {"type":"direct",  "host":"10.0.1.1", "port":2222, "user":"root",
   "password":"<AES-GCM encrypted>", "key_file":""},
  {"type":"server",  "server_ref":"gateway"}
]
```

### 11.3 复用流程

```
-J @3
  │
  ├── jumpStore.Get(3)
  │     └── 解密 path_data → []JumpHop
  │
  ├── for each hop:
  │     ├── type=server → repo.Get(server_ref) → full Server
  │     └── type=direct → 从 path_data 恢复 host/port/user/password/keyFile
  │
  └── Server 列表 → ConnectChain(target, chain)
```

---

## 12. 文件变更清单

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `asshc/domain/jump.go` | 新增 | JumpHistory 领域实体 |
| `asshc/port/connector.go` | 修改 | SSHConnector 新增 ConnectChain |
| `asshc/port/jump.go` | 新增 | JumpHistoryRecorder 接口 |
| `asshc/infra/ssh/client.go` | 修改 | 实现 ConnectChain + chainClient + buildConfig |
| `asshc/infra/store/db.go` | 修改 | jump_history 表迁移 |
| `asshc/infra/store/jump.go` | 新增 | JumpHistory CRUD 实现 |
| `asshc/infra/store/key.go` | 复用 | path_data 加密复用现有 AES-GCM |
| `asshc/service/connect.go` | 修改 | 新增 ConnectChain + parseJumpChain |
| `asshc/service/jump.go` | 新增 | JumpService 跳板历史服务 |
| `cmd/connect.go` | 修改 | login/run/bc 添加 -J 标志 |
| `cmd/sftp_cmd.go` | 修改 | push/pull 添加 -J 标志 |
| `cmd/proxy.go` | 修改 | proxy 添加 -J 标志 |
| `cmd/tunnel.go` | 修改 | tunnel 添加 -J 标志 |
| `cmd/jump.go` | 新增 | jump history 命令组 |
| `cmd/app.go` | 修改 | 注册 jump 命令 |
| `cmd/bootstrap.go` | 修改 | 注入 JumpService |
| `cmd/fs/main.go` | 修改 | assh-fs 注入 jump 支持 |
| `main.go` | 修改 | DI 注入 JumpService + JumpHistoryRecorder |
| `docs/TODO.md` | 修改 | 更新 Phase 10 任务清单 |

---

## 13. 验证计划

```bash
# 编译验证
cd asshv2 && go build ./...

# 静态检查
cd asshv2 && go vet ./...

# 单元测试
cd asshv2 && go test ./asshc/... -v -count=1

# 功能验收
# 1. 多跳登录
assh login internal-web-01 -J bastion,gateway

# 2. 直连+跳板
assh login root@10.0.1.5 -J root@bastion:22

# 3. 命令执行+跳板
assh run internal-web-01 "uptime" -J bastion

# 4. 跳板历史记录
assh jump history

# 5. 复用历史
assh login internal-web-01 -J @3

# 6. 文件传输+跳板
assh-fs push internal-web-01 ./file /tmp/ -J bastion

# 7. 代理/隧道+跳板
assh tunnel start internal-web-01 -L 8080:localhost:80 -J bastion

# 8. Config 默认跳板（Options）
assh set internal-web-01 -o jump_hosts=bastion
assh login internal-web-01  # 自动走跳板

# 9. -J none 禁用跳板
assh login internal-web-01 -J none  # 直连

# 10. IPv6 跳板
assh login target -J bastion,root@[::1]:2222

# 11. 每跳错误报告
# 输出: jump[1] gateway (10.0.1.5:22): dial tcp ...
```

---

## 14. 依赖关系

```
Phase 10 (跳板机)
  └── Phase 3 (SSH 连接层) — 依赖 SSH 连接能力
        └── Phase 2 (数据持久化) — 依赖 store 接口
              └── Phase 1 (基础设施) — 依赖 crypto/AES-GCM
```

跳板机功能不依赖 Phase 5 (SFTP) / Phase 7 (代理)，但 assh-fs 和 proxy/tunnel 的 `-J` 支持
需要调用 `ConnectChain`，因此这些命令的 `-J` 标志将在 Phase 10 中一并添加。
