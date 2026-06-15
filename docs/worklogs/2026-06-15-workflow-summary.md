# 工作流总结：ASSH v2.2.0 阶段验收测试

## 概述

本文档总结从 X1/X2 实验功能开发到阶段验收测试的全工作流，提炼可复用的经验和抽象模式。

## 工作流阶段

### 第一阶段：需求确认 (Requirements Elicitation)

**输入**: 用户提出需求 → **输出**: 需求文档

**流程**:
1. PM Agent 创建问卷 → 多轮问答消除歧义
2. 每轮确认后更新需求文档
3. 最终输出：`docs/requirements/RQ-003-*.md`

**经验**:
- 每次问卷聚焦 3-5 个决策点
- 优先确定边界条件（如"错误时如何表现"）
- 记录被否决的选项（如"默认记录历史"被否决）

### 第二阶段：分支策略 (Branch Management)

**输入**: 需求文档 → **输出**: 开发分支

**模式**:
```
master
  └── dev/v2.2.0-experiment     (实验版本)
        └── dev/v2.2.0-phase-x1  (X1 开发)
        └── dev/v2.2.0-phase-x2  (X2 开发)
```

**经验**:
- 实验功能使用独立分支，不与主版本线冲突
- 每次切换分支前必须编译验证
- 合并冲突采用 `git checkout --theirs/--ours` 策略

### 第三阶段：接口先行 (Interface-First Development)

**输入**: 需求文档 → **输出**: 接口定义 + 实体定义

**顺序**: `domain/entities` → `port/interfaces` → `service/impl` → `infra/impl`

**经验**:
- `domain/` 只包含纯类型和错误定义，零依赖
- `port/` 只依赖 `domain/`
- `service/` 只依赖 `port/` 接口（依赖倒置）
- 发现 `domain/` 中的通用类型应提取为共享错误定义

### 第四阶段：安全审查 (Security Review)

**输入**: 代码变更 → **输出**: 安全修复

**工具链**:
```
govulncheck → 依赖漏洞扫描
rg/rg       → 机密扫描
staticcheck → 代码质量
go test -race → 数据竞争检测
```

**发现的典型问题**:
1. 命令注入 (字符串拼接 SSH 命令)
2. HostKey 校验缺失 (MITM 攻击)
3. SQL 注入 (表名拼接)
4. 不安全 fallback (crypto/rand 失败时)

### 第五阶段：合并冲突处理 (Merge Conflict Resolution)

**问题模式**:
1. Go 结构体字段冲突（`atomic.Value` vs `[]byte`）
2. 函数签名变化（`collectLocalFiles` 2→3 参数）
3. 新增文件未同步（git cherry-pick）
4. go.mod 版本冲突（手动选择较新版本）
5. 全局变量/错误定义冲突（包级别命名冲突）

**解决策略**:
1. `git merge master` 优先
2. 编译失败时逐一检查：`go build ./... 2>&1`
3. 缺失文件用 `git checkout <branch> -- <file>` 恢复
4. go.mod 冲突选择较新安全版本
5. 命名冲突通过重命名解决（加前缀/后缀）

### 第六阶段：验收测试 (Acceptance Testing)

**测试金字塔**:
```
A1: go build ./...        → 编译验证
A2: go vet ./...          → 静态检查
A3: go test ./...         → 单元测试
A4: go test -race ./...   → 数据竞争检测
```

**验收清单结构**:
- 基础验证 (A1-A4)
- 安全修复验证 (B1-B7)
- 功能模块验证 (C1-C8, D1-D8)
- 合并修复验证 (E1-E2)

## 可复用经验

### 1. Domain 错误冲突预防

当多个独立开发的功能（如 X1 和 X2）同时定义 `domain` 级别的错误时：

```go
// 推荐：使用包级前缀命名
var (
    ErrVulnNotFound     = &VulnError{...}
    ErrVirtualTimeout   = &VirtualCommandError{...}  // 有前缀
)
// 不推荐：使用通用命名
var (
    ErrTimeout          = &VulnError{...}   // 冲突风险
    ErrTimeout          = &VirtualCommandError{...}  // 冲突
)
```

### 2. git worktree 分支管理

```bash
# 创建 worktree
git worktree add -b <branch> /path/to/worktree master

# 合并 master
cd /path/to/worktree && git merge master

# cherry-pick 跨 worktree 提交
git cherry-pick <commit-hash>
```

### 3. 安全修复优先级

```
P0 (立即修复)  → 命令注入 / HostKey 缺失 / 依赖漏洞
P1 (当天修复)  → SQL 注入 / 不安全 fallback / 禁用不安全模式
P2 (排期修复)  → 加密密钥同库存储 / 死代码清理
P3 (低优先级)  → 代码重复合并 / 风格问题
```

### 4. 文档对齐检查

每次阶段完成后的检查清单：
- [ ] TODO.md 更新版本和状态
- [ ] AGENTS.md 更新版本/分支/阶段
- [ ] CHANGELOG.md 添加版本记录
- [ ] 需求文档标记完成
- [ ] 工作日志记录分析过程
- [ ] 验收报告生成

## 工作流图

```
需求确认 → 分支创建 → 接口定义 → 代码实现
                                         ↓
                                   安全审查
                                         ↓
                                   阶段验收
                                    /     \
                              编译通过  测试通过
                                    \     /
                                  文档对齐
                                      ↓
                                  合并发布
```

---

*文档版本: 1.0*
*适用项目: ASSH v2.x 开发流程*
*可复用: 适用于任意 Go 语言 CLI 项目开发*