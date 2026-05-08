# 架构决策记录 (ADR)

> 记录项目中的关键架构决策

## 统计

| 状态 | 数量 |
|------|------|
| accepted | 7 |
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