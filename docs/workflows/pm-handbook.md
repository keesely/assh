# PM Agent 实用手册

> 本手册为 assh v2.0.0 项目的 PM Agent 工作方法论总结。
> 适用于 AI 辅助开发场景，可作为新项目启动的参考模板。

---

## 1. 快速启动清单

每次新 Phase 开始时，按顺序执行：

```
□ 1.1 审查 AGENTS.md — 确认项目状态和技术栈
□ 1.2 审查 docs/TODO.md — 查看本 Phase 原始任务
□ 1.3 审查 docs/plan.md — 确认技术选型和依赖
□ 1.4 审查 docs/adrs.md — 查看已有架构决策
□ 1.5 创建 Phase 分支 — git checkout -b dev/v2.0.0-phase-N
□ 1.6 需求澄清 — 使用 question 工具多轮确认
□ 1.7 术语定义 — 发现歧义词立即定义
□ 1.8 架构决策 — 创建/更新 ADR
□ 1.9 方案设计 — 编写模块文档 + TODO 任务表
□ 1.10 文档对齐 — 批量更新所有关联文档
□ 1.11 代码交接 — task 工具交给 Build Agent
□ 1.12 验收测试 — 逐项验证 F1~F{N}
□ 1.13 合并发布 — git merge + tag
```

---

## 2. 需求澄清框架

### 2.1 question 工具最佳实践

**结构化提问模板**：

```
Q: [场景描述]
  A) [方案A] — [优点] / [缺点]
  B) [方案B] — [优点] / [缺点]
  C) [方案C] — [优点] / [缺点]
```

**提问节奏**：
- 每轮 1-3 个问题，不超过 5 个
- 先范围后细节，先方案后边界
- 记录每个决策和理由

### 2.2 常见澄清场景

| 场景 | 关注点 |
|------|--------|
| CLI 参数设计 | 短参冲突、长参对齐 ssh、默认值行为 |
| 接口粒度 | 单一职责 vs 统一接口、Mock 测试友好度 |
| 存储方案 | 数据迁移、查询能力、并发安全 |
| 安全策略 | 加密算法、密钥管理、权限控制 |
| 错误处理 | 重试策略、边界条件、用户提示 |

---

## 3. 架构决策流程

### 3.1 ADR 触发判断

```
新决策 → 是否影响 ≥3 个模块？ → 是 → 必须创建 ADR
       → 是否引入新技术栈？  → 是 → 必须创建 ADR
       → 是否有安全相关？     → 是 → 必须创建 ADR
       → 否则 → 可选创建
```

### 3.2 ADR 模板

```markdown
# ADR-XXX: 标题

## Status
accepted/proposed/rejected

## Date
YYYY-MM-DD

## Context
问题背景、决策驱动力、可选方案

## Decision
最终方案及理由

## Consequences
正面/负面影响、影响范围

## Related
关联 ADR、文档、Issue
```

### 3.3 ADR 编号管理

- 编号格式：`XXX`（3 位，从 001 连续递增）
- 创建前检查 `docs/adrs.md` 列表，确保无冲突
- 发现冲突时通过内容比对确认真实编号，修改文件名

---

## 4. 文档对齐清单

### 4.1 核心计划文档（第一优先级）

| 文档 | 更新触发 |
|------|----------|
| `AGENTS.md` | Phase 状态变化 |
| `docs/TODO.md` | 任务增减、验收标准 |
| `docs/plan.md` | 技术选型、文件数 |

### 4.2 模块文档（第二优先级）

| 文档 | 更新触发 |
|------|----------|
| `docs/modules/<name>.md` | 模块设计完成/变更 |
| `docs/storage.md` | 存储结构变化 |
| `docs/architecture.md` | 架构图/数据流变化 |

### 4.3 索引文档（第三优先级）

| 文档 | 更新触发 |
|------|----------|
| `docs/adrs.md` | ADR 新增/状态变更 |
| `docs/requirements/` | 需求状态变更 |

### 4.4 验证检查

设计完成后执行：

```bash
# 1. Phase 编号检查
rg 'Phase [0-9]+' docs/ --type md

# 2. ADR 引用检查
rg 'ADR-[0-9]+' docs/ --type md

# 3. 文件路径检查
rg 'asshv2/[\w/.-]+' docs/ --type md
```

---

## 5. 代码交接规范

### 5.1 Build Agent 交接清单

| 交接项 | 内容 |
|--------|------|
| 接口签名 | port 层接口定义 |
| 文件清单 | 新建/修改文件列表 |
| 验收标准 | F1~F{N} 验收场景 |
| 测试要求 | Mock 模式、覆盖率 |
| 技术约束 | 依赖、架构要求 |

### 5.2 验收检查

```
实现完成
  ↓
go build ./...  (编译通过)
  ↓
go test ./... -count=1  (测试通过)
  ↓
逐项验证 F1~F{N}  (验收标准)
  ↓
全部通过 → 提交
```

---

## 6. 分支管理规范

### 6.1 分支命名

```
dev/v2.0.0-phase-N     # Phase 开发分支
  ├── feat/<feature>   # 特性分支
  └── bugfix/<issue>  # Bug 修复分支
```

### 6.2 合并流程

```bash
# 1. 确保 Phase 分支最新
git checkout dev/v2.0.0-phase-N
git pull origin dev/v2.0.0-phase-N

# 2. 合并到 master
git checkout master
git merge --no-ff dev/v2.0.0-phase-N

# 3. 打 tag
git tag -a v2.0.0-phase-N -m "Phase N complete"

# 4. 推送
git push origin master --tags
```

---

## 7. 工具使用指南

### 7.1 工具选择矩阵

| 任务 | 推荐工具 | 说明 |
|------|----------|------|
| 需求澄清 | `question` | 多轮逐项确认 |
| 文档创建 | `write` | 新文件 |
| 文档修改 | `edit` | 已有文件 |
| 代码实现 | `task (build agent)` | 编码阶段 |
| 版本管理 | `bash (git)` | 提交/合并/tag |
| 复杂 Git 操作 | `skill → git-manager` | 多分支合并/Changelog |
| ADR 管理 | `skill → adrs-manager` | ADR 数据库 |
| 任务追踪 | `todowrite` | 多步骤任务 |

### 7.2 question 工具技巧

- 使用 `question` 工具进行需求咨询
- 每轮聚焦 1-3 个问题
- 提供 A/B/C 选项并说明优缺点
- 用户用短句回复时表示确认

---

## 8. 常见决策模板

### 8.1 CLI 参数设计决策

| 考虑项 | 检查点 |
|--------|--------|
| 短参冲突 | 检查已有短参占用表 |
| ssh 对齐 | 参考 Linux ssh 参数 |
| 默认值 | 明确无参数时的行为 |
| 组合支持 | 是否需要 `-qnt` 组合支持 |

### 8.2 接口设计决策

| 考虑项 | 检查点 |
|--------|--------|
| 粒度 | 单一职责 vs 统一接口 |
| Mock | 是否容易 mock 测试 |
| 扩展 | 是否有扩展需求 |
| 复用 | 是否可复用到其他场景 |

### 8.3 存储方案决策

| 考虑项 | 检查点 |
|--------|--------|
| 查询能力 | 是否需要 SQL 查询 |
| 并发安全 | 多进程是否安全 |
| 迁移成本 | 数据如何迁移 |
| 依赖引入 | 是否引入新依赖 |

---

## 9. 关键约束参考

### 9.1 项目技术栈

- Go 1.22.0+
- 纯 Go 无 CGO（`modernc.org/sqlite`）
- 依赖注入（构造器注入）
- 三层架构（port/service/infra）

### 9.2 Phase 依赖关系

```
Phase 0 (骨架) → Phase 1 (基础设施) → Phase 2 (持久化)
    → Phase 3 (SSH) → Phase 4 (CLI) ← MVP
                        ↓
                  Phase 5 (SFTP) ← 当前
                        ↓
                  Phase 6 (密钥管理)
                        ↓
                  Phase 7 (代理/隧道)
                        ↓
                  Phase 8 (云同步)
                        ↓
                  Phase 9-12 ...
```

---

## 10. 快速参考

### 10.1 命令速查

| 操作 | 命令 |
|------|------|
| 新 Phase 分支 | `git checkout -b dev/v2.0.0-phase-N` |
| 创建 ADR | `skill → adrs-manager` |
| 提交代码 | `git add -A && git commit -m "type: description"` |
| 合并 Phase | `git merge --no-ff dev/v2.0.0-phase-N` |
| 打 tag | `git tag -a v2.0.0-phase-N -m "Phase N complete"` |

### 10.2 文档位置

| 文档 | 路径 |
|------|------|
| 项目状态 | `AGENTS.md` |
| 任务清单 | `docs/TODO.md` |
| 技术计划 | `docs/plan.md` |
| ADR 索引 | `docs/adrs.md` |
| Bug 记录 | `docs/bugs.md` |
| 工作流文档 | `docs/workflows/` |

---

> 本手册基于 Phase 2~5 实际经验总结，可作为 PM Agent 日常工作的快速参考。