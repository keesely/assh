# Worklog: Phase 2 数据持久化层方案设计与文档对齐

- **日期**: 2026-05-09
- **作者**: PM Agent
- **标签**: `phase-2`, `sqlite`, `architecture`, `adr`, `documentation`

---

## 1. 背景与目标

对 `assh v2.0.0` 项目当前阶段（Phase 2: 数据持久化层）进行全面分析，重新设计任务计划，并将所有相关文档与架构决策对齐。

### 核心目标

1. 审查 Phase 0-2 的完成状态和代码现状
2. 设计 Phase 2 的详细可执行任务计划
3. 根据用户反馈调整架构方向（SQLite + AES-GCM）
4. 同步对齐所有相关文档（需求、术语、特性、计划、ADR）

---

## 2. 当前状态分析

### 2.1 已完成阶段

| Phase | 内容 | 文件数 | 状态 |
|-------|------|--------|------|
| 0 | 项目骨架 — 目录结构、`build.sh`、`main.go`、`cmd/app.go` | 4 | ✅ |
| 1 | 基础设施 — `log/`、`config/`、`asshc/infra/crypto/`、`asshc/port/cryptor.go` | 8 | ✅ |

### 2.2 Phase 2 代码现状

| 目录 | 文件 | 状态 |
|------|------|------|
| `asshc/domain/` | `server.go`, `errors.go` | ❌ 空目录 |
| `asshc/port/` | `repository.go` | ❌ 仅 cryptor.go 存在 |
| `asshc/infra/store/` | `filestore.go` | ❌ 空目录 |
| `asshc/service/` | `server.go`, `auth.go` | ❌ 空目录 |
| `asshc/infra/{proxy,sftp,ssh,sync,tunnel}/` | - | ❌ 空目录 |

### 2.3 关键发现

- 旧版 v1 使用 AES-CBC 加密 JSON 文件 + MD5 签名 + 用户主密码，体验繁琐
- 现有 crypto infra 已支持 AES-GCM 等现代加密模式
- `config/` 模块提供完整的路径管理和全局变量
- `asshc/port/cryptor.go` 定义了加密接口可复用

---

## 3. Phase 2 任务计划设计

### 3.1 初始方案

基于 TODO.md 原计划，拆解为 14 个子任务，沿用 v1 的加密 JSON 文件方案：

| 层级 | 任务 | 文件 |
|------|------|------|
| 领域层 | P2.1-a 错误定义 | `domain/errors.go` |
| 领域层 | P2.1-b 实体定义 | `domain/server.go` |
| 接口层 | P2.2 Repository | `port/repository.go` |
| 基础设施 | P2.3-a 构造函数 | `infra/store/filestore.go` |
| 基础设施 | P2.3-b 加密引擎 | `infra/store/filestore.go` |
| 基础设施 | P2.3-c CRUD | `infra/store/filestore.go` |
| 服务层 | P2.4-a ServerService | `service/server.go` |
| 服务层 | P2.4-b AuthService | `service/auth.go` |

### 3.2 用户反馈与方向调整

用户否决了加密 JSON 文件方案，提出三个关键变更：

| 变更 | 原方案 | 新方案 |
|------|--------|--------|
| **存储引擎** | AES-CBC 加密 JSON 文件 | SQLite 数据库 |
| **密码策略** | 用户主密码每次操作验证 | AES-256-GCM 静态密钥自动加解密 |
| **password 用途** | 解密所有数据 | 仅用于 Phase 6 云同步 |

### 3.3 最终方案（16 个子任务）

```
P2.1-a domain/errors.go         哨兵错误（ErrNotFound, ErrExists 等）
P2.1-b domain/server.go         实体定义（Server, Auth, parseName, Validate）
  └── P2.2 port/repository.go   ServerRepository 接口
        └── P2.3-a infra/store/db.go      SQLite 连接 + 迁移
              ├── P2.3-b infra/store/key.go   AES-256 密钥管理 + 密码加解密
              └── P2.3-c infra/store/server.go SQLite CRUD
                    └── P2.4 service/server.go  ServerService 用例编排
```

### 3.4 关键设计决策

| 决策项 | 结论 |
|--------|------|
| SQLite 驱动 | `modernc.org/sqlite`（纯 Go，无 CGO） |
| 加密算法 | AES-256-GCM（认证加密，消除 CBC padding oracle 风险） |
| 密钥管理 | 首次运行自动生成 → 存入 SQLite `config` 表 → 随数据同步 |
| Auth 结构 | `Auth{Password string, KeyFile string}` 子结构体 |
| 名称解析 | 点号 `group.server` 自动解析 |
| Options | `map[string]interface{}` 保留 |
| 账号密码 | 延后至 Phase 6 云同步时实现 |

#### 数据库表结构

```sql
-- 服务器数据表
CREATE TABLE servers (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT NOT NULL,
    group_name        TEXT NOT NULL DEFAULT '',
    host              TEXT NOT NULL,
    port              INTEGER NOT NULL DEFAULT 22,
    user_name         TEXT NOT NULL DEFAULT 'root',
    password_encrypted BLOB,              -- AES-GCM 加密
    key_file          TEXT DEFAULT '',
    remark            TEXT DEFAULT '',
    options           TEXT DEFAULT '{}',  -- JSON 字符串
    created_at        TEXT DEFAULT (datetime('now')),
    updated_at        TEXT DEFAULT (datetime('now')),
    UNIQUE(group_name, name)
);

-- 应用配置表（存储 AES 静态密钥等）
CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);
```

#### 数据流

```
添加服务器:
  明文密码 → AES-GCM Encrypt(key) → BLOB → SQLite password_encrypted 列

连接服务器:
  SQLite password_encrypted → AES-GCM Decrypt(key) → 明文密码 → SSH 认证
                                ↑
               密钥自动管理：首次运行生成 → config 表 → 复用
```

---

## 4. 文档同步对齐

### 4.1 更新 6 份现有文档

| 文档 | 变更摘要 |
|------|----------|
| **`docs/modules/assh.md`** | filestore → SQLite（db.go/key.go/server.go）；AES-CBC → AES-GCM；新增完整 SQL 建表语句 |
| **`docs/modules/config.md`** | 密码管理标记 Phase 6；强调静态密钥无感加密；路径更新为 `~/.assh/v2` |
| **`docs/architecture.md`** | store 注释：加密文件存储 → SQLite 数据存储 |
| **`docs/constraints.md`** | 新增 `modernc.org/sqlite` 依赖；`kiris` 标记待确认移除 |
| **`docs/storage.md`** | 补充 SQLite 双表结构说明；`.account` 标记 Phase 6 |
| **`docs/plan.md`** | 技术选型 + 项目结构 + 存储位置 + 开发顺序全部同步 |

### 4.2 更新 2 份工程文档

| 文档 | 变更摘要 |
|------|----------|
| **`docs/TODO.md`** | Phase 2 重写：6 粗粒度任务 → 16 子任务（4 层架构 × 4 任务组） |
| **`AGENTS.md`** | 新增 SQLite 技术栈；Phase 6 加入 AuthService |

### 4.3 新建 3 份 ADR

| ADR | 核心决策 | 文件 |
|-----|----------|------|
| **ADR-008** | 废弃加密 JSON，改用 SQLite（modernc.org/sqlite） | `docs/adrs/008-SQLite替代加密JSON文件.md` |
| **ADR-009** | 服务器密码采用 AES-256-GCM 静态密钥加密 | `docs/adrs/009-服务器密码AES-GCM静态密钥加密.md` |
| **ADR-010** | AuthService 延后至 Phase 6 | `docs/adrs/010-AuthService延后至Phase6.md` |

### 4.4 ADR 索引同步

**`docs/adrs.md`** — 统计从 7 → 10 accepted，新增 Phase 2 变更摘要表。

---

## 5. adr-manager 数据库对齐

| 操作 | 详情 |
|------|------|
| 数据库路径 | `/Users/kee/.opencode/skills/adr-manager/adrs.db` |
| 工作空间 | `assh-builder` |
| 状态同步 | ADR-006、007：`proposed` → `accepted` |
| 新建条目 | ADR-001~005（DB ID 8-12）+ ADR-008~010（DB ID 13-15） |
| 最终状态 | 10 条记录，全部 `accepted`，与 markdown 文件完全对齐 |

### 双写策略

```
docs/adrs/*.md  ← 源码控制，人工可读，PR 可 diff（主记录）
adr-manager DB  ← 语义搜索，状态流转，工作空间隔离（从记录）
```

---

## 6. 项目状态总结

### Phase 2 最终状态

| 层级 | 文件 | 状态 |
|------|------|------|
| **domain** | `errors.go`, `server.go` | 📝 待实现 |
| **port** | `repository.go` | 📝 待实现 |
| **infra/store** | `db.go`, `key.go`, `server.go` | 📝 待实现 |
| **service** | `server.go` | 📝 待实现 |

### go.mod 预期变更

```
新增: modernc.org/sqlite        # 纯 Go SQLite 驱动
待确认: github.com/keesely/kiris # 如无依赖则移除
待确认: gopkg.in/yaml.v2        # JSON 替代 YAML 后可移除
```

### 依赖关系

```
Phase 0 (骨架)
  └── Phase 1 (log/config/crypto)
        └── Phase 2 (SQLite + AES-GCM) ← 📝 当前
              └── Phase 3 (SSH 连接)
                    ├── Phase 4 (CLI 组装) ← MVP
                    ├── Phase 5 (SFTP 传输)
                    ├── Phase 6 (云同步 + AuthService)
                    └── Phase 7 (代理/隧道)
```

---

## 7. 经验与抽象

### 7.1 需求分析模式

本工作流验证了一套有效的需求分析方法：

```
需求收集 → 现状分析 → 方案设计 → 用户验证 → 调整重构 → 文档对齐
```

关键在于：在编码前充分暴露设计分歧，通过原型讨论（而非代码实现）快速迭代方案。

### 7.2 架构变更的传导效应

一次存储引擎变更（JSON → SQLite）产生了连锁影响：

```
存储引擎变更
  ├── 加密方案：AES-CBC → AES-GCM
  ├── 密码策略：主密码 → 静态密钥
  ├── 服务拆分：AuthService 延后 Phase 6
  ├── 依赖变更：新增 modernc.org/sqlite
  ├── 文档更新：6 份现有文档
  └── 决策记录：3 份新 ADR
```

建议后续架构变更时，使用类似的传导清单确保无遗漏。

### 7.3 TDD 就绪状态

Phase 2 的所有模块设计已完成，具备 TDD 启动条件：

| 模块 | 接口契约 | 行为预期 | 边界条件 |
|------|----------|----------|----------|
| domain | `Server`, `Auth`, `parseName` | 验证规则、默认值 | 空名称、非法端口 |
| port | `ServerRepository` 接口 | 8 个方法签名 | error 返回 |
| store | SQLite CRUD | 密码加解密、事务 | 并发、文件损坏 |
| service | 业务编排 | 存在性检查、默认值 | 幂等性 |

---

## 8. 附录

### 8.1 相关文档索引

| 文档 | 路径 |
|------|------|
| 项目任务计划 | `docs/TODO.md` |
| 重构计划 | `docs/plan.md` |
| 架构文档 | `docs/architecture.md` |
| 约束条件 | `docs/constraints.md` |
| 存储位置 | `docs/storage.md` |
| ADR 索引 | `docs/adrs.md` |
| 模块文档 - 数据 | `docs/modules/assh.md` |
| 模块文档 - 配置 | `docs/modules/config.md` |
| ADR-008 | `docs/adrs/008-SQLite替代加密JSON文件.md` |
| ADR-009 | `docs/adrs/009-服务器密码AES-GCM静态密钥加密.md` |
| ADR-010 | `docs/adrs/010-AuthService延后至Phase6.md` |

### 8.2 关键命令

```bash
# 编译验证
cd asshv2 && CGO_ENABLED=0 go build ./...

# 静态检查
cd asshv2 && go vet ./...

# 全量测试
cd asshv2 && go test ./... -v -count=1

# ADR 管理
python3 /Users/kee/.opencode/skills/adr-manager/scripts/adr_cli.py list --workspace assh-builder
```
