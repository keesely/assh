# Storage

- 存储根目录：`~/.assh/v2`
- 日志文件：`/tmp/assh.log`（默认，未配置时）
- 数据库文件：`~/.assh/v2/asshv2.db`（SQLite，含 servers 表 + config 表）
- 配置文件：`~/.assh/v2/assh.yml`（RSA 加密，Phase 1 已实现）
- 密钥文件：`~/.assh/v2/.rsa`, `~/.assh/v2/.rsa.pub`（RSA 密钥对）
- 密码文件：`~/.assh/v2/.account`（Phase 7 云同步时实现）

## 数据库结构

数据库 `asshv2.db` 包含三张表：

| 表 | 用途 | 说明 |
|----|------|------|
| `servers` | 服务器列表 | 含 AES-GCM 加密的密码列，`version` 列跟踪配置版本 |
| `config` | 应用配置 | 存储 AES 静态密钥等 |
| `server_changelog` | 服务器配置变更记录 | 每条记录一个版本快照（JSON），支持回滚 |

> 静态密钥（AES-256）首次运行时自动生成，base64 编码存入 `config` 表。
> 密钥随数据库一起同步（Phase 7 云同步），支持多机器共享。

### servers 表

```sql
CREATE TABLE servers (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT NOT NULL,
    group_name        TEXT NOT NULL DEFAULT '',
    host              TEXT NOT NULL,
    port              INTEGER NOT NULL DEFAULT 22,
    user_name         TEXT NOT NULL DEFAULT 'root',
    password_encrypted BLOB,
    key_file          TEXT DEFAULT '',
    remark            TEXT DEFAULT '',
    options           TEXT DEFAULT '{}',
    version           INTEGER NOT NULL DEFAULT 1,  -- 配置版本号，每次修改递增
    created_at        TEXT DEFAULT (datetime('now')),
    updated_at        TEXT DEFAULT (datetime('now')),
    UNIQUE(group_name, name)
);
```

### server_changelog 表

```sql
CREATE TABLE server_changelog (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    group_name  TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL,     -- 版本号（与 servers.version 对应）
    change_type TEXT NOT NULL,         -- 'create', 'update', 'rollback'
    snapshot    TEXT NOT NULL,         -- JSON: 完整 Server 快照（含明文密码）
    created_at  TEXT DEFAULT (datetime('now'))
);

CREATE INDEX idx_changelog_lookup ON server_changelog(server_name, group_name);
CREATE INDEX idx_changelog_version ON server_changelog(server_name, group_name, version);
```

> changelog 中的 snapshot 包含密码明文——与 servers 表同等级安全约束（本地 SQLite 文件）。
> 每次 `Set` 操作自动记录快照，`RollbackTo` 从快照恢复并记录 rollback 类型。