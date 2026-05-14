# Phase 5 规划与设计报告

> 日期: 2026-05-10
> 分支: dev/v2.0.0-phase-5
> 状态: ⏳ 设计完成，待开发

---

## 阶段概述

Phase 5 实现 SFTP 文件传输功能，基于 `github.com/pkg/sftp` 库，遵循 port/service/infra 三层架构。

### 已完成的工作

1. **需求澄清** — 通过 5 轮结构化 question 确认全部需求
2. **分支管理** — Phase 4 合并到 v2.0.0 分支 + tag `v2.0.0-phase-4`
3. **ADR-017** — 记录 SFTP 文件传输架构决策
4. **文档更新** — TODO.md / plan.md / AGENTS.md / modules/sftp.md / adrs.md

---

## 需求澄清结果

| 问题 | 决策 |
|------|------|
| CLI 命令设计 | D: push/pull + 交互式 sftp 均支持 |
| 传输特性 | 全部纳入：-r 递归 / -c 断点续传 / 并发 / 进度 / Glob |
| 覆盖策略 | 默认询问，-f/--force、--skip 参数 |
| 测试策略 | 全部纳入：mock + 单元 + 集成 |
| 全局参数 | push/pull/sftp 均支持 -H/-u/-p/-P/-i/-k 直连 |

---

## 架构决策

ADR-017 记录了以下决策：

1. **技术选型**: `github.com/pkg/sftp` v1.13.7+
2. **架构模式**: 沿用三层架构，新增 `infra/sftp/` 目录（5 文件）
3. **覆盖策略**: 询问 / --force / --skip
4. **连接方式**: 已保存服务器 + 直连参数

## 文件清单

| 文件 | 职责 | 状态 |
|------|------|:----:|
| `asshc/infra/sftp/client.go` | SFTP 连接管理 | ⬜ |
| `asshc/infra/sftp/upload.go` | 文件上传（递归/续传/Glob） | ⬜ |
| `asshc/infra/sftp/download.go` | 文件下载（递归/续传/Glob） | ⬜ |
| `asshc/infra/sftp/ops.go` | List/Remove/Mkdir | ⬜ |
| `asshc/infra/sftp/progress.go` | 进度显示组件 | ⬜ |
| `asshc/service/transfer.go` | TransferService + TransferOptions | ⬜ |
| `asshc/service/transfer_test.go` | Mock 测试 | ⬜ |
| `cmd/sftp.go` | push/pull/sftp 命令 | ⬜ |
| `cmd/app.go`（修改） | 注册新命令 + NewApp 签名变更 | ⬜ |
| `main.go`（修改） | DI 注入 | ⬜ |
| `go.mod`（修改） | 新增 pkg/sftp 依赖 | ⬜ |

## 文档对齐清单

| 文档 | 状态 | 操作 |
|------|:----:|------|
| `docs/TODO.md` | ✅ | 更新 Phase 5 详细任务清单 + 验收标准 |
| `docs/plan.md` | ✅ | Phase 5 文件数更新 4→8 |
| `docs/adrs.md` | ✅ | 统计 16→17，新增 ADR-017 条目+摘要 |
| `docs/adrs/017-SFTP-文件传输架构设计.md` | ✅ | 新建 ADR 文件 |
| `AGENTS.md` | ✅ | Phase 状态更新 + SFTP 库条目 |
| `docs/modules/sftp.md` | ✅ | 重构为完整设计文档 |
| `docs/adrs.db` | ✅ | ADR 数据库同步 |

## 待后续确认项

- ⏳ 断点续传的具体实现方案
- ⏳ 并发传输的默认并发数（当前设计 3）
- ⏳ 交互式 SFTP 会话的具体操作命令集

---

## 第 2 轮更新 (2026-05-10) — 二进制拆分 (Binary Split)

### 变更摘要

在需求细化阶段确认将 SFTP 功能拆分为独立二进制 `assh-fs`，与 `assh` 共享核心模块。

### 具体变更

| 范围 | 旧方案 | 新方案 |
|------|--------|--------|
| 二进制 | 单体 `assh`（含 SFTP 命令） | `assh`（SSH 客户端）+ `assh-fs`（文件系统工具） |
| 入口点 | `main.go` | `main.go` + `cmd/fs/main.go` |
| 命令文件 | `cmd/sftp.go` | `cmd/sftp_cmd.go`（共享命令定义） |
| 共享初始化 | 无 | `cmd/bootstrap.go` → `NewAppComponents()` |
| Build 脚本 | 单二进制构建 | 双二进制构建（`assh` + `assh-fs`） |

### ADR-017 更新

- 新增「修正方案：二进制拆分」章节
- 更新架构层级图（assh vs assh-fs 共享核心）
- 更新 Consequences（正面 + 负面影响细化）
- 更新影响范围表（新增文件路径变更、构建脚本改造）

### 文档变更清单

| 文档 | 状态 | 变更内容 |
|------|:----:|----------|
| `docs/adrs/017-SFTP-文件传输架构设计.md` | ✅ | 新增二进制拆分决策章节 |
| `docs/adrs.md` | ✅ | Phase 5 摘要新增二进制拆分说明 |
| `AGENTS.md` | ✅ | 项目结构树更新（cmd/fs/main.go + cmd/bootstrap.go） |
| `asshv2/build.sh` | ✅ | 双二进制构建：assh（`.`）+ assh-fs（`./cmd/fs/`） |

### 关键决策记录

- `-e` 确认为 `--resume` 短参（唯一可用短 flag）
- 所有 CLI 示例更新为 `assh-fs` 前缀（不再有 `assh-sftp`）
- `cmd/sftp_cmd.go` 专注于 CLI 命令定义，与入口逻辑分离
- `cmd/bootstrap.go` 封装 `NewAppComponents()` 供两个组合根共享

```bash
# assh 代码仓库 (worktree asshv2/)
git checkout v2.0.0
git merge --no-ff dev/v2.0.0-phase-4 -m "chore: merge Phase 4 CLI command assembly into v2.0.0"
git tag -a v2.0.0-phase-4 -m "Phase 4: CLI command assembly completed (2026-05-10)"
git checkout -b dev/v2.0.0-phase-5

# assh-builder 配置仓库
git checkout master
git merge --no-ff dev/v2.0.0-phase-5 -m "chore: merge Phase 4 documentation into master"
git tag -a v2.0.0-phase-4 -m "Phase 4: documentation and project config (2026-05-10)"
git branch -D dev/v2.0.0-phase-5
git checkout -b dev/v2.0.0-phase-5
```
