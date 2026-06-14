# set/rollback/changelog 实现

> 日期: 2026-05-09
> 分支: dev/v2.0.0-phase-4  
> 类型: 功能增强 + 重构

---

## 变更概述

三个相互关联的变更：

1. **edit → set**（重命名 + upsert 语义）: 将原来的 `edit` 改为 `set`，支持自动创建/更新
2. **版本管理 + changelog**：每个服务器配置修改自动记录版本号和快照
3. **rollback 命令**：根据版本号回滚服务器配置

## 详细设计

### 1. set (upsert)

- 服务器不存在 → 自动创建（需 `-H/--host`，port 1-65535 校验）
- 服务器已存在 → 部分更新（仅修改显式指定的字段）
- 行为统一：`server set <name> [flags]` 替代原先的 `edit`
- 参数验证：port 范围检查、创建时 host 必填

### 2. 版本管理

**数据库变更：**
- `servers` 表新增 `version INTEGER NOT NULL DEFAULT 1`
- 新增 `server_changelog` 表：

```sql
CREATE TABLE server_changelog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    group_name TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL,
    change_type TEXT NOT NULL,  -- 'create', 'update', 'rollback'
    snapshot TEXT NOT NULL,     -- JSON: 完整服务器配置快照
    created_at TEXT DEFAULT (datetime('now'))
);
```

**Set 自动流程：**
1. 检查服务器是否存在 → 确定 change_type（create/update）
2. 版本递增（新服务器 v1，已有 v+1）
3. 保存服务器数据（`INSERT OR REPLACE`，含 version）
4. 序列化完整 Server 为 JSON 快照 → 写入 changelog
5. 事务包裹，保证原子性

**Rollback 流程：**
1. 查找 `server_changelog` 中目标版本的 snapshot
2. 反序列化恢复 Server 对象
3. 版本递增（current + 1）
4. 重新加密密码写入 servers 表
5. 写入 changelog 条目（change_type=rollback）
6. 事务包裹

### 3. rollback 命令

```bash
assh server rollback <name> <version>   # 回滚到指定版本
assh server rollback <name> --list      # 显示变更历史
```

### 领域模型变更

- `domain.Server` 新增 `Version int` 字段
- 新增 `domain.ChangelogEntry` 类型
- 新增错误：`ErrVersionNotFound`, `ErrInvalidVersion`

## 文件变更

| 文件 | 变更 |
|------|------|
| `asshc/domain/server.go` | +`Version` 字段 |
| `asshc/domain/changelog.go` | 新建: `ChangelogEntry` 类型 |
| `asshc/domain/errors.go` | +`ErrVersionNotFound`, `ErrInvalidVersion` |
| `asshc/port/repository.go` | +`GetChangelog`, `RollbackTo` 接口方法 |
| `asshc/infra/store/db.go` | 迁移: version 列 + server_changelog 表 + 兼容性检查 |
| `asshc/infra/store/server.go` | 重写: Set 版本化+快照, 新增 GetChangelog/RollbackTo |
| `asshc/service/server.go` | +`SetServer`(upsert), `RollbackServer`, `GetServerChangelog` |
| `cmd/server.go` | edit→set 重命名+upsert+验证, 新增 rollback 子命令 |
| `asshc/service/server_test.go` | mockRepo 新增 GetChangelog/RollbackTo 方法 |

## 验证结果

```bash
go build ./...  # ✅
go vet ./...    # ✅  
go test ./... -count=1  # ✅ (7 包全部通过)
```

### 验收场景 (19 项)

| # | 场景 | 结果 |
|---|------|------|
| 1 | set 创建新服务器 | ✅ |
| 2 | info 显示版本 | ✅ |
| 3 | set 更新已有服务器（版本递增） | ✅ |
| 4 | set 添加 Options | ✅ |
| 5 | set 修改 port + 清除密码 | ✅ |
| 6 | rollback --list 显示 changelog | ✅ |
| 7 | rollback 回滚到 v2 | ✅ |
| 8 | rollback 后 --list 显示 rollback 条目 | ✅ |
| 9 | set 新建无 host 报错 | ✅ |
| 10 | set 非法端口报错 | ✅ |
| 11 | rollback 无效版本报错 | ✅ |
| 12 | set 无标志显示 hint | ✅ |
| 13 | ls 显示 Version 列 | ✅ |

## 设计决策

1. **快照存储密码明文**: server_changelog 存储完整 Server JSON 快照（含明文密码），与 servers 表同等级安全约束。后续可增加快照加密。
2. **Set 自动版本化**: 版本管理对上层透明，service/cmd 无需手动管理版本号。
3. **Rollback 创建新版本**: 回滚不覆盖历史，而是创建新的 rollback 类型条目，保证可审计性。
4. **StringSlice IsSet 兼容**: 沿用之前的 `len(c.StringSlice("option"))` 检测，避免 urfave/cli v1 的 `IsSet` 缺陷。
