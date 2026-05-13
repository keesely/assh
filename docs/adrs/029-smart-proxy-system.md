# ADR-029: Smart Proxy 智能代理系统

## Status
proposed

## Date
2026-05-13

## Context

### 问题背景

Phase 7 目前实现了独立的 SOCKS5 代理（`infra/proxy/socks5.go`）和端口转发（`infra/proxy/port.go`），但存在以下能力不足：

1. **协议单一**：仅支持 SOCKS5 协议，浏览器和应用常需要 HTTP CONNECT 代理
2. **无路由智能**：所有流量都通过 SSH 转发，无法按域名/规则分流（代理 vs 直连）
3. **无操作审计**：代理过程完全黑盒，无法记录请求日志和流量数据
4. **配置僵化**：端口规则只能单条指定，缺乏批量配置能力

用户需求经过多轮澄清后，归纳为以下四个核心能力：

```
Smart Proxy 智能代理系统
├── 正向代理 ──── SOCKS5 + HTTP 统一入口，通过 SSH 隧道转发
├── AutoProxy 规则 ──── 规则引擎决定路由决策（代理/直连）
├── 透明日志 ──── 代理全链路上下游数据交互记录
└── 多端口规则 ──── 参数配置体系（正向/反向代理端口规则）
```

### 核心术语定义

| 术语 | 定义 |
|------|------|
| **正向代理 (Forward Proxy)** | 客户端明确配置代理地址，将所有请求发送到代理服务，由代理转发到目标 |
| **AutoProxy 规则** | 基于域名/URL 模式的路由规则，匹配结果决定流量走代理还是直连（兼容 GFWList/PAC 格式） |
| **透明日志 (Transparent Logging)** | 代理自动记录每笔请求的上行数据（请求头/体）和下行数据（响应头/体），不作修改仅记录 |
| **多端口规则 (Multi-Port Rules)** | 通过参数批量指定端口映射规则，例如 `80-84:81-85` 表示 5 组端口对映射 |

### 技术约束

- 基于现有 `infra/proxy/` 架构扩展，不破坏已有的 SOCKS5 实现
- 保持 SSH 作为底层传输通道
- 兼容现有 CLI 框架（urfave/cli v1）
- 无新增外部依赖
- 日志系统复用现有 `log/` 模块

## Decision

### 决策 1: 整体架构

采用**分层流水线架构**，所有代理请求通过统一管道处理：

```
[Client App]
     │
     ▼
┌─────────────────────┐
│   Listener Layer    │  ← 协议监听：SOCKS5(:1080) + HTTP(:8080)
│   (协议解析入口)     │
└─────────┬───────────┘
          │ *conn
          ▼
┌─────────────────────┐
│  Interception Layer │  ← 请求拦截：解析目标地址、请求元数据
│  (请求拦截与分析)    │
└─────────┬───────────┘
          │ *parsedReq
          ▼
┌─────────────────────┐
│   Rule Engine       │  ← AutoProxy 规则匹配
│   (路由决策)         │      ├── proxy → SSH 隧道
│                     │      └── direct → 本地直连
└─────────┬───────────┘
          │ *route
          ▼
┌─────────────────────┐
│   Logging Layer     │  ← 透明日志：记录请求/响应上下游数据
│   (数据记录)         │
└─────────┬───────────┘
          │ *data
          ▼
┌─────────────────────┐
│   Transport Layer   │  ← SSH Tunnel / Direct Dial
│   (数据传输)         │
└─────────────────────┘
```

### 决策 2: 正向代理 — 统一 SOCKS5 + HTTP 入口

**现状**：`SOCKS5Proxy` 仅处理 SOCKS5 协议。

**决策**：保持 SOCKS5Proxy 独立，新增 HTTP CONNECT 处理器，通过统一的 `Service` 层组装。

```
proxy/                   # 代理基础设施
├── socks5.go           # SOCKS5 协议（已有，增强集成）
├── http_connect.go     # HTTP CONNECT 处理器（新增）
├── listener.go         # 统一监听器管理（新增）
└── ...
```

```go
// 统一代理配置
type ProxyConfig struct {
    SOCKS5Addr string   // SOCKS5 监听地址（如 ":1080"），空=不启用
    HTTPAddr   string   // HTTP 监听地址（如 ":8080"），空=不启用
    RuleFile   string   // AutoProxy 规则文件路径
    LogDir     string   // 代理日志目录
    PortRules  []PortRule // 端口映射规则
}
```

**设计说明**：
- 不改变 `port.Proxy` 接口签名，`SOCKS5Proxy` 维持原样
- HTTP CONNECT 处理器遵循 RFC 7231，监听 TCP 连接、解析 `CONNECT host:port HTTP/1.1` 请求
- 两种协议使用相同的下层路由/日志/传输组件

### 决策 3: AutoProxy 规则引擎

**规则格式**：兼容 AutoProxy / GFWList 规则语法：

```
# 代理规则
*.google.com
*.youtube.com
*.twitter.com
! 注释行

# 直连规则（白名单）
localhost
127.0.0.1
*.local
*.cn
```

支持规则类型：

| 类型 | 语法 | 示例 | 匹配逻辑 |
|------|------|------|----------|
| 域名后缀 | `*.domain` | `*.google.com` | 匹配 `www.google.com`, `mail.google.com` |
| 精确域名 | `domain` | `google.com` | 仅匹配 `google.com` |
| IP 段 | `ip/mask` | `10.0.0.0/8` | CIDR 匹配 |
| 通配符 | `*.domain.*` | `*.google.*` | 多级通配 |
| 注释 | `! text` | `! 规则说明` | 跳过 |
| 白名单 | `@@pattern` | `@@*.cn` | 优先匹配，直连 |
| 正则 | `/regex/` | `/\.gov\.cn$/` | 正则匹配（可选） |

```go
// port/rules.go — 规则引擎接口
type RuleEngine interface {
    // Match 判断目标是否匹配代理规则
    // 返回 true=走代理, false=直连
    Match(target string) (proxy bool, rule *MatchedRule, err error)

    // Load 加载规则文件
    Load(path string) error

    // Reload 热重载规则
    Reload() error
}

type MatchedRule struct {
    Pattern string // 匹配的模式
    Action  string // "proxy" 或 "direct"
    Line    int    // 规则文件行号
}
```

**匹配优先级**：
1. 白名单规则 (`@@pattern`) — 最高优先级
2. 域名模式匹配
3. IP 段匹配
4. 正则匹配
5. 未匹配 → 默认走代理 或 默认直连（可配置）

### 决策 4: 透明日志系统

代理请求的全链路日志记录，作为透明代理的核心体现：

```go
// port/logger.go — 代理日志接口
type ProxyLogger interface {
    // LogRequest 记录一次代理请求
    LogRequest(req *RequestLog) error

    // LogData 记录上下游数据
    LogData(sessionID string, direction string, data []byte) error

    // Close 关闭日志
    Close() error
}

type RequestLog struct {
    SessionID   string    // 会话 ID
    Timestamp   time.Time // 时间戳
    Protocol    string    // "socks5" / "http" / "direct"
    ClientAddr  string    // 客户端地址
    TargetAddr  string    // 目标地址
    Action      string    // "proxy" / "direct"
    RuleMatched string    // 匹配的规则
    BytesSent   int64     // 发送字节数
    BytesRecv   int64     // 接收字节数
    Duration    time.Duration // 耗时
    Error       string    // 错误信息（如有）
}
```

**日志格式**：
- 实时日志：结构化 JSON，每请求一行，追加写入
- 会话日志：按 session ID 组织的完整上下游数据包
- 存储路径：`~/.assh/v2/proxy-logs/{date}/`

**透明代理的"透明"含义**：
- 用户无感知——代理自动决定路由，无需手动切换
- 数据透明——所有通过代理的流量均有日志可查
- 规则透明——用户可查看哪些规则匹配了哪些请求

### 决策 5: 多端口规则配置

端口规则作为正向代理和反向代理的配置参数：

```
# 正向代理端口映射
-P/--proxy-port    80-84:81-85    # A:80→B:81, A:81→B:82, ..., A:84→B:85
-P/--proxy-port    443:443        # A:443→B:443（单端口）

# 反向代理端口映射
-R/--remote-port   8080:localhost:80   # 远程:8080→本地:80（已有 -R）
```

规则解析逻辑：

```go
type PortRule struct {
    Direction string // "forward" 或 "reverse"
    FromPorts []int  // 源端口列表
    ToPorts   []int  // 目标端口列表
    ToHost    string // 目标主机（reverse 适用）
}

// ParsePortRule 解析端口规则字符串
// "80-84:81-85" → {FromPorts:[80,81,82,83,84], ToPorts:[81,82,83,84,85]}
// "443:443"     → {FromPorts:[443], ToPorts:[443]}
// "8080:localhost:80" → {Direction:"reverse", FromPorts:[8080], ToHost:"localhost", ToPorts:[80]}
func ParsePortRule(s string) (*PortRule, error)
```

### 决策 6: 服务编排层

```go
// service/proxy.go (扩展)
type ProxyService struct {
    repo        port.ServerRepository
    connector   port.SSHConnector
    socks5Proxy port.Proxy
    httpProxy   *proxy.HTTPConnectProxy  // 新增
    ruleEngine  port.RuleEngine          // 新增
    proxyLogger port.ProxyLogger         // 新增
    tunnelMgr   port.TunnelManager
}

// StartSmartProxy 启动智能代理
// 根据 ProxyConfig 同时启动 SOCKS5 + HTTP 监听
// 集成 RuleEngine 和 ProxyLogger
func (s *ProxyService) StartSmartProxy(name string, cfg ProxyConfig) error

// StopProxy 停止所有代理
func (s *ProxyService) StopProxy() error

// ReloadRules 热重载规则
func (s *ProxyService) ReloadRules() error
```

### 决策 7: CLI 命令

```bash
assh proxy <server>                       # 默认智能代理模式
assh proxy <server> --socks5 :1080        # 仅 SOCKS5
assh proxy <server> --http :8080          # 仅 HTTP
assh proxy <server> --rule gfwlist.txt    # 加载 AutoProxy 规则
assh proxy <server> --log-dir ./logs      # 启用记录
assh proxy <server> -P 80-84:81-85        # 多端口规则
assh proxy <server> -P 443:443 -P 80-84:81-85  # 多组规则

# 规则管理
assh proxy rule reload                    # 热重载规则
assh proxy rule status                    # 查看规则状态
assh proxy log <session-id>               # 查看会话日志
```

## Consequences

### 正面影响

1. **统一代理入口** — 一个命令同时提供 SOCKS5 + HTTP，用户无需切换
2. **智能路由** — AutoProxy 规则自动决定代理/直连，网络体验流畅
3. **可审计** — 全链路日志为运维和安全审计提供数据基础
4. **批量配置** — 多端口规则减少重复配置
5. **渐进兼容** — 不破坏现有 SOCKS5 接口，旧功能保持不变
6. **透明显式化** — "透明"体现在数据可观测，而非用户无感知（区别于 OS 级别透明代理）

### 负面影响

1. **架构复杂度增加** — 从单 SOCKS5 变为多层流水线架构
2. **AutoProxy 规则解析** — 需要实现完整的规则解析器（无外部依赖）
3. **日志 IO 开销** — 高频请求场景下日志写入可能成为瓶颈（需异步写入 + 缓冲）
4. **HTTP CONNECT 新增协议维护** — 需要处理 HTTP 协议的边界情况
5. **CLI 参数复杂度增加** — `proxy` 命令参数增多，需注意 UX 设计

### 与现有 Phase 7 的兼容关系

| 现有组件 | 变化 |
|----------|------|
| `SOCKS5Proxy` | 保持不变，通过 `proxyListener` 集成到流水线 |
| `LocalForward / RemoteForward` | 保持不变 |
| `PortForward` 接口 | 不变 |
| `TunnelManager` | 扩展以支持规则感知 |
| `port.Proxy` 接口 | 不变 |

### 后续待定事项

- HTTP CONNECT 的完整实现细节（需考虑 HTTPS 嗅探、WebSocket 等）
- AutoProxy 规则文件缓存和增量更新
- 日志轮转和清理策略
- Web 管理界面（查看日志/规则状态）
- 规则远程下发（结合云同步 Phase 8）

## Related

- ADR-028: 代理 & 隧道架构设计（Phase 7 基础架构）
- Phase 7 设计文档: `docs/modules/proxy-tunnel.md`
- [AutoProxy / GFWList 规则格式](https://github.com/gfwlist/gfwlist)
- [RFC 1928 — SOCKS Protocol Version 5](https://datatracker.ietf.org/doc/html/rfc1928)
- [RFC 7231 — HTTP/1.1 CONNECT](https://datatracker.ietf.org/doc/html/rfc7231#section-4.3.6)
