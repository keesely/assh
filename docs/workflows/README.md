# 工作流文档

> 本目录记录本项目在实际开发过程中沉淀的**工作流程**和**可复用经验**。
> 这些流程是跨 Phase 的通用模式，适用于后续所有开发阶段。

## 文档索引

| 文档 | 内容 | 适用角色 |
|------|------|----------|
| [phase-lifecycle.md](phase-lifecycle.md) | Phase 完整生命周期：从需求到交付的 8 步流程 | PM Agent, Build Agent |
| [document-management.md](document-management.md) | 文档创建/更新/对齐/ADR 的管理规范 | PM Agent |
| [lessons-learned.md](lessons-learned.md) | 抽象复盘与可复用经验（含反模式） | 全部 |

## 核心原则

```
需求澄清先行 → 架构决策记录 → 文档对齐 → 代码实现 → 验收闭合
```

- **需求澄清先行**：未澄清的需求不进入设计
- **架构决策记录**：每个重大决策必须有 ADR
- **文档对齐**：设计完成后所有相关文档同步更新
- **验收闭合**：验收标准在实现前定义，实现后逐项验证

## 工具链

| 工具 | 用途 | 使用场景 |
|------|------|----------|
| `question` | 多轮需求确认 | Phase 启动/需求澄清 |
| `write` / `edit` | 文档创建/更新 | 设计完成后文档对齐 |
| `bash (git)` | 版本/分支管理 | 提交/合并/打 tag |
| `task (build agent)` | 代码实现 | 设计完成后的编码阶段 |
| `skill → adrs-manager` | ADR 管理 | 架构决策记录 |
| `skill → git-manager` | 复杂 git 操作 | 多分支合并/Changelog |
| `todowrite` | 任务追踪 | 复杂多步骤任务 |

## 分支模型

```
master
  └── dev/v2.0.0-phase-N       ← 每个 Phase 的独立开发分支
        ├── feat/<feature>      ← 特性分支（从 Phase 分支拉出）
        └── bugfix/<issue>      ← Bug 修复分支
```
