# 需求收集

> 本目录用于记录用户临时需求、功能请求和待确认的改进建议。
> 每个需求独立文件，包含标准元数据用于追踪状态。
> 在阶段开始和复盘时阅读，自主决定是否在当前或后续阶段纳入。

---

## 元数据规范

每个需求文件使用 YAML front matter 作为元数据头：

```yaml
---
id: RQ-XXX              # 需求编号，递增
title: 简短标题          # 一行概述
status: proposed        # 状态（见下方状态定义）
priority: medium        # 优先级：critical / high / medium / low
created: 2026-05-10     # 创建日期
updated: 2026-05-10     # 最后更新日期
source: user            # 来源：user / self / external
phase: none             # 计划纳入的阶段（none 表示未排期）
related_adr: []         # 关联的 ADR 编号
related_rq: []          # 关联的需求编号
tags: [cli, ux]         # 标签
---
```

## 需求状态定义

| 状态 | 含义 | 操作 |
|------|------|------|
| `proposed` | 刚提出，未澄清 | 需要澄清和细化 |
| `clarifying` | 正在澄清中 | 与需求方沟通细节 |
| `accepted` | 需求明确，待排期 | 排入某个 Phase |
| `in_progress` | 正在实现 | 分配给 build agent |
| `implemented` | 已实现 | 更新实现细节 |
| `rejected` | 已拒绝 | 记录拒绝原因 |
| `deferred` | 推迟到后续阶段 | 记录原因和预计阶段 |
| `deprecated` | 已废弃（后被取代） | 记录替代方案 |

## 生命周期流程

```
proposed → clarifying → accepted → in_progress → implemented
                    ↘ rejected
                    ↘ deferred
                                    implemented → deprecated
```

## 工作流

1. **录入**：新需求 → 创建 `.md` 文件，状态 `proposed`
2. **澄清**：阶段开始时阅读，与需求方沟通细节
3. **决策**：决定是否纳入当前阶段（状态 → `accepted`）或推迟（状态 → `deferred`）
4. **实现**：分配给 build agent，状态 → `in_progress`
5. **验收**：完成后更新状态 → `implemented`，记录实现细节

## 索引

| ID | 标题 | 状态 | 优先级 | 阶段 | 创建日期 |
|----|------|------|--------|------|----------|
| RQ-001 | CLI 组合短参数支持（-qnt 风格） | accepted | medium | Phase 5 | 2026-05-10 |
| RQ-002 | Session 管理 — 守护进程生命周期管理 | proposed | high | NEXT-001 | 2026-06-04 |
