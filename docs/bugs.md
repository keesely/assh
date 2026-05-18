# Bug 追踪

> 记录开发过程中发现的 Bug、已知问题及修复方案

## 统计

| 状态 | 数量 |
|------|------|
| fixed | 10 |
| known | 0 |
| in-progress | 0 |

## 列表

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
| BUG-011 | [默认 Action 中 -J 在 target 后未解析](bugs/BUG-011-default-action-jump-flag.md) | fixed | 2026-05-19 |

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