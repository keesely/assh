# Phase 6 密钥管理 — 初始化工作报告

**日期**: 2026-05-11
**阶段**: Phase 6 (Key Management)
**状态**: 初始化完成，开发就绪

---

## 1. 分析过程

### 项目状态快照

Phase 5 (SFTP 文件传输) 已于 2026-05-11 完成验收（F1-F28 全部通过），项目进入 Phase 6 开发阶段。

### 仓库结构发现

通过分析 git worktree 结构，确认了项目采用**嵌套仓库**架构：

```
assh-builder/          ← 项目管理仓库（文档、配置、ADR）
  ├── assh/            ← assh 代码仓库的 .git 主目录
  ├── asshv2/          ← assh 仓库的 worktree（.git → assh/.git/worktrees/asshv2）
  ├── docs/            ← 项目文档（TODO、plan、ADR、worklogs）
  └── AGENTS.md        ← 项目级 Agent 配置
```

- `assh-builder/` 跟踪文档和配置变更
- `asshv2/` 跟踪代码变更（Go 项目）
- 两者是独立的 git 仓库，需要分别操作

### 依赖分析

| 依赖项 | 状态 | 所在分支 |
|--------|------|----------|
| Phase 1 (crypto) | ✅ | master |
| Phase 3 (SSH) | ✅ | master |
| Phase 4 (CLI) | ✅ | dev/v2.0.0-phase-4 (未合入 master) |
| Phase 5 (SFTP) | ✅ | dev/v2.0.0-phase-5 |
| **Phase 6 依赖** | Phase 4 (CLI) | **不依赖 Phase 5** |

### 关键发现

1. **master 分支只到 Phase 3**：Phase 4 和 Phase 5 尚未合入 master
2. **Phase 6 不依赖 Phase 5**：可以从 phase-4 或 phase-5 创建分支
3. **assh-fs 二进制被 git 跟踪**：这是一个需要修复的问题

---

## 2. 设计方案与决策

### 分支创建策略

| 方案 | 说明 | 优缺点 |
|------|------|---------|
| A: 从 master 创建 | 规范流程 | ❌ 缺少 Phase 4 CLI 依赖 |
| B: 从 phase-4 创建 | 最干净（仅 CLI） | ✅ 无多余代码 / ❌ 缺少 BUG-010 等修复 |
| C: 从 phase-5 创建 | 包含所有最新 | ✅ 包含 BUG-010 认证回退等修复 / ❌ 包含不需要的 SFTP 代码 |

**决策**: 用户选择方案 C — 从 `dev/v2.0.0-phase-5` 创建。理由：包含 BUG-010 等关键修复，且 SFTP 代码不会干扰 Phase 6 开发。

### 优先级决策

用户指定优先级为**先 known-servers 表**（ADR-018），因为 known_servers 是直连记录和密钥关联的基础设施。

---

## 3. 执行操作

### Git 操作

| 操作 | 仓库 | 命令 | 结果 |
|------|------|------|------|
| 丢弃 assh-fs 脏状态 | assh | `git checkout -- assh-fs` | ✅ |
| 创建 phase-6 分支 | assh (asshv2/) | `git checkout -b dev/v2.0.0-phase-6` | ✅ |
| 创建 phase-6 分支 | assh-builder | `git checkout -b dev/v2.0.0-phase-6` | ✅ |
| untrack assh-fs | assh | `git rm --cached assh-fs` | ✅ |
| 提交文档变更 | assh-builder | `git commit -m "docs: Phase 6 密钥管理初始化..."` | ✅ `4e08485` |
| 提交 .gitignore 修复 | assh | `git commit -m "chore: add assh-fs to .gitignore..."` | ✅ `b850608` |

### 文档变更

| 文件 | 变更内容 |
|------|----------|
| `AGENTS.md` | Phase 6 状态: "⏳ 规划中" → "🔄 开发中" |
| `docs/TODO.md` | Phase 6 状态标记 + 任务分解重构（K.x → P6.x 体系 + 优先级标注） |
| `docs/modules/key-management.md` | 状态: "⏳ 规划中" → "🔄 开发中 (2026-05-11 起)" |
| `asshv2/.gitignore` | 添加 `assh-fs` 防止二进制被跟踪 |
| `assh-fs` | 从 git 跟踪中移除（git rm --cached） |

---

## 4. 文档对齐清单

### 核心计划文档
- [x] `docs/TODO.md` — Phase 6 任务分解 + 优先级标注已同步
- [x] `docs/plan.md` — 无需修改（Phase 6 已在计划中）
- [x] `AGENTS.md` — Phase 6 状态已同步

### 模块文档
- [x] `docs/modules/key-management.md` — 状态已同步

### 架构决策
- [ ] 无新增 ADR（Phase 6 初始化阶段未涉及架构变更）
- [x] `docs/adrs.md` — 无需更新（ADR-018/020 已存在）

### 工作报告
- [x] `docs/worklogs/2026-05-11-Phase6-Initialization.md` — 本文件

---

## 5. Git 日志记录

### assh-builder 仓库

```
4e08485 docs: Phase 6 密钥管理初始化 — 状态同步、任务分解、优先级标注(先known-servers)
0e9479f docs: Phase 5 doc alignment - interface sync, Go version bump, ADR workspace merge
```

### assh 仓库（asshv2/）

```
b850608 chore: add assh-fs to .gitignore and untrack binary from git
b569a77 fix: BUG-010 SSH key auth fallback to password + agent empty signers fix
```

---

## 6. Phase 6 开发路线图

按优先级排序（先 known-servers → 接口 → infra → service → CLI → 集成）：

```
▶▶ 第一步: P6.5 known-servers 表 (ADR-018)
    ├── P6.5-a 接口扩展 (asshc/port/repository.go)
    ├── P6.5-b 数据库迁移 (asshc/infra/store/db.go)
    ├── P6.5-c 存储实现 (asshc/infra/store/known.go)
    └── P6.5-d 直连接入点 (cmd/connect.go + cmd/app.go)

▶ 第二步: P6.1 KeyManager 接口 (asshc/port/keymanager.go)
▶ 第三步: P6.2 infra 实现 (asshc/infra/keymgr/)
▶ 第四步: P6.3 KeyService (asshc/service/key.go)
▶ 第五步: P6.4 keygen 命令 + P6.6 SSH修改 + P6.7 DI注入
```

验收标准: F1-F16（详见 key-management.md）

---

## 7. 可复用经验

1. **嵌套仓库操作**：assh-builder（文档）和 assh（代码）是独立仓库，文档和代码变更需分别提交
2. **分支创建来源**：当 master 未包含所需依赖时，从包含依赖的开发分支创建新分支是合理的
3. **二进制文件管理**：编译产物应在 .gitignore 中排除，已跟踪的需 `git rm --cached` 配合 .gitignore
4. **优先级驱动**：用户指定"先 known-servers"改变了默认的接口→infra→service→CLI 开发顺序，基础设施优先可能更稳定