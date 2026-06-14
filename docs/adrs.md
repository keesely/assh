# 架构决策记录 (ADR)

> 记录项目中的关键架构决策

## 统计

| 状态 | 数量 |
|------|------|
| accepted | 32 |
| proposed | 0 |
| deprecated | 0 |
| superseded | 0 |

## 列表

| ID | 标题 | 状态 | 日期 |
|----|------|------|------|
| ADR-001 | 采用 port/service/infra 三层架构模式 | accepted | 2026-05-08 |
| ADR-002 | asshc 顶层目录设计（无 internal 嵌套） | accepted | 2026-05-08 |
| ADR-003 | 依赖注入策略（构造器注入，组合根在 main.go） | accepted | 2026-05-08 |
| ADR-004 | crypto 模块从 config 移至 asshc/infra/crypto | accepted | 2026-05-08 |
| ADR-005 | 日志 Fatal/Panic 处理方式（exit 函数分离） | accepted | 2026-05-08 |
| ADR-006 | 日志系统简化：移除双日志输出和级别限制 | accepted | 2026-05-08 |
| ADR-007 | CLI 日志参数简化：移除日志级别参数 | accepted | 2026-05-08 |
| ADR-008 | 废弃加密 JSON 文件，改用 SQLite 数据库 | accepted | 2026-05-09 |
| ADR-009 | 服务器密码采用 AES-256-GCM 静态密钥加密 | accepted | 2026-05-09 |
| ADR-010 | AuthService 延后至 Phase 8 | accepted | 2026-05-09 |
| ADR-011 | SSH 连接层架构设计：接口分离 + 可配置 HostKey + Keepalive | accepted | 2026-05-09 |
| ADR-012 | CLI 参数对齐与命令架构：与 ssh 兼容 + 自动识别 + bc 三通道 | accepted | 2026-05-09 |
| ADR-013 | 配置版本管理与回滚机制：快照式 changelog + rollback | accepted | 2026-05-09 |
| ADR-014 | server 命令组扁平化上移顶层 | accepted | 2026-05-10 |
| ADR-015 | ls 命令输出格式优化（Remark列/合并Host列/颜色样式） | accepted | 2026-05-10 |
| ADR-016 | 顶层默认登录行为（v1 兼容）：assh \<target\> 直连 | accepted | 2026-05-10 |
| ADR-017 | SFTP 文件传输架构设计：基于 pkg/sftp 实现文件传输 | accepted | 2026-05-10 |
| ADR-018 | known-servers 隐性表：未具名服务器直连记录 | accepted | 2026-05-10 |
| ADR-019 | 传输文件 Hash Header：断点续传安全验证 | accepted | 2026-05-10 |
| ADR-020 | 阶段编号调整：插入Phase 6密钥管理 | accepted | 2026-05-10 |
| ADR-021 | RQ-001: 短参数合并展开 | accepted | 2026-05-10 |
| ADR-022 | Phase 5 SFTP 功能拆分为F1-F28验收标准 | accepted | 2026-05-10 |
| ADR-023 | 文件名规范：xxx → BUG-XXX-标题.md | accepted | 2026-05-10 |
| ADR-024 | v1→v2 配置导入指令 | accepted | 2026-05-10 |
| ADR-025 | assh keygen ssh-keygen 参数对齐 | accepted | 2026-05-12 |
| ADR-026 | SSH 连接密钥备份优先策略 | accepted | 2026-05-13 |
| ADR-027 | 简化 SSH 认证策略 — 去除两轮认证回退 | accepted | 2026-05-13 |
| ADR-028 | 代理 & 隧道架构设计（SOCKS5/端口转发/隧道管理） | accepted | 2026-05-13 |
| ADR-029 | Smart Proxy 智能代理系统（正向代理+AutoProxy规则+透明日志+多端口规则） | accepted | 2026-05-13 |
| ADR-030 | 跳板机多跳架构设计 | accepted | 2026-05-19 |
| ADR-031 | 自动更新/升级架构设计（GitHub Release + 原子更新 + 回滚） | accepted | 2026-05-26 |
| ADR-032 | 远程服务器挂载方案选型与架构设计（sshfs + WebDAV） | accepted | 2026-05-29 |

## 新增 ADR

格式：`docs/adrs/XXX-标题.md`

内容模板：

```markdown
# ADR-XXX: 标题

## Status
accepted/proposed/rejected/deprecated/superseded

## Date
YYYY-MM-DD

## Context
...

## Decision
...

## Consequences
...
```

## 相关链接

- [ADR 管理工具](../.opencode/skills/adr-manager/README.md)
- [ADR 模板](../.opencode/skills/adr-manager/references/adr-template.md)
- [ADR 状态说明](../.opencode/skills/adr-manager/references/adr-statuses.md)

## Phase 2 ADR 变更摘要

本次 Phase 2 调整为项目引入了三个关键架构变更：

| ADR | 变更 | 影响 |
|-----|------|------|
| ADR-008 | 文件存储 → SQLite | 存储引擎、依赖、数据结构 |
| ADR-009 | AES-CBC+密码 → AES-GCM+静态密钥 | 安全模型、密钥管理 |
| ADR-010 | Phase 2 AuthService → Phase 8 | 职责划分、发布计划 |

详细内容见对应 ADR 文件。

## Phase 3 ADR 变更摘要

本次 Phase 3 引入的关键架构决策：

| ADR | 变更 | 影响 |
|-----|------|------|
| ADR-011 | SSH 接口分离（Connector/Session）+ 可配置 HostKey + Keepalive | 新增 7 个文件，接口职责清晰，策略灵活 |

详细内容见对应 ADR 文件。

## Phase 4 ADR 变更摘要

本次 Phase 4 引入的关键架构决策：

| ADR | 变更 | 影响 |
|-----|------|------|
| ADR-012 | CLI 参数与 ssh 对齐 + 自动识别直连 + bc 三通道输出 | 参数命名规范、命令层级、login 自动识别策略、bc 输出架构 |
| ADR-013 | 配置版本管理与回滚机制：快照式 changelog + rollback | store.Set() 版本递增 + 快照、store.RollbackTo()、service.SetServer() upsert |
| ADR-014 | server 命令组扁平化上移顶层 | server 子命令（add/set/ls/info/rm/mv/rollback）上移为顶层扁平命令 |
| ADR-015 | ls 输出格式优化（Remark 列、合并 Host 列、颜色方案） | ls 6 列输出（Group/Name/Host/Ver/Auth/Remark） |
| ADR-016 | 顶层默认登录行为（v1 兼容） | `assh \<target\>` 直连，不再需要显式 `login` 命令 |

**Phase 4 细化（2026-05-10）**：info 格式重构（居中标题 + Auth Type + 首字母大写标签）、ls 自适应列宽、info/默认 Action 模糊建议（前缀匹配 + Levenshtein）。以上为 UX 增强，未涉及架构变更，无需新增 ADR。

## Phase 5 ADR 变更摘要

本次 Phase 5 引入的关键架构决策：

| ADR | 变更 | 影响 |
|-----|------|------|
| ADR-017 | 引入 pkg/sftp 实现 SFTP 文件传输 + **二进制拆分（assh vs assh-fs）** | 新增依赖、`cmd/fs/main.go` 独立入口、`cmd/bootstrap.go` 共享初始化、`cmd/sftp_cmd.go` 共享命令定义、`infra/sftp/` 下 6 文件、双二进制构建脚本改造 |
| ADR-019 | **Hash Header 断点续传安全验证** | 新增 `infra/sftp/header.go`（256B 固定头部）、修改 upload/download 流程加入 hash 验证、传输完成后剥离头部 |

**二进制拆分要点**：
- `assh`：SSH 客户端主二进制（shell/run/bc/server 管理）
- `assh-fs`：文件系统工具独立二进制（push/pull/sftp 交互）
- 共享核心：`asshc/` 下所有 domain/port/service/infra 模块
- 共享初始化：`cmd/bootstrap.go` 提供 `NewAppComponents()`
- 短期承载 SFTP 文件传输，未来可扩展挂载功能（Phase 12）

详细内容见对应 ADR 文件。

## Phase 6 ADR 变更摘要

本次 Phase 6 引入的关键架构决策：

| ADR | 变更 | 影响 |
|-----|------|------|
| ADR-020 | 阶段编号调整：插入Phase 6密钥管理 | Phase 编号重排 |
| ADR-025 | **ssh-keygen 参数对齐**：新增 `-f`/`-C`/`-N` 参数、`Generate`/`GenerateToPath`/`GenerateAndDeploy` 接口签名增加 `comment` 参数、`keygenInteractive` 无参交互式向导 | Port/infra/service/CLI 四层传导、mock + 测试更新（37 处）|
| ADR-026 | **SSH 连接密钥备份优先策略**：Connector 持有 accountPassphrase + `tryBackupKey` + 三层嵌套目录备份优先 | `sshinfra/client.go` 新增 passphrase 字段/方法、`connect.go` 签名变更、`main.go` 注入顺序调整 |
| ADR-027 | **简化 SSH 认证策略**：去除两轮认证回退 | 认证流程简化，减少延迟和日志噪声 |

详细内容见对应 ADR 文件。

## Phase 7 ADR 变更摘要

本次 Phase 7 引入的关键架构决策：

| ADR | 变更 | 影响 |
|-----|------|------|
| ADR-028 | **代理 & 隧道架构设计**：SOCKS5（NO AUTH + USERNAME/PASSWORD）、本地/远程端口转发（完整 [bind_addr]:port:host:hostport 规格）、TunnelManager 生命周期管理、Fork+PID 文件 daemon 模式、CLI 参数+配置驱动重连、CLI 拆分 proxy/tunnel 两命令组 | port/proxy.go、infra/proxy/（socks5.go+port.go）、service/proxy.go、cmd/proxy.go、cmd/tunnel.go 共约 1300+ 行新增 |
| ADR-029 | **Smart Proxy 智能代理系统**：正向代理统一 SOCKS5+HTTP 入口、AutoProxy 规则引擎（GFWList 兼容）、透明日志（全链路请求/响应记录）、多端口规则（批量参数配置） | 新增 port/rules.go、port/logger.go、infra/proxy/rules/、infra/proxy/http_connect.go、infra/proxy/logger.go，扩展 service/proxy.go、cmd/proxy.go |

详细内容见 [ADR-028](adrs/028-proxy-tunnel-architecture.md) 和 [ADR-029](adrs/029-smart-proxy-system.md)。

## Phase 10 ADR 变更摘要

本次 Phase 10 引入的关键架构决策：

| ADR | 变更 | 影响 |
|-----|------|------|
| ADR-030 | **跳板机多跳架构设计**：基于 ssh.Dial→client.Dial→ssh.NewClientConn 链式连接、ChainClient 全链路 cleanup、JumpHistory 领域实体+AES-GCM 加密 path_data、jump_history 数据库表、parseJumpChain 链式表达式解析（server名/direct/`@id`/IPv6/`-J none`）、Config 隔离规则（CLI flags 不传递跳板机）、Options["jump_hosts"] 默认跳板、每跳 Keepalive+错误报告 | 新增 domain/jump.go、port/connector.go(ConnectChain 扩展)、port/jump.go、infra/ssh/client.go(ConnectChain+chainClient)、infra/store/db.go(jump_history 表)、infra/store/jump.go、service/connect.go(ConnectChain+parseJumpChain)、service/jump.go、cmd/connect.go(-J 标志) |

详细内容见 [ADR-030](adrs/030-jump-host-multi-hop.md)。

## Phase 12 ADR 变更摘要

本次 Phase 12 引入的关键架构决策：

| ADR | 变更 | 影响 |
|-----|------|------|
| ADR-032 | **远程服务器挂载方案选型与架构设计**：sshfs（P0）+ WebDAV（P1）双方案、挂载命令归属 assh-fs、SQLite mounts 表持久化、Go 内置 WebDAV 服务端 + SSH 隧道、自动依赖检测 + 安装提示、sshfs 默认 reconnect、前台 + --daemon 双模式 | 新增 port/mounter.go、infra/mount/（sshfs.go+webdav.go+detect.go+port.go）、infra/store/mount.go、service/mount.go、cmd/sftp_cmd.go 扩展、cmd/fs/main.go DI 变更 |

详细内容见 [ADR-032](adrs/032-远程服务器挂载方案选型.md)。