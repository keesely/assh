# Phase 10 跳板机方案 vs SSH -J 对比分析

> 对比来源：`man ssh`(1) `-J destination` + `man ssh_config`(5) `ProxyJump`
> 日期：2026-05-19

---

## 一、功能覆盖矩阵

| # | 功能 | SSH -J | ASSH Phase 10 设计 | 差距 |
|---|------|--------|-------------------|------|
| 1 | **多跳串联** | `-J A,B,C` | `-J A,B,C` | ✅ 一致 |
| 2 | **直连参数** | `user@host[:port]` | `user@host[:port]` | ✅ 一致 |
| 3 | **服务器名引用** | ❌ | `-J bastion` (DB 查询) | ✅ ASSH 增强 |
| 4 | **IPv6 地址** | `[::1]:22` | 未定义 | ⚠️ 缺失 |
| 5 | **scp/sftp 支持** | `scp -J` / `sftp -J` | `assh-fs push -J` / `pull -J` | ✅ 一致 |
| 6 | **本地转发 -L + -J** | `ssh -L 8080:host:80 target -J bastion` | `assh tunnel start -L 8080:host:80 -J bastion` | ✅ 一致 |
| 7 | **Config 默认跳板** | `~/.ssh/config` 中 `Host *.internal → ProxyJump bastion` | ❌ 未设计 | 🔴 **缺失** |
| 8 | **Config 隔离规则** | 命令行参数**不应用于跳板机**，跳板机配置走 ssh_config | ❌ 未定义 CLI flage 是否透传到跳板机 | ⚠️ 需定义 |
| 9 | **-J none 禁用** | `-J none` 禁用继承跳板 | ❌ 未定义 | ⚠️ 缺失 |
| 10 | **每跳 HostKey 验证** | 每跳独立验证 known_hosts | ❌ 全部 InsecureIgnoreHostKey | 🔴 **安全差距** |
| 11 | **每跳错误报告** | 明确输出哪一跳失败 | ❌ 未定义错误格式 | ⚠️ 需补充 |
| 12 | **每跳 Keepalive** | 可配置 ServerAliveInterval | ❌ 仅目标连接有 keepalive | ⚠️ 长连接风险 |
| 13 | **每跳 Agent 转发** | 默认不转发（-A 需显式），跳板机独立认证 | ❌ 未定义 -A 行为 | ⚠️ 需定义 |
| 14 | **Auth 复用** | ❌ 每次需重新认证 | ✅ `-J @id` 复用加密存储的凭据 | ✅ ASSH 增强 |
| 15 | **历史记录** | ❌ 无 | ✅ `assh jump history` | ✅ ASSH 增强 |
| 16 | **批量执行 + 跳板** | `for h in s1 s2; do ssh -J bastion $h cmd; done` | `assh bc cmd --servers s1,s2 -J bastion` | ✅ ASSH 增强 |

---

## 二、关键差距详解

### 2.1 Config 默认跳板（#7）

SSH 行为：
```ssh_config
Host bastion
  HostName bastion.example.com

Host *.internal
  ProxyJump bastion
```
→ `ssh internal-web-01.internal` 自动走跳板

ASSH 目前缺失，**建议用 Options 实现**：
```bash
assh set internal-web-01 -o jump_hosts=bastion
# 之后连接自动走跳板
assh login internal-web-01  # → bastion → internal-web-01
```

`ConnectByName` 检测 `Options["jump_hosts"]` 时自动调用 `ConnectChain`。
**不改 domain.Server，不改 DB 表**。

### 2.2 Config 隔离规则（#8）

SSH 语义：「命令行 flags 应用于目标主机，不应用于跳板机」。
例如 `ssh -i ~/.ssh/target_key internal -J bastion` — `-i` 只作用于 `internal`，不作用于 `bastion`。

ASSH 需要同样语义：
- `-P`（密码）→ 只用于目标，不传给跳板机
- `-i`（密钥）→ 只用于目标
- 跳板机的认证来自服务器配置或历史记录的 auth_data

### 2.3 每跳 HostKey 验证（#10）

当前 ASSH Connector 使用 `InsecureIgnoreHostKey()`。
SSH 默认验证 `known_hosts`。

建议：跳板机连接复用同样的 HostKey 策略，至少不降低现有安全性。
但这不是 Phase 10 独有的问题（整体 HostKey 策略待后续 Phase 统一处理），**不作为阻拦**。

### 2.4 每跳错误报告（#11）

当前 Connect 失败返回：
```
ssh dial failed: dial tcp 10.0.0.1:22: i/o timeout
```

多跳时需要明确标注哪一跳：
```
jump[0] bastion (10.0.0.1:22): dial tcp ...
jump[1] gateway (10.0.1.5:22): dial tcp ...
target internal-web-01: ssh: handshake failed
```

设计中已考虑这点（见 Phase10-Design.md 的 `connectThrough` 错误包装）。

### 2.5 Keepalive 透传（#12）

SSH 的 ServerAliveInterval 可配置在每台跳板机上。
ASSH 目前只在目标连接上配置 keepalive。

建议：对中间跳板连接也应用 Keepalive，防止 NAT 超时断开。
实现简单：在 `connectThrough` 中调用 `setupKeepalive`。

### 2.6 IPv6（#4）

`ssh -J [::1]:22 target` — IPv6 地址需方括号包裹。

建议：在 `parseJumpChain` 中加入 IPv6 地址识别：
- 检测 `[...]` 包裹的地址
- 或通过 `net.ParseIP` + `strings.Count(host, ":") > 1`

---

## 三、建议补充项

### 优先级 P0（必须补）

| 项目 | 改动 | 原因 |
|------|------|------|
| Config 默认跳板 | `ConnectByName` + `Options["jump_hosts"]` | SSH 核心能力缺失 |
| Config 隔离规则 | CLI flags 只传给目标，跳板用自身配置 | 安全+语义一致性 |
| 每跳 Keepalive | `connectThrough` 中调用 `setupKeepalive` | 长连接稳定性 |
| IPv6 支持 | `parseJumpChain` 识别 `[...]` | 标准能力 |
| `-J none` 禁用 | 检查 `-J none` 跳过跳板 | 标准能力 |

### 优先级 P1（建议补）

| 项目 | 改动 | 原因 |
|------|------|------|
| 每跳错误报告 | 带索引的错误包装（设计中已有） | 用户体验 |
| -A 标志 | 定义 agent 转发行为 | SSH 标准 `-A` 交互 |

### 预留（非 Phase 10）

| 项目 | 原因 |
|------|------|
| 每跳 HostKey 验证 | 全局 HostKey 策略待统一 Phase 处理 |
| ProxyCommand 支持 | 太底层，ProxyJump 已够用 |
| ControlMaster 复用 | 复杂，后续再评估 |

---

## 四、结论

ASSH Phase 10 设计在功能广度上**超过** SSH `-J` （历史记录/@id 复用/批量执行），但在以下几个 SSH 标准行为上有缺失：

1. **Config 默认跳板** — 最严重缺失，建议用 `Options["jump_hosts"]` 补齐
2. **Config 隔离规则** — CLI flags 不应透传到跳板机
3. **IPv6** — 方括号语法
4. **每跳 Keepalive** — 稳定性的重要保障
5. **-J none 禁用** — 标准语义

建议在实现前将 P0 项补入设计文档。
