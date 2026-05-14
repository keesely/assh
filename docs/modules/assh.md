# Server Data Module

**文件**: `asshv2/asshc/domain/server.go` + `asshv2/asshc/infra/store/{db,key,server}.go`

## 功能

管理服务器列表的核心数据结构：

- **数据结构**：`map[string]map[string]*Server`（群组 -> 服务器名 -> 服务器信息）
- **命名格式**：`group.server_name`（群组可省略，点号解析）
- **存储引擎**：SQLite 数据库（modernc.org/sqlite，纯 Go 无 CGO）
- **密码加密**：AES-256-GCM 模式，自动密钥管理
- **认证方式**：`Auth{Password, KeyFile}` 子结构体
- **密钥管理**：每个服务器可独立关联 SSH 私钥文件

## 表结构

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
    created_at        TEXT DEFAULT (datetime('now')),
    updated_at        TEXT DEFAULT (datetime('now')),
    UNIQUE(group_name, name)
);

CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);
```

> `config` 表存储 AES-256 静态密钥（首次运行时自动生成，base64 编码存储）。
> 密钥随数据库一起被云同步（Phase 8），实现多机器共享。