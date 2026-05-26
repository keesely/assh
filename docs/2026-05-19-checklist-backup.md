# 项目清单备份

> 日期: 2026-05-19
> 分支: `dev/v2.0.0-phase-10`
> 环境: assh v2.0.0 (Phase 10 进行中)

---

## 1. 项目状态总览

| 项目 | 值 |
|------|-----|
| 版本 | v2.0.0 |
| 当前分支 | `dev/v2.0.0-phase-10` |
| 最新提交 (assh-builder) | `e7c941d` docs: BUG-011 bug record file |
| 最新提交 (asshv2 worktree) | `42f6bf7` feat: Phase 10 跳板机支持 P10.1-P10.4 |
| 编译状态 | ✅ 通过 (`go build ./...`) |

---

## 2. Phase 进度

| Phase | 内容 | 状态 |
|-------|------|------|
| 0-9 | 基础架构→SSH连接→CLI→SFTP→密钥→代理→云同步→健康检查 | ✅ 全部完成 |
| **10** | **跳板机/堡垒机支持** | **🆕 进行中** |
| 11 | 远程服务器挂载 | ⏳ |
| 12 | 自动更新/升级 | ⏳ |
| 13 | 远程命令执行增强 | ⏳ |
| X1 | AI 安全检查 | 🆕 进行中 |
| X2 | 虚拟命令投放与执行 | ⏳ |
| X3 | 堡垒机管理 | ⏳ 排队中 |

---

## 3. Phase 10 任务清单

### 已完成 (P10.1-P10.4)

| 编号 | 任务 | 文件 | 说明 |
|------|------|------|------|
| ✅ P10.1-a | JumpHistory 领域实体 | `asshc/domain/jump.go` | JumpHistory + JumpHopAuth 类型 |
| ✅ P10.1-b | SSHConnector 接口扩展 | `asshc/port/connector.go` | 新增 ConnectChain(target, chain) |
| ✅ P10.1-c | JumpHistoryRecorder 接口 | `asshc/port/jump.go` | Record/List/Get/Delete/Clear/IncrementUse |
| ✅ P10.2-a | 多跳连接实现 | `asshc/infra/ssh/client.go` | ConnectChain + chainClient + buildConfig 提取 |
| ✅ P10.2-b | 数据库迁移 | `asshc/infra/store/db.go` | jump_history 表 |
| ✅ P10.2-c | JumpHistory 存储 | `asshc/infra/store/jump.go` | CRUD + AES-GCM 加密 |
| ✅ P10.3-a | ConnectChain 服务 | `asshc/service/connect.go` | ConnectChain + parseJumpChain |
| ✅ P10.3-b | JumpService | `asshc/service/jump.go` | RecordJumpChain + List/Delete/Clear |
| ✅ P10.4-a | login -J 支持 | `cmd/connect.go` | loginAction 识别 -J/--jump |
| ✅ P10.4-b | run -J 支持 | `cmd/connect.go` | runAction 识别 -J |
| ✅ P10.4-c | bc -J 支持 | `cmd/connect.go` | bcAction 识别 -J |

### 待完成 (P10.5-P10.9)

| 编号 | 任务 | 优先级 | 说明 |
|------|------|--------|------|
| ⏳ P10.5-a | assh-fs push -J 支持 | P1 | push 命令 -J 标志 |
| ⏳ P10.5-b | assh-fs pull -J 支持 | P1 | pull 命令 -J 标志 |
| ⏳ P10.6-a | proxy -J 支持 | P1 | proxy 命令 -J 标志 |
| ⏳ P10.6-b | tunnel -J 支持 | P1 | tunnel 命令 -J 标志 |
| ⏳ P10.7-a | jump history 命令 | P1 | assh jump history [list/--clear/--delete] |
| ⏳ P10.7-b | 历史自动记录 | P1 | 有 direct 跳板时自动写历史 |
| ⏳ P10.7-c | -J @id 复用 | P1 | history-ref 解析 |
| ⏳ P10.8-a | DI 注入 assh | P0 | JumpService + JumpHistoryRecorder 注入 |
| ⏳ P10.8-b | DI 注入 assh-fs | P1 | assh-fs 的 jump 支持 |
| ⏳ P10.9-a | Options 默认跳板 | P0 | ConnectByName 检查 Options["jump_hosts"] |
| ⏳ P10.9-b | Config 隔离规则 | P0 | CLI flags 不传给跳板机 |
| ⏳ P10.9-c | -J none 禁用 | P0 | `-J none` 跳过跳板逻辑 |
| ⏳ P10.9-d | IPv6 方括号解析 | P0 | parseJumpChain 识别 `[...]` |
| ⏳ P10.9-e | 每跳 Keepalive | P0 | dialThrough 中 setupKeepalive |
| ⏳ P10.9-f | 每跳错误报告 | P0 | 带索引的错误包装 |

---

## 4. Bug 追踪

| ID | 标题 | 状态 | 日期 |
|----|------|------|------|
| BUG-001 | log模块Fatal/Panic测试导致进程退出 | fixed | 2026-05-08 |
| BUG-002 | lib/qiniu getCfg不可达代码 | fixed | 2026-05-08 |
| BUG-003 | log级别过滤测试期望值颠倒 | fixed | 2026-05-08 |
| BUG-004 | 顶层默认 Action 短参数（-p/-P/-u 等）未解析 | fixed | 2026-05-10 |
| BUG-005 | assh info 不显示 Password 和 KeyFile 详情 | fixed | 2026-05-10 |
| BUG-006 | assh-fs sftp 连接后立即退出，无法进入交互模式 | fixed | 2026-05-10 |
| BUG-007 | -f/--skip 覆盖控制参数未传递到 infra 层 | fixed | 2026-05-11 |
| BUG-008 | --resume 参数未从 service 层传递到 infra 层 | fixed | 2026-05-11 |
| BUG-009 | ComputeRemoteSHA256 是 stub，-S 校验总是失败 | fixed | 2026-05-11 |
| BUG-010 | SSH 密钥认证失败后未回退到密码认证 | fixed | 2026-05-11 |
| BUG-011 | 默认 Action 中 -J 在 target 后未解析 | fixed | 2026-05-19 |

---

## 5. 架构决策 (ADR) 清单

| ID | 标题 | 状态 | 日期 |
|----|------|------|------|
| ADR-001 | port/service/infra 三层架构 | accepted | 2026-05-08 |
| ADR-002 | asshc 顶层目录设计 | accepted | 2026-05-08 |
| ADR-003 | 依赖注入策略 | accepted | 2026-05-08 |
| ADR-004 | crypto 模块移动 | accepted | 2026-05-08 |
| ADR-005 | 日志 Fatal/Panic 处理 | accepted | 2026-05-08 |
| ADR-006 | 日志系统简化 | accepted | 2026-05-08 |
| ADR-007 | CLI 日志参数简化 | accepted | 2026-05-08 |
| ADR-008 | SQLite 替代加密 JSON 文件 | accepted | 2026-05-09 |
| ADR-009 | AES-256-GCM 静态密钥加密 | accepted | 2026-05-09 |
| ADR-010 | AuthService 延后至 Phase 8 | accepted | 2026-05-09 |
| ADR-011 | SSH 接口分离+可配置 HostKey+Keepalive | accepted | 2026-05-09 |
| ADR-012 | CLI 参数对齐与命令架构 | accepted | 2026-05-09 |
| ADR-013 | 配置版本管理与回滚 | accepted | 2026-05-09 |
| ADR-014 | server 命令组扁平化 | accepted | 2026-05-10 |
| ADR-015 | ls 输出格式优化 | accepted | 2026-05-10 |
| ADR-016 | 顶层默认登录行为 | accepted | 2026-05-10 |
| ADR-017 | SFTP 文件传输架构 | accepted | 2026-05-10 |
| ADR-018 | known-servers 隐性表 | accepted | 2026-05-10 |
| ADR-019 | Hash Header 断点续传 | accepted | 2026-05-10 |
| ADR-020 | 阶段编号调整 | accepted | 2026-05-10 |
| ADR-021 | 短参数合并展开 | accepted | 2026-05-10 |
| ADR-022 | SFTP F1-F28 验收标准 | accepted | 2026-05-10 |
| ADR-023 | 文件名规范 | accepted | 2026-05-10 |
| ADR-024 | v1→v2 配置导入 | accepted | 2026-05-10 |
| ADR-025 | ssh-keygen 参数对齐 | accepted | 2026-05-12 |
| ADR-026 | SSH 密钥备份优先策略 | accepted | 2026-05-13 |
| ADR-027 | 简化 SSH 认证策略 | accepted | 2026-05-13 |
| ADR-028 | 代理&隧道架构 | accepted | 2026-05-13 |
| ADR-029 | Smart Proxy 智能代理系统 | accepted | 2026-05-13 |
| ADR-030 | 跳板机多跳架构设计 | accepted | 2026-05-19 |

**合计**: 30 ADR (30 accepted, 0 proposed, 0 deprecated, 0 superseded)

---

## 6. 已实现模块清单

| 模块路径 | 功能 | Phase |
|----------|------|-------|
| `log/` | 日志模块 (zerolog) | 1 |
| `config/` | 配置管理 (config.go + path.go) | 1 |
| `asshc/infra/crypto/` | RSA/AES/Ed25519/ECDSA 加解密 | 1 |
| `asshc/port/cryptor.go` | Cryptor 接口 | 1 |
| `asshc/domain/` | 领域实体 Server/Auth/ChangelogEntry/JumpHistory | 2+10 |
| `asshc/port/repository.go` | ServerRepository + KnownServerRecorder 接口 | 2+6 |
| `asshc/port/connector.go` | SSHConnector 接口 (Connect/Close/ConnectChain) | 3+10 |
| `asshc/port/session.go` | SSHSession 接口 (Shell/Run/RunWithOutput) | 3 |
| `asshc/port/hostkey.go` | HostKeyChecker 接口 | 3 |
| `asshc/port/jump.go` | JumpHistoryRecorder 接口 | 10 |
| `asshc/port/transfer.go` | FileTransfer 接口 | 5 |
| `asshc/port/proxy.go` | Proxy/PortForward/TunnelManager 接口 | 7 |
| `asshc/port/rules.go` | RuleEngine 接口 (AutoProxy) | 7 |
| `asshc/port/logger.go` | ProxyLogger 接口 (透明日志) | 7 |
| `asshc/port/syncer.go` | Syncer 接口 (云同步) | 8 |
| `asshc/port/health.go` | HealthChecker 接口 | 9 |
| `asshc/port/keymanager.go` | KeyManager 接口 | 6 |
| `asshc/infra/store/` | SQLite 持久化 (Server/Config/Changelog/JumpHistory) | 2+6+10 |
| `asshc/infra/ssh/` | SSH 连接 (认证/HostKey/Keepalive/Shell/Run/Health) | 3+9 |
| `asshc/infra/sftp/` | SFTP 传输 (client/upload/download/header/ops/progress/verify) | 5 |
| `asshc/infra/proxy/` | 代理 (SOCKS5/HTTP/PortForward/TunnelManager/Rules/Logger) | 7 |
| `asshc/infra/sync/qiniu/` | 七牛云同步 | 8 |
| `asshc/service/` | 服务编排 (Server/Connect/Transfer/Proxy/Key/Sync/Health/Jump) | 4+5+6+7+8+9+10 |
| `cmd/` | CLI 命令 + app/bootstrap | 4+5 |
| `cmd/fs/main.go` | assh-fs 独立入口 | 5 |

---

## 7. 文档对齐审计

| 文档 | 发现的问题 |
|------|-----------|
| `AGENTS.md` | 🔴 架构图仍显示 `server add/set/...` 子命令组（应为扁平后的顶层命令）；Phase 10 行仍写"跳板机/堡垒机"（堡垒机已移至 X3）；缺少 Phase 10 新增模块（domain/jump.go, port/jump.go, store/jump.go, service/jump.go） |
| `docs/adrs.md` | 🔴 统计显示 29 accepted（应为 30，缺失 ADR-030）；ADR-029 显示 proposed（应为 accepted） |
| `docs/bugs.md` | 🔴 统计显示"fixed: 10"但列表已有 11 个 bug（计数未更新） |
| `docs/TODO.md` | ✅ Phase 10 清单与完成状态一致；Phase 6 状态仍显示 🔄（应改为 ✅） |
| `docs/Phase10-Design.md` | ✅ 设计文档与实现一致 |

---

## 8. 分支与提交

### assh-builder 主仓库
```
* e7c941d (HEAD -> dev/v2.0.0-phase-10) docs: BUG-011 bug record file
* 2e7dde9 docs: BUG-011 默认 Action 中 -J 在 target 后未解析
* bddfe2e docs: Phase 10 跳板机支持文档对齐
* 6f7058c docs: sync server 命令扁平化 (ADR-014) to USAGE.md
* 6977175 docs: 构建 assh v2 综合使用手册 (Phase 1-9)
* 3ca3922 feat: Phase 9 服务器健康检查完成
```

### asshv2 worktree (assh/.git 工作空间)
```
* 42f6bf7 (HEAD -> dev/v2.0.0-phase-10) feat: Phase 10 跳板机支持 P10.1-P10.4
* dcb7a9e docs: align USAGE.md link to local path in README
* 4ff3084 docs: add comprehensive USAGE.md (Phase 1-9, 1388 lines, 13 chapters)
* 55fda5e docs: update README for Phase 9 (health check) + ADR-014 flattened commands
* 0186bf4 feat: Phase 9 服务器健康检查 - 核心代码实现
```

---

*备份生成时间: 2026-05-19 自动创建*
