# TODO.md

> 开发任务计划 - 按 Phase 分阶段实施

## 开发阶段

### ✅ Phase 0: 项目骨架搭建（已完成）

目标：创建项目目录结构和基础骨架

| 任务 | 文件 | 状态 |
|------|------|------|
| P0.1 创建目录结构 | 全部空目录 | ✅ |
| P0.2 构建脚本 | `asshv2/build.sh` | ✅ |
| P0.3 程序入口 | `asshv2/main.go` | ✅ |
| P0.4 CLI 框架 | `asshv2/cmd/app.go` | ✅ |
| **验证**: `go build ./... && go vet ./... && go run . version` | | ✅ |

### ✅ Phase 1: 基础设施移植（已完成）

目标：移植日志、配置、加解密基础模块

| 任务 | 文件 | 来源 | 状态 |
|------|------|------|------|
| P1.1 日志模块 | `log/log.go` + `log_test.go` | `assh/log/` | ✅ |
| P1.1.1 日志简化 | 移除 dual log，移除级别过滤 | 2026-05-09 | ✅ |
| P1.2 配置管理 | `config/config.go` + `path.go` | `assh/config/` | ✅ |
| P1.3 RSA 加解密 | `asshc/infra/crypto/rsa.go` | `assh/config/keygen/` | ✅ |
| P1.4 AES 加解密 | `asshc/infra/crypto/aes.go` | `assh/config/keygen/` | ✅ |
| P1.5 Ed25519 密钥 | `asshc/infra/crypto/ed25519.go` | `assh/config/keygen/` | ✅ |
| P1.6 ECDSA 密钥 | `asshc/infra/crypto/ecdsa.go` | `assh/config/keygen/` | ✅ |
| P1.7 Cryptor 接口 | `asshc/port/cryptor.go` | 新写 | ✅ |
| **验证**: `go test ./log/... -v` (28 tests) | | | ✅ |

### 📝 Phase 2: 数据持久化层（当前阶段）

目标：实现基于 SQLite 的服务器数据存储，密码使用 AES-GCM 静态密钥加密

架构变更（vs v1）：
- ❌ 废弃：加密 JSON 文件 + 用户主密码（每次操作需验证）
- ✅ 改用：SQLite 数据库 + AES-256-GCM 静态密钥（首次自动生成，密钥存入 config 表，可云同步共享）
- ✅ 账号密码（account）延后至 Phase 6 云同步时实现

依赖：Phase 1 (config, crypto) + **新增** `modernc.org/sqlite`（纯 Go，无 CGO）

---

#### P2.1 领域层 — `asshc/domain/`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|
| P2.1-a 领域错误 | `asshc/domain/errors.go` | 哨兵错误：`ErrNotFound`, `ErrExists`, `ErrInvalidName`, `ErrInvalidPort` | 📝 |
| P2.1-b 领域实体 | `asshc/domain/server.go` | `Server`, `Auth` 结构体；`parseName`/`JoinName`/`Validate` | 📝 |
| **验证** | `go vet ./asshc/domain/...` | | |

**设计说明：**
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
- 名称解析：`parseName("prod.web-01")` → `("prod", "web-01")`，按首点号分割
- Server 默认值：`User="root"`, `Port=22`

---

#### P2.2 接口层 — `asshc/port/repository.go`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|
| P2.2 Repository 接口 | `asshc/port/repository.go` | `ServerRepository` 接口定义 | 📝 |
| **验证** | `go vet ./asshc/port/...` | | |

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

---

#### P2.3 基础设施层 — `asshc/infra/store/`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|
| P2.3-a SQLite 连接与迁移 | `asshc/infra/store/db.go` | `Store` 结构体、`NewStore(dbPath)`、自动建表迁移 | 📝 |
| P2.3-b 静态密钥管理 | `asshc/infra/store/key.go` | AES-256 密钥自动生成/存储/加载；密码加解密 | 📝 |
| P2.3-c CRUD 实现 | `asshc/infra/store/server.go` | SQLite CRUD（List/Get/Set/Delete/Move/Search/GetGroup） | 📝 |
| **验证** | `go test ./asshc/infra/store/... -v -count=1` | | |

**P2.3-a 数据库设计 (`db.go`):**
```sql
CREATE TABLE IF NOT EXISTS servers (
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

CREATE TABLE IF NOT EXISTS config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);
```

**P2.3-b 密钥管理 (`key.go`):**
- 首次运行：自动生成 32 字节 AES-256 密钥 → base64 编码 → 存入 `config` 表
- 后续运行：从 `config` 表读取密钥
- 密码加解密：使用 AES-256-GCM 模式

**P2.3-c CRUD 实现 (`server.go`):**
- 写入时：自动用静态密钥 AES-GCM 加密 `Password` → 存入 `password_encrypted` 列
- 读取时：自动解密 `password_encrypted` → 填充 `Server.Auth.Password`
- 其他字段（Host/Port/User/KeyFile/Remark/Options）明文存储

---

#### P2.4 服务层 — `asshc/service/server.go`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|
| P2.4 ServerService | `asshc/service/server.go` | `ServerService{repo}` CRUD 用例编排 + 业务规则 | 📝 |
| **验证** | `go test ./asshc/service/... -v -count=1` | | |

**业务规则：**
- `AddServer(name, server)`: 名称不重复（`ErrExists`），设置默认值
- `UpdateServer(name, server)`: 需存在（`ErrNotFound`）
- `RemoveServer(name)`: 需存在（`ErrNotFound`）
- `MoveServer(from, to)`: 来源存在 + 目标不冲突

---

#### Phase 2 总体验证

```bash
cd asshv2 && CGO_ENABLED=0 go build ./...   # 编译（确认无 CGO 依赖）
cd asshv2 && go vet ./...                    # 静态检查
cd asshv2 && go test ./... -v -count=1       # 全量测试
```

#### go.mod 变更

```
新增: modernc.org/sqlite    # 纯 Go SQLite 驱动（无 CGO 依赖）
待确认: 是否移除 github.com/keesely/kiris 和 gopkg.in/yaml.v2
```

### 📝 Phase 3: SSH 连接层

目标：实现 SSH 登录和远程命令执行

依赖：Phase 2 (domain/repository)

| 任务 | 文件 | 说明 |
|------|------|------|
| P3.1 Connector 接口 | `asshc/port/connector.go` | SSHConnector 接口 |
| P3.2 SSH 客户端 | `asshc/infra/ssh/client.go` | golang.org/x/crypto/ssh 封装 |
| P3.3 会话管理 | `asshc/infra/ssh/session.go` | 交互终端、keepalive |
| P3.4 连接服务 | `asshc/service/connect.go` | ConnectService 登录/命令 |
| **验证**: `go test ./asshc/... -v` | |

### 📝 Phase 4: CLI 命令组装

目标：组装完整 CLI，实现 MVP

依赖：Phase 2 + Phase 3

| 任务 | 文件 | 说明 |
|------|------|------|
| P4.1 服务器命令 | `cmd/server.go` | add/ls/info/rm/mv |
| P4.2 连接命令 | `cmd/connect.go` | login/bc |
| P4.3 全局入口 | `cmd/app.go` | 补全命令注册 |
| P4.4 DI 注入 | `main.go` | 完整依赖注入 |
| **验证**: `go run . help` | 全命令可见 |

### 📝 Phase 5: SFTP 文件传输

目标：支持文件推送和拉取

依赖：Phase 3 (ssh)

| 任务 | 文件 | 说明 |
|------|------|------|
| P5.1 Transfer 接口 | `asshc/port/transfer.go` | FileTransfer 接口 |
| P5.2 SFTP 实现 | `asshc/infra/sftp/handler.go` | pkg/sftp 封装 |
| P5.3 传输服务 | `asshc/service/transfer.go` | TransferService |
| P5.4 传输命令 | `cmd/sftp.go` | push/pull 命令 |
| **验证**: `go test ./asshc/... -v` | |

### 📝 Phase 6: 云同步 (Qiniu)

目标：实现七牛云数据备份同步

依赖：Phase 2 (storage)

| 任务 | 文件 | 来源 |
|------|------|------|
| P6.1 Syncer 接口 | `asshc/port/syncer.go` | 新写 |
| P6.2 Qiniu 同步 | `asshc/infra/sync/qiniu/` | 从 `asshv2/lib/qiniu/` 移植重构 |
| P6.3 同步服务 | `asshc/service/sync.go` | SyncService |
| P6.4 同步命令 | `cmd/sync.go` | sync push/pull/account |
| P6.5 清理旧代码 | 移除 `asshv2/lib/qiniu/` | |
| **验证**: `go test ./asshc/... -v` | |

### 📝 Phase 7: 代理 & 隧道

目标：实现 SOCKS5 代理、端口转发、SSH 隧道

依赖：Phase 3 (ssh)

| 任务 | 文件 | 说明 |
|------|------|------|
| P7.1 Proxy 接口 | `asshc/port/proxy.go` | Proxy 接口 |
| P7.2 SOCKS5 | `asshc/infra/proxy/socks5.go` | SOCKS5 代理 |
| P7.3 端口转发 | `asshc/infra/proxy/port.go` | 本地/远程端口转发 |
| P7.4 SSH 隧道 | `asshc/infra/tunnel/tunnel.go` | 隧道管理 |
| P7.5 代理服务 | `asshc/service/proxy.go` | ProxyService |
| P7.6 代理命令 | `cmd/proxy.go` | proxy/hostproxy/localproxy |
| **验证**: `go test ./asshc/... -v` | |

## 依赖顺序

```
Phase 0 (骨架)
  └── Phase 1 (基础设施)
        └── Phase 2 (数据持久化)
              └── Phase 3 (SSH 连接)
                    ├── Phase 4 (CLI 组装) ← MVP
                    ├── Phase 5 (SFTP 传输)
                    ├── Phase 6 (云同步)
                    └── Phase 7 (代理/隧道)
```

## 验证策略

每个 Phase 完成后执行：

```bash
cd asshv2 && go build ./...     # 编译
cd asshv2 && go vet ./...       # 静态检查
cd asshv2 && go test ./... -v   # 测试
```

Phase 4 起额外验证：

```bash
cd asshv2 && go run . help      # CLI 可用
cd asshv2 && go run . version   # 版本输出
```
