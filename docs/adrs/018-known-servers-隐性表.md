# ADR-018: known-servers 隐性表 — 未具名服务器直连记录

## Status
accepted

## Date
2026-05-10

## Context

Phase 6 (密钥管理) 引入 `assh keygen` 命令，其中直连模式（`assh keygen -H host -u root -P pass`）不关联任何已保存的服务器配置，生成的密钥需要有个地方记录其与目标服务器的关联关系。

同时，`assh` 长期以来支持直连登录（`assh root@host` / `assh login -H host -u root`），但每次直连都是"一次性"的，没有记录连接历史。这意味着：

1. 直连 keygen 生成的密钥没有地方关联到对应的匿名服务器
2. 后续再次直连同一服务器无法复用之前关联的密钥
3. 无法追踪匿名服务器的连接频率和使用模式

需要一个轻量级的记录机制，对用户几乎透明（"隐性"），不干扰正常的服务器管理流程。

## Decision

### 方案：`known_servers` 隐性表

在现有 SQLite 数据库中新增 `known_servers` 表，自动记录所有未通过已保存配置的直连操作。

### 表结构设计

```sql
CREATE TABLE IF NOT EXISTS known_servers (
    id                TEXT PRIMARY KEY,     -- SHA256(user@host:port#auth_fingerprint)
    host              TEXT NOT NULL,        -- 主机地址
    port              INTEGER NOT NULL DEFAULT 22,
    user_name         TEXT NOT NULL DEFAULT 'root',
    auth_fingerprint  TEXT NOT NULL DEFAULT '',  -- SHA256(password|key_path) 用于身份匹配
    key_backup_path   TEXT DEFAULT '',      -- data/keys/ 中关联的密钥备份路径
    last_connected_at TEXT,                 -- 最后一次连接时间
    connect_count     INTEGER DEFAULT 1,   -- 累计连接次数
    created_at        TEXT DEFAULT (datetime('now')),
    updated_at        TEXT DEFAULT (datetime('now'))
);
```

### ID 生成规则

```
input = fmt.Sprintf("%s@%s:%d#%s", user, host, port, authFingerprint)
id    = SHA256(input)
```

其中 `authFingerprint` 计算规则：

| 认证方式 | fingerprint 计算 | 示例 |
|----------|-----------------|------|
| 密码 | `SHA256(password)` | 同一密码始终得到相同 ID |
| 密钥文件 | `SHA256(绝对路径)` | 同一路径始终得到相同 ID |
| 两者都有 | `SHA256(password)` + `:SHA256(keypath)` 组合 | 完整认证指纹 |
| 两者都无（agent） | 空字符串 `""` | 仅按 `user@host:port` 聚合 |

### 写入时机

所有直连接入点均触发 `known_servers` 的 upsert：

| 场景 | 触发命令 | 写入内容 |
|------|----------|----------|
| 直连 keygen | `assh keygen -H host -u root -P pass` | 生成记录 + 更新 `key_backup_path` |
| 直连登录 | `assh root@host` | upsert 记录，递增 `connect_count` |
| 直连 run | `assh run -H host -u root "cmd"` | upsert 记录 |
| 直连 login | `assh login -H host -u root` | upsert 记录 |
| 直连 bc | `assh bc -H host -u root "cmd"` | upsert 记录 |

### 读取时机

| 场景 | 读取内容 |
|------|----------|
| 直连登录时 | 查找 `key_backup_path` → 尝试使用备份密钥 |
| keygen 直连前 | 检查是否已有记录 → 避免重复部署 |

### 不支持的操作

- 不提供 CLI 命令查询/修改 `known_servers`（保持隐性）
- 不自动清理（数据量很小，一条直连记录约 200 字节）
- 不纳入版本管理 / changelog（区别于 `servers` 表）

## Consequences

### 正面影响

- 直连 keygen 生成的密钥有地方持久化关联
- 后续直连可复用已部署密钥，无需每次都指定 `-k`
- 连接历史记录可用于后续功能（如最近连接列表、连接频率统计）
- 表结构简单，对用户完全透明
- 基于 hash 的 ID 天然去重，同参数连接自动聚合

### 负面影响

- 数据库新增一个表，迁移代码需更新（`migrate()` 增加建表语句）
- 直连操作的延迟略微增加（一次 upsert 写入）
- 密码的 SHA256 存入数据库（不是明文密码本身，但需要注意 auth_fingerprint 不应暴露原始密码）
- 所有直连入口需统一接入写入逻辑（需修改 `connect.go` / `app.go` 的直连路径）

### 影响范围

| 范围 | 文件 |
|------|------|
| 数据库迁移 | `asshc/infra/store/db.go` — `migrate()` 新增 `known_servers` 建表 |
| 存储接口 | `asshc/port/repository.go` — 新增 `KnownServerRecorder` 接口或方法 |
| 存储实现 | `asshc/infra/store/known.go` — 新增文件，实现 upsert/lookup |
| 直连接入点 | `cmd/connect.go` + `cmd/app.go` — 直连成功后调用 record |
| 密钥管理 | `asshc/infra/keymgr/deploy.go` — keygen 直连后更新 `key_backup_path` |
| SSH 连接 | `asshc/infra/ssh/client.go` — 连接前检查 `key_backup_path` |
| DI 注入 | `main.go` + `cmd/fs/main.go` |

### 安全性说明

`auth_fingerprint` 存储的是密码的 SHA256 哈希，而非明文密码。这提供了：
- 不可逆性：无法从 hash 还原密码
- 匹配能力：同一密码始终产生相同 hash
- 有限攻击面：即使数据库泄露，仅暴露密码 hash（仍需暴力破解）

## Related
- ADR-008: SQLite 数据库替代加密 JSON 文件
- ADR-009: AES-256-GCM 静态密钥加密
- Phase 6: 密钥管理模块
- docs/modules/key-management.md
