# Phase 7 需求澄清与文档对齐工作总结

## 日期
2026-05-13

## 概述
Phase 6 验收通过后，对 Phase 7（代理 & 隧道）进行需求澄清、文档对齐和架构决策记录。
在此之前，核心接口和基础设施实现（SOCKS5 + 端口转发）已完成，但编排层、CLI、测试和 DI 尚未开始。

## 本轮完成工作

### 1. 需求澄清（多轮 question）

通过与用户的逐轮确认，明确了以下关键设计决策：

| 决策项 | 结论 | 影响 |
|--------|------|------|
| CLI 命令结构 | `proxy` + `tunnel` **分开两组** | 两个命令文件，独立的子命令体系 |
| SOCKS5 默认端口 | **1080** | 对齐标准 |
| SOCKS5 认证 | **NO AUTH + USERNAME/PASSWORD 双支持** | 扩展握手流程，新增 `authFunc` 回调 |
| 后台驻留 | **Fork + PID 文件**（`--daemon` 参数） | 新增 daemon 工具函数，PID 文件路径 `~/.assh/v2/tunnels/{id}.pid` |
| 重连策略 | **CLI 参数 + 服务器配置** | `--auto-reconnect=[retries/]interval` 参数 + Server.Options["auto_reconnect"] |
| 端口转发规格 | **完整 `[bind_addr:]port:host:hostport`** | 对齐 ssh -L/-R 格式 |
| 多转发支持 | **支持**同时指定多个 `-L`/`-R` | 同一个 tunnel start 命令可启动多个转发 |
| proxy 直连 | **支持** `-H/-u/-P/-i` 参数 | 无需提前保存服务器 |
| tunnel 直连 | **仅数据库模式** | 需要先保存服务器到数据库 |
| TunnelManager.StartAll | **暂缓**，后续迭代 | 当前仅 Add/Remove/List/Get/StopAll |

### 2. 文档对齐

#### 设计文档全面重写
- **`docs/modules/proxy-tunnel.md`** — 全面重写（231行→约320行）：
  - 接口签名对齐到实际代码（`Start(client *ssh.Client, ...)`）
  - 补充 SOCKS5 USERNAME/PASSWORD 认证设计
  - 新增后台驻留（Fork + PID）策略
  - 新增重连策略（参数 + 配置驱动）
  - 更新 CLI 命令规格（完整的 proxy + tunnel 参数表）
  - 更新端口转发规格解析表
  - 更新数据流图（增加 tunnel 命令组）
  - 更新依赖关系图和错误处理策略
  - 新增生命周期流程图

#### 架构文档修正
- **`docs/architecture.md`**：
  - 修正二进制架构图：assh 增加 proxy/tunnel/keygen 命令
  - DI 流程增加 TunnelManager + ProxyService（标注待实现）
  - 移除不存在的 `infra/tunnel/` 目录引用（合并到 `infra/proxy/`）

#### 存储文档更新
- **`docs/storage.md`**：新增隧道 PID 文件路径 `~/.assh/v2/tunnels/{id}.pid`

#### 过期文档清理
- **`docs/modules/proxy.md`**：标记为废弃，添加跳转到新文档的说明

### 3. 架构决策记录

- **创建 ADR-028**：`docs/adrs/028-proxy-tunnel-architecture.md`
  - 记录 8 项关键架构决策（SSH 生命周期/CLI 拆分/SOCKS5 认证/后台驻留/重连策略/端口转发规格/直连控制/StartAll 暂缓）
  - 包含正面/负面影响分析及后续待定事项

- **更新 ADR 索引**：`docs/adrs.md`
  - 统计数量：27→28 accepted
  - 列表新增 ADR-028 条目
  - Phase 7 摘要更新为完整描述

### 4. TODO.md 更新

- 新增 P7.2-a-2: SOCKS5 USERNAME/PASSWORD 认证扩展
- 更新所有待实现任务的详细说明和优先级
- 更新参数规格（完整命令示例 + 端口转发解析表）
- 更新阶段优先级顺序

## 文档对齐清单完成情况

| 文档 | 操作 | 状态 |
|------|------|:----:|
| `docs/TODO.md` | 更新 Phase 7 任务状态和规格 | ✅ |
| `docs/plan.md` | 无需更新 | ⬜ |
| `AGENTS.md` | 已完成模块列表已有 proxy 条目，无需补充 | ✅ |
| `docs/constraints.md` | 不涉及新依赖 | ⬜ |
| `docs/architecture.md` | 修正目录引用、更新架构图和 DI 流程 | ✅ |
| `docs/storage.md` | 补充 tunnels PID 文件路径 | ✅ |
| `docs/modules/proxy-tunnel.md` | 全面重写 | ✅ |
| `docs/modules/proxy.md` | 标记废弃 | ✅ |
| `docs/adrs.md` | 索引更新 + Phase 7 摘要 | ✅ |
| `docs/adrs/028-proxy-tunnel-architecture.md` | 新建 | ✅ |
| adr-manager 数据库同步 | 待执行 | ⬜ |

## 发现的问题

### 文档与代码不一致（已修复）

| 不一致项 | 设计文档（旧） | 实际代码 | 处理 |
|----------|---------------|----------|------|
| Proxy.Start 签名 | `Start(localAddr)` | `Start(client, localAddr)` | 文档对齐代码 |
| PortForward.Start 签名 | `Start()` | `Start(client)` | 文档对齐代码 |
| TunnelManager.Add 返回值 | `(id, error)` | `error` | 文档对齐代码 |
| SOCKS5Proxy 构造 | `NewSOCKS5Proxy(client)` | `NewSOCKS5Proxy()` | 文档对齐代码 |
| 文件名 | `port_forward.go` | 实际 `port.go` | 文档对齐代码 |
| tunnel/ 目录 | 存在 | 不存在 | architecture.md 修正 |

### 接口签名修正待办（代码未改，仅文档对齐）

用户指示"后续修正"，当前保持代码不变。待后续实现阶段评估：
- `TunnelManager.Add` 返回值是否需要返回 ID（用户需要 stop 时定位隧道）
- 目前 ID 由 `LocalForward.ID()` / `RemoteForward.ID()` 在构造时生成，Manager 只需保留引用

## 关键设计决策摘要

Phase 7 的设计理念：**灵活可控、渐进迭代**。
- CLI 拆分方便用户理解和使用
- 参数驱动的后台/前台模式适应不同使用场景
- 多级重连配置兼顾简单和灵活
- SOCKS5 双认证方式平衡安全性和易用性

## 引用
- ADR-028: 代理 & 隧道架构设计
- 模块设计文档: `docs/modules/proxy-tunnel.md`
- 前序工作: `2026-05-13-Phase7-Init.md`

## Git 日志
（本次为文档工作，未涉及代码提交）
