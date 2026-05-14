# Phase 5 文档对齐与验收收尾

> 2026-05-11 | PM Agent

## 任务

Phase 5 验收结束后，执行文档对齐清单检查，确保所有相关文档与项目实际状态一致。

## 文档对齐清单执行结果

### 核心计划文档
- [x] `docs/TODO.md` — 已反映 Phase 5 完成（F1-F28 ✅）
- [x] `docs/plan.md` — **已更新** Go 版本 1.22.0 → 1.25.0

### 工程元文档
- [x] `AGENTS.md` — Phase 5 状态 ✅（前期已提交）
- [x] `docs/constraints.md` — **已更新** Go 版本 1.22.0+ → 1.25.0+
- [x] `docs/architecture.md` — 架构图和数据流已准确（双二进制架构 + DI 注入流程均已反映 Phase 5）
- [x] `docs/storage.md` — 存储路径和数据库结构准确，无需更新

### 模块文档
- [x] `docs/modules/sftp.md` — **已更新**：
  - `ProgressInfo` → `TransferInfo`（新增 `Bytes`/`TotalBytes` 字段）
  - 接口签名加入 `opts TransferOptions` 参数
  - 新增 `TransferOptions` 结构体定义

### 架构决策
- [x] `docs/adrs.md` 索引 — 全部 24 条 ADR 记录齐全（ADR-001 ~ ADR-024）
- [x] `docs/adrs/` — 24 个 ADR 文件全部存在
- [x] adrs-manager 数据库 — **已同步**：
  - 合并两个工作空间 `assh` + `assh-builder` → 统一为 `assh`
  - 共 24 条 ADR 记录，全部状态 `accepted`
  - 覆盖 ID 1~24（存在历史重复记录 ID 1-5 和 ID 8-12，为早期数据导入遗留）
  - 无需新建 ADR（本阶段无重大架构变更）

### 工作报告
- [x] `docs/worklogs/` — 本工作日志已记录

### 未发现需修复项
- 代码与文档接口签名一致（`port/transfer.go` vs `docs/modules/sftp.md` 对齐 ✅）
- 依赖列表完整（sftp v1.13.7+ / zerolog / sqlite / urfave/cli 等）
- 存储结构准确（`~/.assh/v2/asshv2.db` + servers/config/server_changelog 三表）

## 变更摘要

| 文件 | 变更 |
|------|------|
| `docs/plan.md` | Go 版本 1.22.0 → 1.25.0 |
| `docs/constraints.md` | Go 版本 1.22.0+ → 1.25.0+ |
| `docs/modules/sftp.md` | 接口签名同步 `TransferOptions` + `TransferInfo` |
| `adrs.db` (project level) | `assh-builder` → `assh` 工作空间合并 |
| `docs/worklogs/2026-05-11-Phase5-Doc-Alignment.md` | 本工作日志 |
