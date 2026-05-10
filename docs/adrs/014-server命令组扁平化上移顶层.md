# ADR-014: server 命令组扁平化上移顶层

## Status
accepted

## Date
2026-05-10

## Context
v2 当前将服务器管理命令组织为 `assh server <subcommand>` 的子命令组（add/set/ls/info/rm/mv/rollback），需要输入 `assh server` 前缀才能访问这些高频操作。v1 采用扁平命令风格（`assh set` / `assh ls` / `assh info`），用户已习惯。v2 当前顶层仅有 5 个命令（version/login/run/bc/server），命令空间充足。用户反馈当前命令层级太多，不利于日常使用和快速操作。

## Decision
将 server 子命令组整体上移到顶层，变为扁平命令：add/set/ls/info/rm/mv/rollback。移除 server 命令组容器。login/run/bc/version 保持原地不动。变更后顶层命令从 5 个增加到 11 个。

## Consequences

### 正面
- 减少输入层级，用户可用 `assh add <name>` 而非 `assh server add <name>`
- 与 v1 用户习惯一致，降低迁移成本
- 快捷键操作更便捷，效率提升

### 负面
- 顶层命令从 5 个增加到 11 个，略微增加认知负担
- 与未来 Phase 5-7 新增命令（push/pull/sync/proxy）可能产生命名冲突风险，需提前规划命名空间

## Related
- 关联 ADR：ADR-012（CLI 参数对齐与命令架构）
- 涉及文件：`cmd/server.go`、`cmd/app.go`
