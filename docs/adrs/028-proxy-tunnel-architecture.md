# ADR-028: 代理 & 隧道架构设计

## Status
accepted

## Date
2026-05-13

## Context

### 问题背景

Phase 7 需要实现 SSH 代理和隧道功能，包括 SOCKS5 代理、本地/远程端口转发和 SSH 隧道管理。该项目涉及多个层级的设计决策：

1. **接口签名**：Proxy/PortForward 的 `Start` 方法是否应该在构造时绑定 SSH 客户端，还是由调用方传入
2. **SOCKS5 认证**：是否仅支持 NO AUTH，还是同时支持 USERNAME/PASSWORD 认证
3. **CLI 组织**：SOCKS5 代理和端口转发隧道应该共享同一命令组还是分开
4. **后台驻留**：隧道/代理需要持续运行，当前 CLI 模型（同步执行）如何适配
5. **重连策略**：SSH 断开时如何处理 —— 直接退出还是自动重连
6. **直连支持**：代理命令是否需要支持直连参数（`-H/-u/-P` 等）

### 技术约束

- SSH 连接来自 `port.SSHConnector` 接口
- 代理和隧道需要在 goroutine 中持续运行
- 需兼容现有 CLI 框架（urfave/cli v1）
- 多隧道场景的并发安全

## Decision

### 决策 1: SSH 客户端生命周期 — Start 时传入

**Proxy.Start 和 PortForward.Start 方法接收 `*ssh.Client` 参数**，而非构造时注入。

```go
// Proxy
Start(client *ssh.Client, localAddr string) error

// PortForward
Start(client *ssh.Client) error
```

**理由**：
- SSH 客户端由 Service 层统一创建和管理，便于重连时复用同一 Proxy 实例
- 构造器保持简单，不依赖外部资源
- Service 层可以更灵活地控制连接生命周期

### 决策 2: CLI 命令拆分 — proxy + tunnel 两组

| 命令组 | 职责 | 命令示例 |
|--------|------|----------|
| `proxy` | SOCKS5 代理 | `assh proxy <server> [-p port]` |
| `tunnel` | 端口转发管理 | `assh tunnel start/stop/list` |

**理由**：
- SOCKS5 代理和端口转发是用户心智模型中不同的功能
- 避免命令组过于臃肿
- proxy 更像"开/关"，tunnel 需要多子命令管理生命周期

### 决策 3: SOCKS5 认证 — NO AUTH + USERNAME/PASSWORD

实现 RFC 1928 标准中的两种认证方式：

| 方法值 | 认证方式 | 启用条件 |
|--------|----------|----------|
| 0x00 | NO AUTH | 默认（未指定 `--auth` 时仅提供此方式） |
| 0x02 | USERNAME/PASSWORD | 指定 `--auth user:pass` 时启用 |

握手时根据客户端支持的方法列表**协商选择**，优先级：
`USERNAME/PASSWORD(0x02) > NO AUTH(0x00)`

**理由**：
- NO AUTH 满足大部分场景（代理本身已受 SSH 加密保护）
- 用户名密码认证提供额外访问控制层
- 两者兼顾兼容性和安全性

### 决策 4: 后台驻留 — Fork + PID 文件

| 参数 | 行为 |
|------|------|
| 无 `--daemon`（默认） | 前台阻塞，等待 Ctrl+C 信号 |
| `--daemon` | Fork 子进程 → 写 PID 文件 → 父进程退出 |

PID 文件路径：`~/.assh/v2/tunnels/{id}.pid`

**理由**：
- 前台模式最简洁，适用于交互式使用
- Daemon 模式适用于长期运行的场景（如 SOCKS5 代理作为系统代理）
- PID 文件便于 `tunnel stop` 定位进程

### 决策 5: 重连策略 — CLI 参数 + 服务器配置

| 控制源 | 格式 | 示例 |
|--------|------|------|
| CLI 参数 `--auto-reconnect` | `[retries/]interval` | `5/10s`（重试 5 次，间隔 10 秒） |
| Server.Options["auto_reconnect"] | 同上的 string | `"3/5s"` |
| 默认（不配置） | - | 断开即停，不重连 |

重连循环：断开 → 检查 ctx.Done() → 等待 interval → 重连 → 重启所有隧道 → 成功/达上限

**理由**：
- 参数最高优先级，满足临时需求
- 配置提供持久化，按服务器管理
- 默认不重连保持简单和安全

### 决策 6: 端口转发规格 — 完整 `[bind_addr:]port:host:hostport`

对齐 ssh 命令，支持完整格式：
```
-L [bind_addr:]port:host:hostport
-R [bind_addr:]port:host:hostport
```

支持同时指定多个 `-L` 和 `-R`。

**理由**：
- 用户习惯与 ssh 一致，零学习成本
- 多转发减少命令执行次数

### 决策 7: 直连支持 — proxy 支持，tunnel 仅数据库

| 命令 | 直连 | 理由 |
|------|:----:|------|
| `proxy` | ✅ | SOCKS5 常用于临时场景，用户可能未保存服务器 |
| `tunnel start` | ❌ | 隧道通常是长期配置，建议先保存服务器管理 |

### 决策 8: TunnelManager.StartAll — 暂缓实现

当前接口仅提供 `Add/Remove/Get/List/StopAll`，不提供 `StartAll`。
隧道 Add 后默认不自动启动，需单独调用 Start。`StartAll` 留到后续迭代（Phase 10+）按需实现。

## Consequences

### 正面影响

1. **Start 传入 client 的设计**使 Service 层能统一管理重连、生命周期
2. **proxy + tunnel 拆分**职责清晰，命令不会过重
3. **双认证方式**兼顾易用性和安全性
4. **前台/后台双模式**覆盖交互式和长期运行场景
5. **多级重连配置**灵活可控
6. **完整转发规格**对齐 ssh 用户习惯
7. **直连控制**平衡灵活性和规范性

### 负面影响

1. USERNAME/PASSWORD 认证需要对 SOCKS5 握手流程做扩展修改
2. Fork + PID 文件增加了跨平台处理复杂度（特别是 Windows）
3. 重连逻辑增加了 TunnelManager 实现的复杂度
4. `Start` 方法接收 client 参数要求调用方确保 client 存活期

### 后续待定事项

- TunnelManager.StartAll() 按需实现
- UDP ASSOCIATE 支持（SOCKS5）
- SOCKS5 BIND 命令支持
- Windows 下 daemon 模式适配
- 多跳/跳板机中的隧道支持（Phase 10）

## Related

- ADR-001: 采用 port/service/infra 三层架构模式
- ADR-011: SSH 连接层架构设计
- ADR-012: CLI 参数对齐与命令架构
- RFC 1928 — SOCKS Protocol Version 5
- RFC 1929 — Username/Password Authentication for SOCKS V5
