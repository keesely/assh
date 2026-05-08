# 架构决策记录 (ADR)

> 记录项目中的关键架构决策

## 统计

| 状态 | 数量 |
|------|------|
| accepted | 10 |
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
| **ADR-008** | **废弃加密 JSON 文件，改用 SQLite 数据库** | **accepted** | **2026-05-09** |
| **ADR-009** | **服务器密码采用 AES-256-GCM 静态密钥加密** | **accepted** | **2026-05-09** |
| **ADR-010** | **AuthService 延后至 Phase 6** | **accepted** | **2026-05-09** |

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
| ADR-010 | Phase 2 AuthService → Phase 6 | 职责划分、发布计划 |

详细内容见对应 ADR 文件。