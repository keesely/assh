# Bug 追踪

> 记录开发过程中发现的 Bug、已知问题及修复方案

## 统计

| 状态 | 数量 |
|------|------|
| fixed | 3 |
| known | 0 |
| in-progress | 0 |

## 列表

| ID | 标题 | 状态 | 日期 |
|----|------|------|------|
| BUG-001 | log模块Fatal/Panic测试导致进程退出 | fixed | 2026-05-08 |
| BUG-002 | lib/qiniu getCfg不可达代码 | fixed | 2026-05-08 |
| BUG-003 | log级别过滤测试期望值颠倒 | fixed | 2026-05-08 |

## 新增 Bug

格式：`docs/bugs/XXX-简短描述.md`

内容模板：

```markdown
# BUG-XXX: 标题

## Status
fixed/known/in-progress

## Date
YYYY-MM-DD

## Description
问题描述...

## Solution
修复方案...

## Affected Files
- file1.go
- file2.go
```

## 相关链接

- [ADR 总览](adrs.md) - 架构决策记录