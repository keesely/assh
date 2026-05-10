# ADR-008: 废弃加密 JSON 文件，改用 SQLite 数据库

## Status

accepted

## Date

2026-05-09

## Context

v1（以及原始 Phase 2 计划）使用加密 JSON 文件存储服务器数据：

- **文件格式**：`~/.assh/data/servers.db`
- **加密方案**：AES-CBC + MD5 双重校验
- **密码管理**：需要用户输入主密码才能解密数据
- **查询能力**：每次操作需要全量加载/写入文件，不支持条件查询

该方案存在以下问题：

1. **用户干扰大**：每次使用 assh 都需要输入密码，影响流畅体验
2. **查询低效**：不支持 SQL 查询，搜索/分组过滤需全量遍历
3. **并发不安全**：多进程同时操作存在竞态
4. **格式脆弱**：文件损坏则数据全部丢失

## Decision

改用 SQLite 数据库（`modernc.org/sqlite`）替代加密 JSON 文件：

- **驱动选择**：`modernc.org/sqlite`（纯 Go 实现，无 CGO 依赖，支持跨平台编译）
- **数据库路径**：`~/.assh/v2/asshv2.db`
- **表结构**：`servers` 表（服务器数据）+ `config` 表（应用配置）
- **迁移策略**：不提供 v1 到 v2 的数据迁移（独立版本），v2 全新开始

### 表结构设计

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

## Consequences

1. **优点**：
   - 无需主密码即可操作服务器（密码仅用于同步）
   - 支持 SQL 查询（搜索、分组、排序）
   - 纯 Go 无 CGO，跨平台编译无障碍
   - SQLite 本身支持事务和并发
   - 数据文件可作为整体被云同步（Phase 8）

2. **缺点**：
   - 新增 `modernc.org/sqlite` 依赖（~5MB 二进制体积增加）
   - 数据库中密码列虽加密但数据库文件本身需文件系统保护
   - 不兼容 v1 数据格式（需手动迁移）

3. **ADR 影响**：
   - 废弃原 Phase 2.4（加密文件存储）的实现
   - 更新 ADR-001 中的存储实现描述

## Related

- Supersedes: 原 Phase 2 加密 JSON 文件方案
- Related ADRs: ADR-001 (三层架构), ADR-004 (crypto 模块)
