# ADR-030: 跳板机多跳架构设计

## Status

proposed

## Date

2026-05-19

## Context

### 问题背景

Phase 10 需要实现 SSH 跳板机（Jump Host / Bastion）支持。核心需求包括：

1. **多跳支持** — 按顺序通过多个跳板机串联到达目标服务器（`assh A -J B,C,D,E`）
2. **文件传输通道** — `assh-fs` 的 push/pull 命令支持通过跳板链传输
3. **端口转发通道** — `proxy/tunnel` 命令支持通过跳板链建立代理
4. **跳板历史记录** — 自动记录跳板路径及认证凭据，支持快速复用（`-J @id`）

### 现有架构约束

- `SSHConnector.Connect(server)` 接口只接收单服务器参数，不支持多跳
- `ConnectService.ConnectByName(name)` / `ConnectDirect(...)` 只支持直连
- 所有命令（login/run/bc/push/pull/proxy/tunnel）的 SSH 连接注入都经由 `ConnectService`

### 技术方案选项

| 选项 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| **A. 修改 Server 结构** | 在 `domain.Server` 增加 JumpHost 字段持久化 | 配置持久化，使用透明 | 改 domain 实体+DB 迁移，影响大 |
| **B. Options 存储** | 用 `Options["jump_host"]` 存跳板名 | 无需改 domain+DB | 隐式，不直观 |
| **C. CLI 层 + ConnectChain** | 仅 CLI 传参 `-J`，Service 层新增 `ConnectChain` 方法 | 不改 domain/port 接口？但需改 port 接口加 ConnectChain | 需要改 port.SSHConnector |

### 认证存储需求

跳板历史需要存储直接指定的认证信息（密码/密钥），以便 `-J @id` 复用时免密码。
已有 AES-GCM 加密基础设施可复用。

## Decision

### 决策 1: SSHConnector 接口新增 ConnectChain

```go
type SSHConnector interface {
    Connect(server *domain.Server) (*ssh.Client, error)
    ConnectChain(target *domain.Server, chain []*domain.Server) (*ssh.Client, error)
    Close(client *ssh.Client) error
}
```

**理由**：
- 多跳连接本质上是不同的连接模式，与直连显然是不同的能力
- 将多跳逻辑放在 `ConnectService` 层会导致 SSH 细节泄露到服务层
- `chainClient` 包装类（全链 cleanup）由 infra 层管理，服务层无需关心

### 决策 2: 不修改 domain.Server 实体

**不添加** `JumpHost` 字段到 `Server`，跳板信息仅由 CLI 的 `-J` 标志传递。

**理由**：
- 跳板路径通常是临时的执行上下文，不是服务器固有属性
- `-J` 多跳语法灵活，支持混合服务器名和直连参数
- 跳板路径由 `jump_history` 表持久化，与服务器配置分离

### 决策 3: 跳板历史独立存储

**新增** `jump_history` 表 + `JumpHistoryRecorder` 接口，独立于 `ServerRepository`。

**理由**：
- 跳板历史是使用记录，与服务器配置生命周期不同
- 历史记录包含认证凭据（AES-GCM 加密），与服务器密码加密统一
- 独立接口便于后续扩展（如按使用频率排序、自动清理）

### 决策 4: 链式连接使用 Go 标准 ssh.Dial 转发

多跳实现通过 `ssh.Dial` → `client.Dial("tcp", addr)` → `ssh.NewClientConn` 链式转发。

**理由**：
- 纯 Go 实现，无需外部依赖
- Go 标准库的 `ssh.Dial` 已涵盖所有认证机制（Agent/Key/Password）
- 与现有代码完全兼容，`authMethods`/`buildConfig` 可复用
- `ssh.NewClientConn` 升级 TCP 连接到 SSH 连接，是 Go SSH 的标准做法

### 决策 5: chainClient 包装类管理连接生命周期

```go
type chainClient struct {
    *ssh.Client            // 目标服务器连接
    parents []*ssh.Client  // 跳板机连接链（从远到近）
}
```

Close() 时按"目标 → 最后一跳 → ... → 第一跳"顺序级联关闭。

**理由**：
- 保证全链路资源释放
- 对调用方透明
- 多次 close 安全（`ssh.Client.Close()` 是幂等的）

### 决策 6: history-ref 语法使用 `@id`

`-J @3` 引用 jump_history 中 ID=3 的记录。

**理由**：
- 显式区分于服务器名和直连参数（`@` 不是合法主机名/服务器名）
- 简洁，符合 CLI 使用习惯
- 易于解析（`strings.HasPrefix(token, "@")`）

### 决策 7: Config 隔离规则 — CLI flags 不传跳板

CLI `-P`/`-i`/`-u` 等认证参数**只应用于目标服务器**，不应用于跳板机。

```bash
assh login target -J bastion -P mypass      # mypass 只用于 target
assh login target -J root@10.0.1.1 -P mypass # mypass 只用于 target，不用于 10.0.1.1
```

**理由**：
- 对齐 SSH 标准行为（"configuration directives supplied on the command-line generally apply to the destination host and not any specified jump hosts"）
- 安全性：防止密码意外泄露到跳板机
- 清晰性：跳板机的认证信息从 DB 或历史记录独立获取

### 决策 8: Options["jump_hosts"] 提供 Config 默认跳板

`ConnectByName` 检查 `server.Options["jump_hosts"]`，自动调用 `ConnectChain`。

```bash
assh set internal-web-01 -o jump_hosts=bastion
assh login internal-web-01  # 自动走跳板
```

**理由**：
- 提供 SSH `~/.ssh/config` 的 `ProxyJump` 等效能力，不改 domain.Server
- 与现有 Options 机制完全兼容（无需 DB 迁移）
- 优先级：CLI `-J` > `-J none` > Options `jump_hosts` > 无跳板

### 决策 9: 每跳独立 Keepalive

`dialThrough` 连接每个跳板后调用 `setupKeepalive`，防止 NAT 超时断开中间连接。

**理由**：
- SSH `ProxyJump` 同样依赖客户端保活机制
- 长连接场景（proxy/tunnel/assh-fs 交互会话）下中间跳可能长时间空闲
- `setupKeepalive` 已是现成方法，调用成本低

### 决策 10: IPv6 方括号语法

`parseJumpChain` 识别 `[...]` 包裹的 IPv6 地址。

```bash
assh login target -J bastion,root@[::1]:2222
```

**理由**：
- SSH `-J` 标准语法
- `net.JoinHostPort` 需要分离 host/port，方括号是唯一可靠的 IPv6 边界标识

## Consequences

### 积极影响

1. **全命令覆盖** — login/run/bc/push/pull/proxy/tunnel 统一使用 `-J` 标志
2. **认证复用** — 跳板历史记录认证凭据，`-J @id` 免密码复用
3. **低侵入** — 不改 domain 实体，不改 DB servers 表
4. **透明** — 拿到 `*ssh.Client` 后向下传递，proxy/tunnel/SFTP 无需额外适配

### 消极影响

1. **接口变更** — `SSHConnector` 接口新增方法，所有实现需更新
2. **测试增加** — 需要 mock chain 连接测试
3. **并发复杂** — `chainClient` 必须保证 Close() 的并发安全性

### 未解决的问题

1. 历史记录自动清理策略（按数量/时间上限）
2. 跳板机本身的健康检测（提前检测跳板可用性）
3. 多跳超时控制（每跳独立超时 vs 整体超时）
4. 每跳 HostKey 验证（当前全局 InsecureIgnoreHostKey，待后续 Phase 统一处理）
