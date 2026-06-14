# Phase 4 细化及体验优化工作报告

> 日期: 2026-05-10
> 分支: dev/v2.0.0-phase-4
> 基准: 94cd841 (info 格式重构前)

---

## 变更摘要

本轮是 Phase 4 (CLI 命令组装) 的细化与体验优化，聚焦 info/ls 输出格式、模糊匹配建议和工作流规范。

### 提交列表

| 提交 | 说明 |
|------|------|
| `3c440cc` | feat: refine assh info format — 居中标题+对齐加号、Auth Type、首字母大写标签 |
| `b14e5be` | feat: fuzzy server suggestion on info not-found & adaptive ls column widths |
| `630b7d2` | feat: default login action suggests similar server name on not-found |
| `7ffe18a` | docs: add auto-build check workflow rule |
| `07a3ef3` | docs: use Q shorthand for issue references in methodology doc |

### 变更文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `asshv2/cmd/server.go` | 修改 | info 格式重构 + ls 自适应列宽 + info 模糊建议 |
| `asshv2/cmd/app.go` | 修改 | 默认 Action 模糊建议 |
| `asshv2/asshc/service/server.go` | 修改 | 新增 SuggestServer / levenshtein |
| `AGENTS.md` | 修改 | 新增工作流规则章节 |
| `docs/pm-workflow-methodology.md` | 修改 | 新增零、自动编译检查章节 |

---

## 变更详情

### 1. `assh info` 输出格式重构

**目标**: 对齐 v1 的 info 样式，提升可读性

**修改内容** (`cmd/server.go: printServerDetail`):

1. **Header 加号对齐** — `+` 固定在行首行尾（第 1 列和第 40 列），标题内容居中，始终与上下分割线的两端对齐
2. **Label 命名规范** — 首字母大写 + 业务前缀：
   - `name` → `Server Name`, `host` → `Server Host`, ...
   - `password` → `Password`, `key file` → `Key File`
3. **Auth Type** — 新增类型区分行：
   - `Auth Type: password` / `key` / `password+key` / `none`
4. **Label 宽度** — 设为 11（最长 label "Server Name" 长度），实现零多余缩进

**输出示例**:
```
----------------------------------------
+          demo-srv (ver. 1)           +
========================================
Server Name: demo-srv
Server Host: 10.0.0.1
Server Port: 2222
Server User: admin
  Auth Type: password
   Password: s3cret
   Key File: /Users/kee/.ssh/id_rsa
     Remark: test vm
```

---

### 2. `assh info` / `assh <target>` 模糊匹配建议

**目标**: 服务器不存在时自动给出相近名称提示

**技术方案** (`asshc/service/server.go`: `SuggestServer`):

1. **前缀匹配**（优先）: input 是某服务器名（含分组前缀）的前缀 → 直接建议
   - 多个前缀匹配时选择最短名称（最接近）
2. **Levenshtein 编辑距离**（回退）: 同时匹配全名 `group.name` 和裸名 `name`，取较小距离
   - 阈值 = `max(2, len(input)*0.6)`
3. 判定通过则返回建议的全名

**入口集成**:
- `serverInfoAction`: 截获 `domain.ErrNotFound` → 调用 `SuggestServer` → 输出 `"case.bbp" not found, did you mean [case.bbp2]?`
- `defaultAction`: 同上逻辑，输出 `server not found, did you mean [case.bbp2]?`

**测试验证**:

| 输入 | 匹配方式 | 结果 |
|------|----------|------|
| `case.bbp` | Levenshtein (dist=1) | → `case.bbp2` |
| `bbp2` | 裸名 Levenshtein (dist=0) | → `case.bbp2` |
| `test` | 前缀匹配 (test-*) | → `test-box` (最短) |
| `totally.wrong` | 无匹配 | → `server not found` |

---

### 3. `assh ls` 自适应列宽

**目标**: 表格紧凑美观，列宽根据内容动态调整

**修改内容** (`cmd/server.go: serverListAction`):

1. 先收集所有数据到 `rows []row` 切片
2. 计算 Group/Name/Host 三列的最大宽度（>= 表头宽度）
3. 用动态宽度格式化输出
4. Group 列右对齐，Name/Host 左对齐

**输出对比**:
```
# 之前（固定宽度）
Group            Name                 Host                         Ver  Auth           Remark
# 之后（自适应宽度）
Group Name        Host                  Ver  Auth           Remark
```

---

### 4. 自动编译检查工作流

**目标**: 确保每次代码变更后编译通过

**修改内容**:
- `AGENTS.md`: 新增"工作流规则"章节
- `docs/pm-workflow-methodology.md`: 新增"零、自动编译检查"章节

**规则**: 每次修改代码文件后，必须立即执行 `cd asshv2 && go build ./...` 确认编译通过，否则不得进入下一步。

---

## 被否决的方案

### 模糊匹配阈值调整

**方案**: 单一 Levenshtein 阈值 = `max(3, len(input)/2)`

**问题**: 短输入如 `test` (4 字符) 距离 `test-server` (11 字符) 为 6，远超阈值 2 → 不匹配

**最终方案**: 加入前缀匹配作为优先策略，Levenshtein 作为回退。前缀匹配返回最短的名称（`test-box` 而非 `dev.test-server`），更节省用户输入。

---

## 文档对齐清单

| 文档 | 操作 | 说明 |
|------|------|------|
| `AGENTS.md` | ✅ 已更新 | 新增工作流规则章节 |
| `docs/pm-workflow-methodology.md` | ✅ 已更新 | 新增自动编译检查章节 |
| `docs/TODO.md` | ⬜ 本轮更新 | 追加 Phase 4 细化任务 |
| `docs/plan.md` | ⬜ 本轮检查 | 功能列表无需调整 |
| `docs/adrs.md` | ⬜ 本轮检查 | 无需新建 ADR（UX 增强非架构变更） |
| `docs/bugs.md` | ⬜ 本轮检查 | 无新 bug |
| `docs/worklogs/` | ✅ 本文件 | 本轮工作报告 |

---

## 验证结果

```bash
go build ./...          ✅ 编译通过
go vet ./...            ✅ 静态检查通过
go test ./... -count=1  ✅ 7 包全部通过
```
