# Worklog: Phase 2 数据持久化层实现与验收

- **日期**: 2026-05-09
- **作者**: PM Agent
- **标签**: `phase-2`, `sqlite`, `aes-gcm`, `implementation`, `tdd`, `acceptance`

---

## 1. 概述

本 worklog 记录 Phase 2 数据持久化层从编码实现到验收的全过程。基于 [方案设计阶段](2026-05-09-Phase2-Plan-Design.md) 确定的 16 子任务计划，采用 TDD（Red-Green-Refactor）模式驱动开发。

### 核心目标回顾

| 目标 | 说明 |
|------|------|
| **数据存储** | 废弃加密 JSON 文件，改用 SQLite（modernc.org/sqlite） |
| **密码加密** | 服务器密码采用 AES-256-GCM 静态密钥自动加解密 |
| **架构合规** | 遵循 domain → port → infra → service 三层依赖倒置 |
| **无 CGO** | 纯 Go 编译，跨平台无阻 |
| **TDD 驱动** | 每个任务先写测试后实现（Red → Green → Refactor） |

---

## 2. 实现路线

### 2.1 任务分解

```
Phase 2: 数据持久化层 (SQLite + AES-GCM)
│
├── P2.1-a domain/errors.go     ✅ 哨兵错误（5 个）
├── P2.1-b domain/server.go     ✅ 领域实体 + 验证
├── P2.2   port/repository.go   ✅ ServerRepository 接口（8 方法）
├── P2.3-a infra/store/db.go    ✅ SQLite 连接 + 自动迁移
├── P2.3-b infra/store/key.go   ✅ AES-256 密钥管理 + 密码加解密
├── P2.3-c infra/store/server.go ✅ SQLite CRUD 实现
└── P2.4   service/server.go    ✅ ServerService 用例编排
```

### 2.2 实现顺序与依赖

```
P2.1-a (哨兵错误)
  └── P2.1-b (领域实体) — 依赖 errors.go
        └── P2.2 (Repository 接口) — 依赖 domain
              └── P2.3-a (SQLite 连接) — 依赖 modernc.org/sqlite
                    ├── P2.3-b (密钥管理) — 依赖 db.go
                    └── P2.3-c (CRUD) — 依赖 db.go + key.go
                          └── P2.4 (服务层) — 依赖 port 接口
```

每一层都是下一层的基础，严格按依赖顺序实现。

---

## 3. 实现详情

### 3.1 P2.1-a 领域错误 (`asshc/domain/errors.go`)

**哨兵错误定义：**

| 错误变量 | 消息 | 使用场景 |
|----------|------|----------|
| `ErrNotFound` | `"server not found"` | Get/Delete/Move 操作目标不存在 |
| `ErrExists` | `"server already exists"` | Add/Move 操作名称冲突 |
| `ErrInvalidName` | `"invalid server name"` | 名称为空或格式错误 |
| `ErrInvalidPort` | `"invalid port number"` | 端口超出 1-65535 范围 |
| `ErrEmptyField` | `"empty field not allowed"` | 必填字段为空 |

**测试：** 6 个测试（每个错误哨兵验证 + 错误消息校验）

### 3.2 P2.1-b 领域实体 (`asshc/domain/server.go`)

**核心结构体：**

```go
type Auth struct {
    Password string `json:"password,omitempty"` // 明文密码（持久化时加密）
    KeyFile  string `json:"keyfile,omitempty"`  // SSH 私钥路径
}

type Server struct {
    Name    string                 `json:"name"`
    Group   string                 `json:"group"`
    Host    string                 `json:"host"`
    Port    int                    `json:"port"`
    User    string                 `json:"user"`
    Auth    *Auth                  `json:"auth,omitempty"`
    Remark  string                 `json:"remark,omitempty"`
    Options map[string]interface{} `json:"options,omitempty"`
}
```

**关键函数：**

| 函数 | 说明 |
|------|------|
| `ParseName(name)` | `"prod.web-01"` → `("prod", "web-01")`，按首点号分割 |
| `JoinName(group, name)` | `("prod", "web-01")` → `"prod.web-01"` |
| `ValidateServer(s)` | 验证 Name/Host/Port 合法性 |
| `NewServer(name, host, port, user)` | 工厂方法，默认 User=root, Port=22 |

**测试：** 6 个测试（parseName/JoinName/Validate/NewServer/AuthStructure/ServerStructure）

### 3.3 P2.2 接口层 (`asshc/port/repository.go`)

```go
type ServerRepository interface {
    List() (map[string]map[string]*domain.Server, error)
    Get(name string) (*domain.Server, error)
    Set(name string, server *domain.Server) error
    Delete(name string) error
    Move(from, to string) error
    Search(keyword string) (map[string]map[string]*domain.Server, error)
    GetGroup(group string) (map[string]*domain.Server, error)
    Close() error
}
```

**设计要点：**
- 只依赖 `domain` 包（`import "assh/asshc/domain"`）
- 采用 `map[string]map[string]*Server` 结构：外层 map key=group，内层 key=name
- `Close()` 用于释放数据库连接等资源

### 3.4 P2.3-a SQLite 连接 (`asshc/infra/store/db.go`)

**Store 结构体：**

```go
type Store struct {
    dbPath    string      // 数据库文件路径
    db        *sql.DB     // SQLite 连接
    mu        sync.RWMutex // 读写锁（并发安全）
    cryptoKey []byte      // AES-256 密钥（内存缓存）
}
```

**自动迁移建表：**

```sql
CREATE TABLE IF NOT EXISTS servers (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL,
    group_name         TEXT NOT NULL DEFAULT '',
    host               TEXT NOT NULL,
    port               INTEGER NOT NULL DEFAULT 22,
    user_name          TEXT NOT NULL DEFAULT 'root',
    password_encrypted BLOB,        -- AES-GCM 加密后的密码
    key_file           TEXT DEFAULT '',
    remark             TEXT DEFAULT '',
    options            TEXT DEFAULT '{}',  -- JSON 格式
    created_at         TEXT DEFAULT (datetime('now')),
    updated_at         TEXT DEFAULT (datetime('now')),
    UNIQUE(group_name, name)
);

CREATE TABLE IF NOT EXISTS config (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);
```

**测试：** 4 个测试（NewStore/Migrate/StoreLock/DbPath）

### 3.5 P2.3-b 密钥管理 (`asshc/infra/store/key.go`)

**密钥生命周期：**

```
首次运行：
  generateRandomKey(32)                   生成 32 字节密钥
  → base64.StdEncoding.EncodeToString(key) base64 编码
  → INSERT INTO config (key, value)        存入 config 表（key="aes_crypto_key"）

后续运行：
  SELECT value FROM config WHERE key=?    从 config 表读取
  → base64.StdEncoding.DecodeString(encoded) 解码
  → cryptoKey 内存缓存                     后续使用无需再次查询
```

**密码加解密：**

```
encryptPassword(plaintext):
  plaintext → AES-GCM Encrypt(key, nonce) → nonce(12B) + ciphertext + tag

decryptPassword(ciphertext):
  nonce(12B) + ciphertext + tag → AES-GCM Decrypt(key) → plaintext
```

**并发安全：**
- `getCryptoKey()` 读操作，多处调用无锁问题
- `generateAndStoreKey()` 写操作，仅在首次加密密码时调用

**测试：** 6 个测试（GenerateAndStoreKey/GetExistingKey/EncryptDecrypt/EmptyPassword/Base64Encoding）

### 3.6 P2.3-c CRUD (`asshc/infra/store/server.go`)

**各方法实现要点：**

| 方法 | 实现要点 |
|------|----------|
| `List()` | 查询全部 → 按 group 分组 → `map[group]map[name]*Server` |
| `Get(name)` | `ParseName` 解析 group/name → WHERE 查询 |
| `Set(name, server)` | 密码 AES-GCM 加密后存入 `password_encrypted` 列 |
| `Delete(name)` | 检查 `RowsAffected`，为 0 返回 `ErrNotFound` |
| `Move(from, to)` | 原子：检查来源存在 → 检查目标不冲突 → UPDATE |
| `Search(keyword)` | LIKE %keyword% 匹配 name/host/remark |
| `GetGroup(group)` | WHERE group_name = ? 查询 |
| `scanServer(row)` | 统一行扫描器，自动解密密码 |
| `Close()` | 关闭数据库连接 |

**读取时密码解密流程：**

```
scanServer:
  name, group, host, port, user, password_encrypted, key_file, remark, options
  → password_encrypted != nil → decryptPassword() → Server.Auth.Password
  → key_file != "" → Server.Auth.KeyFile
  → options != "{}" → json.Unmarshal → Server.Options
```

**测试：** 7 个测试（List/SetAndGet/Delete/Move/Search/GetGroup/ServerWithOptions）

### 3.7 P2.4 服务层 (`asshc/service/server.go`)

**ServerService — 业务规则：**

| 方法 | 前置检查 | 后置行为 |
|------|----------|----------|
| `AddServer(name, server)` | 名称不重复（`ErrExists`），Host 不能为空 | 设置默认值 Port=22, User=root |
| `UpdateServer(name, server)` | 必须存在（`ErrNotFound`） | 整体替换 |
| `RemoveServer(name)` | 必须存在（`ErrNotFound`） | 删除 |
| `MoveServer(from, to)` | 来源存在 | 目标不冲突则由 repo 处理 |
| `GetServer(name)` | 名称非空 | 委托 repo.Get |
| `ListServers()` | - | 委托 repo.List |
| `SearchServers(keyword)` | 空关键词 = 全量列表 | 委托 repo.Search |
| `GetGroup(group)` | - | 委托 repo.GetGroup |

**注入方式：**

```go
type ServerService struct {
    repo port.ServerRepository
}

func NewServerService(repo port.ServerRepository) *ServerService {
    return &ServerService{repo: repo}
}
```

**测试：** 9 个测试（NewService/Add/AddDuplicate/Get/GetNotFound/Remove/Move/List/Search）

---

## 4. 文件清单

### 4.1 新增文件（13 个）

| 文件 | 行数 | 说明 |
|------|:----:|------|
| `asshc/domain/errors.go` | 11 | 5 个哨兵错误 |
| `asshc/domain/errors_test.go` | 55 | 错误测试 |
| `asshc/domain/server.go` | 76 | Server/Auth 实体 + 函数 |
| `asshc/domain/server_test.go` | 133 | 领域测试 |
| `asshc/port/repository.go` | 14 | 8 方法接口 |
| `asshc/infra/store/db.go` | 77 | SQLite 连接 + 迁移 |
| `asshc/infra/store/db_test.go` | 99 | db 测试 |
| `asshc/infra/store/key.go` | 127 | 密钥管理 + 加解密 |
| `asshc/infra/store/key_test.go` | 143 | 密钥测试 |
| `asshc/infra/store/server.go` | 251 | SQLite CRUD |
| `asshc/infra/store/server_test.go` | 246 | CRUD 测试 |
| `asshc/service/server.go` | 120 | ServerService |
| `asshc/service/server_test.go` | 246 | 服务测试 |
| **合计** | **1,598** | |

### 4.2 修改文件（3 个）

| 文件 | 变更 |
|------|------|
| `.gitignore` | 新增 `assh/` `backup/` 忽略规则 |
| `go.mod` | 新增 `modernc.org/sqlite v1.50.0` 依赖 |
| `go.sum` | SQLite 依赖校验和 |

### 4.3 架构结构

```
asshv2/asshc/
├── domain/
│   ├── errors.go           # 哨兵错误定义
│   ├── errors_test.go
│   ├── server.go           # Server + Auth 实体
│   └── server_test.go
├── port/
│   └── repository.go       # ServerRepository 接口
├── infra/
│   └── store/
│       ├── db.go           # Store 结构体 + 迁移
│       ├── db_test.go
│       ├── key.go          # AES-256-GCM 密钥 + 加解密
│       ├── key_test.go
│       ├── server.go       # SQLite CRUD 实现
│       └── server_test.go
└── service/
    ├── server.go           # ServerService 用例编排
    └── server_test.go
```

---

## 5. 测试结果

### 5.1 测试汇总

| 包 | 测试数 | 结果 |
|----|:------:|:----:|
| `asshc/domain` | 12 | ✅ ALL PASS |
| `asshc/infra/store` | 17 | ✅ ALL PASS |
| `asshc/service` | 9 | ✅ ALL PASS |
| `log` | 34 | ✅ ALL PASS（Phase 1 遗留） |
| **Total** | **72** | **✅ 100% PASS** |

### 5.2 TDD 执行记录

| 任务 | RED（先写测试） | GREEN（实现通过） | REFACTOR |
|------|:----:|:----:|:----:|
| P2.1-a errors.go | ✅ 6 tests | ✅ | - |
| P2.1-b server.go | ✅ 6 tests | ✅ | - |
| P2.2 repository.go | -（接口无需测试） | ✅ | - |
| P2.3-a db.go | ✅ 4 tests | ✅ | - |
| P2.3-b key.go | ✅ 6 tests | ✅ | - |
| P2.3-c server.go | ✅ 7 tests | ✅（修复 1 个 nil pointer bug） | ✅ |
| P2.4 server.go | ✅ 9 tests | ✅ | - |

**测试驱动过程发现的问题：**
1. `server.go` Set 方法：`server.Auth.KeyFile` 在 Auth 为 nil 时 panic → 加 nil 检查修复
2. `server_test.go` GetGroup 测试：预期值从 2 修正为 3（与实际写入数据量一致）

---

## 6. 验证结果

### 6.1 编译验证

```bash
$ CGO_ENABLED=0 go build ./...
✅ 编译成功（无 CGO 依赖，跨平台兼容）
```

### 6.2 静态检查

```bash
$ go vet ./...
✅ 无警告/错误
```

### 6.3 架构合规检查

| 规则 | 验证方式 | 结果 |
|------|----------|:----:|
| service 只依赖 port/domain | `grep -r "asshc/infra" asshc/service/` → empty | ✅ |
| 构造器注入 | `NewServerService(repo port.ServerRepository)` | ✅ |
| 无 CGO 依赖 | `CGO_ENABLED=0 go build ./...` success | ✅ |
| 读写锁保护 | `mu.RLock/RUnlock/Lock/Unlock` 覆盖率 | ✅ |

---

## 7. Git 历史

### 7.1 当前分支

```
dev/v2.0.0-phase-2
```

### 7.2 Phase 2 相关提交

| SHA | 消息 | 日期 |
|:----|------|:----:|
| `156703a` | docs: redesign Phase 2 with SQLite + AES-GCM, add ADR-008~010 | 2026-05-09 |
| `08b0ebb` | chore: add git authorization and version/branch management strategy to PM agent | 2026-05-09 |
| `74a009b` | chore: add adrs.db to .gitignore | 2026-05-09 |
| `da8841e` | feat: implement Phase 2 data persistence layer (SQLite + AES-GCM) | 2026-05-09 |

### 7.3 提交统计

```
16 files changed, 1628 insertions(+), 93 deletions(-)
```

---

## 8. ADR 决策对齐

| ADR | 核心决策 | 代码体现 |
|-----|----------|----------|
| **ADR-008** | 废弃加密 JSON，改用 SQLite | `infra/store/db.go`, `server.go`, `go.mod` |
| **ADR-009** | AES-256-GCM 静态密钥加密 | `infra/store/key.go` |
| **ADR-010** | AuthService 延后 Phase 6 | `service/server.go` 无 auth 逻辑 |

---

## 9. 经验总结

### 9.1 TDD 有效性

- **提前暴露问题**：测试先于实现，在编码前就明确了行为的预期边界
- **安全重构**：`server.go` 修复 nil pointer 时，已有测试立即验证修改正确性
- **文档即测试**：测试代码本身就是可执行的接口文档

### 9.2 技术决策验证

- **modernc.org/sqlite** ✅ 纯 Go 编译成功，无 CGO 依赖，测试速度快（每次 <1s）
- **AES-256-GCM** ✅ 认证加密消除 padding oracle 攻击面，nonce 随机化+密文一起存储
- **读写锁（RWMutex）** ✅ List/Search/Get/GetGroup 共享锁，Set/Delete/Move 排他锁

### 9.3 改进空间

| 领域 | 现状 | 潜在改进 |
|------|------|----------|
| **测试覆盖** | 主要流程覆盖 | 补充边界测试（超大 password、特殊字符名称、并发竞争） |
| **错误处理** | 哨兵错误 | 补充 wrap errors 携带上下文信息 |
| **事务支持** | 单条 SQL 自动事务 | 批量操作需显式事务 |
| **Options 类型** | `map[string]interface{}` | 考虑更严格的类型约束 |

---

## 10. Phase 2 最终状态

```
Phase 2: 数据持久化层 (SQLite + AES-GCM)
  └── 状态: ✅ 已完成（2026-05-09）
  └── 产出: 13 新增 + 3 修改 = 16 files
  └── 测试: 72 tests ALL PASS
  └── 构建: CGO_ENABLED=0 ✅
  └── 依赖: modernc.org/sqlite v1.50.0 ✓
```

### 开发阶段总览

| Phase | 内容 | 状态 |
|-------|------|:----:|
| 0 | 项目骨架搭建 | ✅ 完成 |
| 1 | 基础设施移植（log/config/crypto） | ✅ 完成 |
| **2** | **数据持久化层（SQLite + AES-GCM）** | **✅ 完成** |
| 3 | SSH 连接层 | ⏳ 待开始 |
| 4 | CLI 命令组装 | ⏳ 待开始 |
| 5 | SFTP 文件传输 | ⏳ 待开始 |
| 6 | 云同步（Qiniu）+ AuthService | ⏳ 待开始 |
| 7 | 代理 & 隧道 | ⏳ 待开始 |

---

## 11. 附录

### 11.1 关键命令速查

```bash
# 编译
cd asshv2 && CGO_ENABLED=0 go build ./...

# 测试
cd asshv2 && go test ./asshc/... -v -count=1

# 静态检查
cd asshv2 && go vet ./...

# 全量
cd asshv2 && CGO_ENABLED=0 go build ./... && go vet ./... && go test ./... -count=1
```

### 11.2 引用文档

| 文档 | 路径 |
|------|------|
| 方案设计工作日志 | `docs/worklogs/2026-05-09-Phase2-Plan-Design.md` |
| 项目任务计划 | `docs/TODO.md` |
| 项目重构计划 | `docs/plan.md` |
| ADR 索引 | `docs/adrs.md` |
| ADR-008 | `docs/adrs/008-SQLite替代加密JSON文件.md` |
| ADR-009 | `docs/adrs/009-服务器密码AES-GCM静态密钥加密.md` |
| ADR-010 | `docs/adrs/010-AuthService延后至Phase6.md` |

### 11.3 当前 go.mod 依赖

```
module assh

go 1.22.0（自动升级至 1.25.0）

require (
    github.com/rs/zerolog v1.32.0
    github.com/urfave/cli v1.22.2
    golang.org/x/crypto v0.17.0
    modernc.org/sqlite v1.50.0    ← 新增（Phase 2）
)
```
