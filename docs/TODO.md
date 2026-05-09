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

### ✅ Phase 2: 数据持久化层（已完成）

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
| P2.1-a 领域错误 | `asshc/domain/errors.go` | 哨兵错误：`ErrNotFound`, `ErrExists`, `ErrInvalidName`, `ErrInvalidPort` | ✅ |
| P2.1-b 领域实体 | `asshc/domain/server.go` | `Server`, `Auth` 结构体；`parseName`/`JoinName`/`Validate` | ✅ |
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
| P2.2 Repository 接口 | `asshc/port/repository.go` | `ServerRepository` 接口定义 | ✅ |
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
| P2.3-a SQLite 连接与迁移 | `asshc/infra/store/db.go` | `Store` 结构体、`NewStore(dbPath)`、自动建表迁移 | ✅ |
| P2.3-b 静态密钥管理 | `asshc/infra/store/key.go` | AES-256 密钥自动生成/存储/加载；密码加解密 | ✅ |
| P2.3-c CRUD 实现 | `asshc/infra/store/server.go` | SQLite CRUD（List/Get/Set/Delete/Move/Search/GetGroup） | ✅ |
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
| P2.4 ServerService | `asshc/service/server.go` | `ServerService{repo}` CRUD 用例编排 + 业务规则 | ✅ |
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

### ✅ Phase 3: SSH 连接层（已完成）

目标：实现 SSH 登录、远程命令执行、交互式终端

依赖：Phase 2 (domain/repository)
设计文档：[Phase3-Design.md](Phase3-Design.md)
架构决策：ADR-011（接口分离 + 可配置 HostKey + Keepalive）

#### P3.1 接口层 — `asshc/port/`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|
| P3.1-a Connector 接口 | `asshc/port/connector.go` | `SSHConnector` 接口：`Connect(server)` / `Close(client)` | ✅ |
| P3.1-b Session 接口 | `asshc/port/session.go` | `SSHSession` 接口：`Shell(client)` / `Run(client, cmd)` / `RunWithOutput(client, cmd)` | ✅ |
| P3.1-c HostKey 接口 (预留) | `asshc/port/hostkey.go` | `HostKeyChecker` 接口定义，`HostKeyCallback` 类型别名 | ✅ |
| **验证** | `go vet ./asshc/port/...` | | ✅ |

**设计说明：**
```go
// SSHConnector — 连接管理
type SSHConnector interface {
    Connect(server *domain.Server) (*ssh.Client, error)
    Close(client *ssh.Client) error
}

// SSHSession — 会话操作  
type SSHSession interface {
    Shell(client *ssh.Client) error
    Run(client *ssh.Client, cmd string) error
    RunWithOutput(client *ssh.Client, cmd string) (string, error)
}

// HostKeyChecker (预留)
type HostKeyCallback func(hostname string, remote net.Addr, key ssh.PublicKey) error
type HostKeyChecker interface {
    Check(hostname string, remote net.Addr, key ssh.PublicKey) error
}
```

---

#### P3.2 基础设施层 — `asshc/infra/ssh/`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|
| P3.2-a SSH 客户端 | `asshc/infra/ssh/client.go` | 实现 `SSHConnector`：认证链(Agent→私钥→密码)、HostKey策略、Keepalive | ✅ |
| P3.2-b 会话管理 | `asshc/infra/ssh/session.go` | 实现 `SSHSession`：交互终端(pty+SIGWINCH)、命令执行 | ✅ |
| P3.2-c HostKey 验证 | `asshc/infra/ssh/hostkey.go` | KnownHosts + Insecure 两种策略实现 | ✅ |
| P3.2-d 单元测试 | `asshc/infra/ssh/client_test.go` | mock SSH server 测试连接/认证/HostKey | ✅ |
| P3.2-e 单元测试 | `asshc/infra/ssh/session_test.go` | 会话 mock 测试 | ✅ |
| P3.2-f 单元测试 | `asshc/infra/ssh/hostkey_test.go` | HostKey 检查测试 | ✅ |
| **验证** | `go test ./asshc/infra/ssh/... -v -count=1` | | ✅ |

**认证流程：**
```
Connect(server)
  ├── 检测 SSH_AUTH_SOCK → Agent 认证（优先）
  ├── 检测 Auth.KeyFile  → 私钥认证
  ├── 检测 Auth.Password → 密码认证
  ├── HostKey 策略: Options["insecure_skip_hostkey"] ? InsecureIgnoreHostKey : known_hosts
  └── Keepalive: Options["keepalive"] → 心跳间隔
```

---

#### P3.3 服务层 — `asshc/service/`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|
| P3.3-a 连接服务 | `asshc/service/connect.go` | `ConnectService{connector, session, repo}` | ✅ |
| P3.3-b 单元测试 | `asshc/service/connect_test.go` | mock connector+session，测试所有方法 | ✅ |
| **验证** | `go test ./asshc/service/... -v -count=1` | | ✅ |

**ConnectService 方法：**
| 方法 | 入口 | 用途 |
|------|------|------|
| `ConnectByName(name)` | 按服务器名 | 从 repo 获取 Server → 连接 |
| `ConnectDirect(host, port, user, password, keyFile)` | 直连参数 | 构建 Server 对象 → 连接 |
| `Shell(client)` | 已连接 | 交互式终端 |
| `Run(client, cmd)` | 已连接 | 执行命令（输出到 stdout） |
| `RunWithOutput(client, cmd)` | 已连接 | 执行命令（返回字符串） |

---

#### P3.4 DI 注入

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|
| P3.4-a DI 注入 | `main.go` | 注入 SSHConnector/SSHSession/ConnectService | ✅ |

---

#### Phase 3 总体验证

```bash
cd asshv2 && go build ./...      # 编译 ✅
cd asshv2 && go vet ./...        # 静态检查 ✅
cd asshv2 && go test ./... -v -count=1   # 全量测试 ✅ (6 包全部通过)
```

### ✅ Phase 4: CLI 命令组装（已完成）

目标：组装完整 CLI，实现 MVP

依赖：Phase 2 + Phase 3 + 全局参数对齐 ssh（ADR-012）
设计文档：[Phase4-Design.md](Phase4-Design.md)
架构决策：ADR-012（CLI 参数对齐与命令架构）

---

#### P4.1 `cmd/server.go` — 服务器管理命令（新建）

| 任务 | 说明 | 验证场景 | 状态 |
|------|------|----------|------|
| P4.1-a add 子命令 | `assh server add <name> -H <host> [-pPuik]` → `serverSvc.AddServer()` | 添加成功、重复、参数缺失 | ✅ |
| P4.1-b ls 子命令 | `assh server ls [--group] [--search]` → `serverSvc.List/Search/GetGroup` | 全部列表、分组筛选、关键词搜索 | ✅ |
| P4.1-c info 子命令 | `assh server info <name>` → `serverSvc.GetServer()` | 存在、不存在 | ✅ |
| P4.1-d rm 子命令 | `assh server rm <name>` → `serverSvc.RemoveServer()` | 存在、不存在 | ✅ |
| P4.1-e mv 子命令 | `assh server mv <from> <to>` → `serverSvc.MoveServer()` | 成功、冲突、不存在 | ✅ |
| P4.1-f 注册 | `registerServerCommands()` 关联到 App | help 显示 server 子命令 | ✅ |

参数规格（对齐 ADR-012）：
```
-H/--host    必填，主机地址
-p/--port    端口（默认22）
-u/-l/--user/--login    用户名（默认root）
-P/--password    密码（明文/交互）
-i/--identity-file   密钥文件
-k/--key    密钥文件（同 -i）
-o/--option    Options，可多次
--remark    备注
--group    分组
```

---

#### P4.2 `cmd/connect.go` — 连接管理命令（新建）

| 任务 | 说明 | 验证场景 | 状态 |
|------|------|----------|------|
| P4.2-a login 命令 | 自动识别：按名 / user@host / 分离参数 → `Connect` → `Shell` | 三种登录方式、参数缺失 | ✅ |
| P4.2-b run 命令 | `assh run <name> <command>` → 连接 → 执行 → 关闭 | 正常执行、空命令、server 不存在 | ✅ |
| P4.2-c bc 命令 | 批量执行 + 三通道输出（goroutine 并发） | 多服务器、空结果、全部失败 | ✅ |
| P4.2-d 注册 | `registerConnectCommands()` 关联到 App | help 显示 login/run/bc | ✅ |

参数规格（对齐 ADR-012）：
```
login 自动识别：
  - 含 "@" → user@host 直连
  - 含 "-H/--host" → 分离参数直连
  - 纯 name → 数据库查找

bc 三通道输出：
  通道1: 实时日志 /tmp/assh/bclogs/{server}.log
  通道2: stdout 标头 [server] 实时输出
  通道3: JSON 汇总（全部完成后）
  --log /path/file.json    自定义输出路径（默认 ./bc-result-{ts}.json）
  --servers s1,s2    指定服务器列表
  --group name    按分组执行
```

---

#### P4.3 `cmd/app.go` — 命令注册调整（修改）

| 任务 | 说明 | 状态 |
|------|------|------|
| P4.3-a `NewApp` 签名变更 | 接收 `*service.ServerService` 参数 | ✅ |
| P4.3-b 全局参数注册 | `-v/--verbose`, `-q/--quiet`, `-F/--config` | ✅ |
| P4.3-c 子命令注册 | 调用 `registerServerCommands()` + `registerConnectCommands()` | ✅ |
| P4.3-d `beforeAction` 完善 | 数据库路径初始化、日志初始化 | ✅ |

---

#### P4.4 `main.go` — DI 注入（修改）

| 任务 | 说明 | 状态 |
|------|------|------|
| P4.4-a Store 初始化 | `store.NewStore(dbPath)`，添加错误处理 | ✅ |
| P4.4-b ServerService 注入 | `service.NewServerService(repo)` | ✅ |
| P4.4-c ConnectService repo 补全 | 将 `nil` 替换为真实 repo | ✅ |
| P4.4-d App 构建 | 传递 `serverSvc` 给 `cmd.NewApp()` | ✅ |

---

#### 验证结果

```bash
cd asshv2 && go build ./...         # 编译 ✅
cd asshv2 && go vet ./...           # 静态检查 ✅
cd asshv2 && go test ./... -count=1 # 全量测试 ✅（7 包全部通过）
cd asshv2 && go run . help          # CLI 可用 ✅
cd asshv2 && go run . version       # 版本输出 ✅ (v2.0.0)
```

#### 验收清单

| # | 场景 | 操作 | 预期 | 结果 |
|---|------|------|------|------|
| F1 | 添加服务器 | `assh server add t -H 127.0.0.1 -P test` | 成功 | ✅ |
| F2 | 列表 | `assh server ls` | 显示分组 | ✅ |
| F3 | 详情 | `assh server info t` | 显示完整信息 | ✅ |
| F4 | 删除 | `assh server rm t` | 成功 | ✅ |
| F5 | 移动 | `assh server mv t t2` | 成功 | ✅ |
| F6 | 按名登录 | `assh login t` | 连接 mock | ✅ |
| F7 | user@host | `assh login root@127.0.0.1` | 自动解析 | ✅ |
| F8 | 远程执行 | `assh run t "echo ok"` | 输出 ok | ✅ |
| F9 | 批量执行 | `assh bc "echo ok" --servers s1,s2` | 标头 + 结果 | ✅ |
| F10 | 帮助 | `assh help` | 完整命令列表 | ✅ |

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
