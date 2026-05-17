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
- ✅ 账号密码（account）延后至 Phase 8 云同步时实现

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
补充（2026-05-09）：set(upsert) + rollback + 版本管理/changelog
补充（2026-05-10）：ADR-014 server 命令组扁平化，add/set/ls/info/rm/mv/rollback 上移为顶层命令

依赖：Phase 2 + Phase 3 + 全局参数对齐 ssh（ADR-012）
设计文档：[Phase4-Design.md](Phase4-Design.md)
架构决策：ADR-012（CLI 参数对齐与命令架构）, ADR-013（配置版本管理与回滚）, ADR-014（server 命令扁平化）

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
| P4.1-g set 子命令 (原 edit) | `assh server set <name> [flags]` → upsert (`SetServer`) | 创建新服务器、更新已存在、参数验证(port范围) | ✅ |
| P4.1-h rollback 子命令 | `assh server rollback <name> <version>` → `RollbackServer()` | 版本回滚、--list 查看历史、版本不存在 | ✅ |
| P4.1-i 版本管理 | `asshc/infra/store/` Set 自动版本递增+快照+changelog | 创建v1→更新v2→回滚v3(rollback) | ✅ |
| P4.1-j 参数验证 | set 命令执行 port 范围校验 (1-65535)、新建 host 必填检查 | 非法端口拒绝、无 host 拒绝 | ✅ |

参数规格：
```
set (upsert, 替代原来的 edit):
  -H/--host    地址（新建必填）
  -p/--port    端口（1-65535 校验）
  -u/-l/--user/--login  用户名
  -P/--password    密码
  -i/--identity-file   密钥文件
  -k/--key    密钥文件（同 -i）
  -o/--option    添加/替换 Options
  --remark    备注
  --clear-password    清除密码
  --clear-key    清除密钥文件

rollback:
  <name> <version>    按版本号回滚
  <name> --list       查看变更历史

行为：
  - 服务器不存在 → 用提供的参数创建新服务器
  - 服务器已存在 → 仅修改显式指定的字段（深拷贝 Options，c.IsSet 检测）
  - 每次修改自动记录快照到 server_changelog 表
  - Version 显示在 info/ls 输出中
  - rollback 创建新的版本条目（类型=rollback），不覆盖历史
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

#### 2026-05-09 新增功能验收

```bash
# set (upsert) — 21 场景全通过
# 版本管理 — 版本自动递增验证通过
# rollback — 回滚到 v1 验证配置正确性通过
# 详情见 docs/worklogs/2026-05-09-Phase4-Set-Rollback-Acceptance.md
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
| F11 | set 创建新服务器 | `assh server set t -H 1.2.3.4 -P pass` | 创建成功 v1 | ✅ |
| F12 | set 更新已存在 | `assh server set t --remark "new"` | 版本递增 | ✅ |
| F13 | set 无标志 | `assh server set t` | 显示详情+提示 | ✅ |
| F14 | set 无 host 新建 | `assh server set newbox` | 报错提示 | ✅ |
| F15 | set 清理密码 | `assh server set t --clear-password` | 密码清除 | ✅ |
| F16 | rollback --list | `assh server rollback t --list` | 显示 changelog | ✅ |
| F17 | rollback 版本号 | `assh server rollback t 1` | 回滚到 v1 | ✅ |
| F18 | ls 显示版本 | `assh server ls` | 有 Version 列 | ✅ |
| F19 | info 显示版本 | `assh server info t` | 有 Version 行 | ✅ |
| F20 | info 模糊建议 | `assh info case.bbp` → 建议 `case.bbp2` | 智能提示 | ✅ |
| F21 | 默认 Action 建议 | `assh case.bbp` → 建议 `case.bbp2` | 智能提示 | ✅ |
| F22 | ls 自适应列宽 | `assh ls` → 紧凑表格，Group 右对齐 | 美观紧凑 | ✅ |

---
#### Phase 4 细化与体验优化（2026-05-10）

在本轮对 Phase 4 已完成的命令做了用户体验和输出格式的精细化调整：

| 任务 | 说明 | 状态 |
|------|------|------|
| P4.4-a info 格式重构 | 居中标题+对齐加号、Auth Type 区分、标签首字母大写+Server前缀 | ✅ |
| P4.4-b info 模糊建议 | 服务器不存在时前缀匹配+Levenshtein 给出建议名称 | ✅ |
| P4.4-c ls 自适应列宽 | Group/Name/Host 按最长内容自适应，Group 右对齐 | ✅ |
| P4.4-d 默认 Action 模糊建议 | `assh \<target\>` 找不到服务器时也给出近似提示 | ✅ |
| P4.4-e Shell 补全支持 | info/rm/mv/rollback/login/run/bc 命令的服务器名 Tab 补全 | ✅ |
| P4.4-f add/set 确认流程 | 新增/修改前预览信息 → 确认保存 → 显示结果 | ✅ |
| P4.4-e 自动编译检查 | 每次代码变更后自动 `go build ./...` 验证 | ✅ |

**相关文件**: `cmd/server.go`, `cmd/app.go`, `asshc/service/server.go`
**验证**: `go build ./... && go test ./... -count=1` ✅ (7 包通过)

---

### ✅ Phase 5: SFTP 文件传输（已完成）

验收：F1-F28 全部通过 | 完成时间：2026-05-11

目标：支持文件推送和拉取，包含递归传输、断点续传、并发传输、进度显示、通配符匹配

依赖：Phase 3 (ssh)
设计文档：`docs/modules/sftp.md`
架构决策：ADR-017（SFTP 文件传输架构设计）

**新增依赖**：`github.com/pkg/sftp` v1.13.7+

---

#### P5.1 FileTransfer 接口 ✅（已实现）

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|:----:|
| P5.1-a 接口定义 | `asshc/port/transfer.go` | FileTransfer (Push/Pull/List/Remove/Mkdir) + TransferProgress + FileInfo + TransferResult + PushBatch/PullBatch | ✅ |
| **验证** | `go vet ./asshc/port/...` | | |

---

#### P5.2 SFTP 基础设施 — `asshc/infra/sftp/`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|:----:|
| P5.2-a 连接管理 | `asshc/infra/sftp/client.go` | SFTPSession 封装（创建 SFTP 客户端、关闭、重连） | ⬜ |
| P5.2-b Hash Header | `asshc/infra/sftp/header.go` | 256B 固定头部构建/解析/剥离（ADR-019） | ⬜ |
| P5.2-c 文件上传 | `asshc/infra/sftp/upload.go` | Push 实现：单文件/目录递归(-r)/断点续传(--resume)/hash header/进度回调/Glob匹配 | ⬜ |
| P5.2-d 文件下载 | `asshc/infra/sftp/download.go` | Pull 实现：单文件/目录递归(-r)/断点续传(--resume)/hash header/进度回调/Glob匹配 | ⬜ |
| P5.2-e 远程操作 | `asshc/infra/sftp/ops.go` | List/Remove/Mkdir 实现 | ⬜ |
| P5.2-f 进度显示 | `asshc/infra/sftp/progress.go` | 进度条显示（百分比、速率、ETA、传输字节） | ⬜ |
| P5.2-g 传输后验证 | `asshc/infra/sftp/verify.go` | 传输完成后比对文件大小，可选 SHA256 校验 | ⬜ |
| P5.2-h 单元测试 | `asshc/infra/sftp/*_test.go` | mock/local SFTP server 测试 | ⬜ |
| **验证** | `go test ./asshc/infra/sftp/... -v -count=1` | | |

**进度显示格式：**
```
( 1/5) [project.tar.gz] .............. 58%  2.1MB/s  ETA 4s
( 2/5) [config.json] ................. 100%  5.3MB/s  ETA 0s

verifying: project.tar.gz
  ✓ size match: 12.3MB / 12.3MB
  ✓ sha256: a3f2c8d1... (--checksum 开启时)
```

**回调接口**：
```go
type ProgressInfo struct {
    Index    int     // 当前文件序号（1-based）
    Total    int     // 总文件数
    FileName string  // 当前文件名
    Progress float64 // 0.0 ~ 1.0
    Rate     string  // 格式化速率，如 "2.1MB/s"
    ETA      string  // 格式化剩余时间，如 "4s"
}
type TransferProgress func(info ProgressInfo)
```

**断点续传策略（ADR-017 + ADR-019）：**
- **Hash Header 安全验证**：目标文件头部 256B 固定格式 header，记录源文件 SHA256
  ```
  [Magic:4B][Version:2B][HashType:2B][OrigSize:8B][SHA256:64B][Reserved:176B]
  ```
- **首次传输流程**：
  1. 计算源文件 SHA256
  2. 构建 256B header
  3. 先发送 header → 再发送文件内容
  4. 传输完成后 → 重写目标文件，剥离 header（恢复到原始格式）
- **续传流程（hash header 优先）**：
  1. 读取目标文件前 256B → 解析 hash header
  2. 计算当前源文件 SHA256
  3. hash 一致 → `Seek(256)` 从头部后继续传输
  4. hash 不一致 → 文件被修改，询问是否覆盖
  5. header 不存在或无法解析 → 回退到纯大小比对
- **上传详细流程**：
  1. `sftp.Stat(remotePath)` → 获取远程文件大小
  2. 读取远程前 256B 解析 header（如有）
  3. hash 验证通过 → `localFile.Seek(256 + offset)` → 续传剩余
  4. 完成后重写远程文件剥离 header
- **下载详细流程**：
  1. `os.Stat(localPath)` → 获取本地文件大小
  2. 读取本地前 256B 解析 header（如有）
  3. hash 验证（通过 SSH exec `sha256sum` 获取远程 hash）
  4. `remoteFile.Seek(256 + offset)` → 续传剩余
  5. 完成后重写本地文件剥离 header
- **SFTP 客户端配置**：
  ```go
  sftp.NewClient(sshClient,
      sftp.UseConcurrentReads(true),       // 默认已开启
      sftp.UseConcurrentWrites(true),      // 并发写入
      sftp.MaxConcurrentRequestsPerFile(64),
  )
  ```

**传输后验证：**
- **大小比对（始终执行）**：传输完成后 `sftp.Stat()` vs `os.Stat()` 比对文件大小
- **SHA256 校验（可选 --checksum）**：通过 SSH exec 在远程执行 `sha256sum` 或本地计算后比对
- **验证失败处理**：输出详细错误信息，保留文件不自动删除

---

#### P5.3 传输服务 — `asshc/service/`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|:----:|
| P5.3-a TransferService | `asshc/service/transfer.go` | 传输用例编排（PushFile/PullFile/ListRemote/RemoveRemote/MkdirRemote） | ⬜ |
| P5.3-b 覆盖策略 | `asshc/service/transfer.go` | 交互询问/--force强制覆盖/--skip跳过 | ⬜ |
| P5.3-c 服务构造 | `asshc/service/transfer.go` | `NewTransferService(transfer, repo)` 注入 | ⬜ |
| P5.3-d Mock 测试 | `asshc/service/transfer_test.go` | mock FileTransfer 覆盖 Push/Pull/List/Remove + 覆盖策略 | ⬜ |
| **验证** | `go test ./asshc/service/... -v -count=1` | | |

**TransferService 方法设计：**
```go
type TransferService struct {
    transfer port.FileTransfer
    repo     port.ServerRepository
}

// PushFile 从本地推送文件到远程服务器。
// 支持递归目录 (-r)、断点续传 (--resume)、强制覆盖 (-f/--force)、跳过已存在 (--skip)
func (s *TransferService) PushFile(ctx context.Context, name, localPath, remotePath string, opts TransferOptions) error

// PullFile 从远程服务器拉取文件到本地。
func (s *TransferService) PullFile(ctx context.Context, name, remotePath, localPath string, opts TransferOptions) error

// ListRemote 列出远程目录内容。
func (s *TransferService) ListRemote(name, remotePath string) ([]port.FileInfo, error)

// RemoveRemote 删除远程文件或目录。
func (s *TransferService) RemoveRemote(name, remotePath string) error

// MkdirRemote 在远程服务器上创建目录。
func (s *TransferService) MkdirRemote(name, remotePath string) error
```

**TransferOptions：**
```go
type TransferOptions struct {
    Recursive      bool   // -r: 递归目录（保持目录结构；Glob 平铺到目标目录）
    Resume         bool   // --resume/-e: 断点续传
    Overwrite      string // ""=询问, "force"=--force/-f, "skip"=--skip
    Concurrency    int    // 多文件 goroutine 池大小（默认 3，单文件内部由 pkg/sftp 控制 MaxConcurrentRequestsPerFile=64）
    Progress       bool   // 显示进度条
    VerifyChecksum bool   // --checksum/-S: 传输后 SHA256 校验
}
```

**批量传输行为**：
- 单个文件失败 → 继续传后续文件
- 覆盖策略 ya/na → 本次会话有效
- 远程/本地目录不存在 → 询问后创建（push/pull 一致）

---

#### P5.4 CLI 命令 — `cmd/`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|:----:|
| P5.4-a push 命令 | `cmd/sftp_cmd.go` + `cmd/fs/main.go` | `assh-fs push <server> <local> <remote>` 文件上传 | ⬜ |
| P5.4-b pull 命令 | `cmd/sftp_cmd.go` + `cmd/fs/main.go` | `assh-fs pull <server> <remote> [local]` 文件下载 | ⬜ |
| P5.4-c 交互命令 | `cmd/sftp_cmd.go` + `cmd/fs/main.go` | `assh-fs <server>` 交互式 SFTP 会话 | ⬜ |
| P5.4-d 命令参数 | `cmd/sftp_cmd.go` | `-r`/`--resume`/`-e`/`-f/--force`/`--skip`/`--checksum`/并发数/进度开关 | ⬜ |
| P5.4-e 直连参数 | `cmd/sftp_cmd.go` | `-H`/`-u`/`-p`/`-P`/`-i`/`-k` 直连支持 | ⬜ |
| P5.4-f 入口集成 | `cmd/fs/main.go` | assh-fs 独立入口（DI 组合根 + 命令注册） | ⬜ |
| P5.4-g 共享逻辑 | `cmd/bootstrap.go` | 提取公共初始化逻辑（DB/Service 创建），供 assh 和 assh-fs 共用 | ⬜ |
| **验证** | `go build -o assh-fs ./cmd/fs/ && assh-fs help` | 显示 push/pull/交互命令 | |

**用法示例：**
```bash
# 推送文件
assh-fs push myserver ./local/file.txt /remote/path/
assh-fs push -r myserver ./local/dir/ /remote/dir/       # 递归目录
assh-fs push --resume myserver ./large.tar.gz /remote/    # 断点续传
assh-fs push -e myserver ./large.tar.gz /remote/          # 断点续传（短参）
assh-fs push --skip myserver ./file.txt /remote/          # 跳过已存在
assh-fs push -f myserver ./file.txt /remote/              # 强制覆盖

# 拉取文件
assh-fs pull myserver /remote/file.txt ./local/
assh-fs pull -r myserver /remote/dir/ ./local/

# 交互式 FS 会话
assh-fs myserver

# 传输后 SHA256 校验
assh-fs push --checksum myserver ./important.iso /remote/

# 直连
assh-fs push -H 192.168.1.1 -u root -P mypass /local/file /remote/
```

---

#### P5.5 集成测试

| 任务 | 说明 | 状态 |
|------|------|:----:|
| P5.5-a 集成测试 | 使用 embed 测试服务器或容器化 SFTP server 测试全链路 push/pull | ⬜ |
| P5.5-b **认证回退** | `asshc/infra/ssh/client.go` — key 失败后回退 password | ✅ BUG-010 |
| **验证** | `go test ./... -v -count=1` | |

---

#### P5.6 依赖注入

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|:----:|
| P5.6-a go.mod 更新 | `go.mod` | 添加 `github.com/pkg/sftp` | ✅ |
| P5.6-b DI 注入 | `main.go` | 注入 `FileTransfer` + `TransferService` 到 `cmd.NewApp()` | ✅ |
| P5.6-c App 签名变更 | `cmd/app.go` | `NewApp` 增加 `transferSvc` 参数 | ✅ |
| **验证** | `go build ./... && go vet ./...` | | ✅ |

---

#### P5.7 CLI 组合短参数支持（RQ-001）

参见：[RQ-001 详情](../requirements/RQ-001-combined-short-flags.md)

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|:----:|
| P5.7-a expandCombinedFlags | `main.go` | 实现 `os.Args` 预处理：`-qnt` → `-q -n -t` | ✅ |
| P5.7-b bool flag 白名单 | `main.go` | 定义已知 bool 短参集合 `{"v","q","V"}`，预留 `-e`(resume) | ✅ |
| P5.7-c 边界处理 | `main.go` | `-p22` 保持原样、未知 flag 报错、`--` 分隔符、混入带值 flag 不展开 | ✅ |
| **验证** | `go build ./... && go test ./... -v -count=1` | | ✅ | |

---

#### Phase 5 验收清单

| # | 场景 | 操作 | 预期 | 状态 |
|---|------|------|------|:----:|
| F1 | 推送单文件 | `assh-fs push t ./a.txt /tmp/` | 文件传输成功 | ✅ |
| F2 | 拉取单文件 | `assh-fs pull t /tmp/a.txt ./` | 文件下载成功 | ✅ |
| F3 | 递归目录上传 | `assh-fs push -r t ./mydir/ /tmp/` | 目录递归传输 | ✅ |
| F4 | 递归目录下载 | `assh-fs pull -r t /tmp/mydir/ ./` | 目录递归拉取 | ✅ |
| F5 | 断点续传 | `assh-fs push --resume t ./large.iso /tmp/` | 中断后续传 | ✅ |
| F6 | 强制覆盖 | `assh-fs push -f t ./a.txt /tmp/a.txt` | 已存在时覆盖 | ✅ |
| F7 | 跳过已存在 | `assh-fs push --skip t ./a.txt /tmp/a.txt` | 已存在时跳过 | ✅ |
| F8 | 默认询问 | `assh-fs push t ./a.txt /tmp/a.txt` | 交互式询问 | ✅ |
| F14 | Glob 平铺 | `assh-fs push t "*.log" /tmp/logs/` | 匹配多个文件，平铺到 /tmp/logs/ | ✅ |
| F15 | Server 不存在 | `assh-fs push nonexist ./a.txt /tmp/` | 报错提示 | ✅ |
| F16 | SHA256 校验 | `assh-fs push -S t ./file /tmp/` | 传输后校验 checksum 并显示结果 | ✅ |
| F17 | 验证失败检测 | 模拟传输后大小不匹配 | 输出验证失败错误，保留文件 | ✅ |
| F18 | 批量续传 | 中断后 `assh-fs push --resume -r t ./dir/ /remote/` | 从断点继续批量传输 | ✅ |
| F19 | 断点边界：远程>本地 | `assh-fs push --resume t ./small /remote/big` | 报错"远程文件比本地大，续传不可用" | ✅ |
| F20 | Glob pull 平铺 | `assh-fs pull t "/var/log/*.log" ./logs/` | 平铺到 ./logs/ | ✅ |
| F21 | 交互会话：完整命令 | `assh-fs t` 内执行 ls/cd/get/put/chmod/rename/ln/df/umask/lls/lcd/lpwd | 全命令可用 | ✅ |
| F22 | RQ-001: 两个 bool 组合 | `assh -qv root@host` | 等价于 `-q -v` | ✅ |
| F23 | RQ-001: 三个 bool 组合 | `assh -qVv root@host` | 等价于 `-q -V -v` | ✅ |
| F24 | RQ-001: 单个 bool 不变 | `assh -q root@host` | 行为不变 | ✅ |
| F25 | RQ-001: 组合+带值参数混用 | `assh -qv root@host -c "uptime"` | bool 组合正常，带值参数正常 | ✅ |
| F26 | RQ-001: 长参数不受影响 | `assh --quiet --verbose root@host` | 行为不变 | ✅ |
| F27 | RQ-001: 组合中含未知 flag | `assh -qx root@host` | 报错未知 flag `-x` | ✅ |
| F28 | RQ-001: 混入带值短参不展开 | `assh -qp root@host` | 报错未知 flag（urflags原始行为） | ✅ |
| **验证**: `go build ./... && go test ./... -v -count=1` | | | ✅ |

---

#### P5.8 v1→v2 配置导入

参见：[ADR-024](adrs/024-v1-to-v2-migration.md) | [设计文档](worklogs/2026-05-11-auth-fallback-and-v1-import.md)

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|:----:|
| P5.8-a v1 解密核心 | `asshc/service/importer.go` | RSA PKCS1v15 + AES-CBC 解密 v1 servers.db | ⬜ |
| P5.8-b import 命令 | `cmd/import.go` | `assh import v1` 子命令 + 交互式确认 | ⬜ |
| P5.8-c 密钥复制 | `asshc/service/importer.go` | 扫描 v1 data/ 目录复制密钥到 v2 credentials/ | ⬜ |
| P5.8-d 写入 v2 SQLite | `asshc/service/importer.go` | 通过 Store API 写入 v2 数据库 | ⬜ |
| **验证** | `go build ./... && go test ./... -v -count=1` | | |

---

#### P5.9 认证回退 (BUG-010)

参见：[BUG-010](bugs/BUG-010-密钥认证失败后未回退到密码认证.md)

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|:----:|
| P5.9-a 两轮认证 | `asshc/infra/ssh/client.go` | key 失败后 Password-only 回退 | ✅ |
| **验证** | `go build ./... && go test ./... -v -count=1` | | ✅ |

---

### 🔄 Phase 6: 密钥管理 (Key Management)

目标：实现 SSH 密钥自动生成、加密存储、轮换管理，支持 `-K`/`--keygen` 快捷生成

详情：参见 [docs/modules/key-management.md](modules/key-management.md)

依赖：Phase 2 (SQLite) + Phase 3 (SSH) + Phase 4 (CLI) — 不依赖 Phase 5 (SFTP)

**阶段优先级**: 先 known-servers 表 → KeyManager 接口 → infra 层 → service 层 → CLI 命令 → 集成

| 编号 | 任务 | 文件名 | 说明 | 优先级 |
|------|------|--------|------|--------|
| P6.1 | KeyManager 接口 | `asshc/port/keymanager.go` | Generate/DeployPublicKey/Backup/Lookup 接口 | ✅ |
| P6.2-a | 密钥生成 | `asshc/infra/keymgr/generate.go` | RSA/Ed25519/ECDSA 生成，OpenSSH 格式，指纹计算 | ✅ |
| P6.2-b | 公钥部署 | `asshc/service/deploy.go` | DeployService 部署公钥到 ~/.ssh/authorized_keys，幂等实现 | ✅ |
| P6.2-c | 密钥备份 | `asshc/infra/keymgr/backup.go` | 指纹命名→复制→data/keys/索引维护 | ✅ |
| P6.2-d | 单元测试 | `asshc/infra/keymgr/*_test.go` | 56 个测试覆盖 Generate/GenerateToPath/Backup/BackupPath/KeyManager | ✅ |
| P6.3 | KeyService | `asshc/service/key.go` | GenerateAndDeploy / HandleKeyFlag 用例编排 | ✅ |
| P6.3-b | 单元测试 | `asshc/service/key_test.go` | Mock KeyManager + Mock ServerRepository (16 个测试) | ✅ |
| P6.4 | keygen 命令 | `cmd/keygen.go` | `assh keygen <server> [--type] [--bits]` 命令，三种模式 + ssh-keygen 参数对齐 | ✅ |
| P6.4-c | add/set -k 增强 | `cmd/server.go` | 空参/stdin/备份逻辑，集成 HandleKeyFlag | ✅ |
| P6.4-d | GenerateToPath 接口+实现+CLI | `asshc/port/keymanager.go` + `asshc/infra/keymgr/generate.go` + `cmd/keygen.go` | `GenerateToPath` 接口在 port、实现 infra/keymgr、`--output`/-O/-f 独立生成模式，新增 `-C` comment/`-N` passphrase 参数和交互式向导 | ✅ |

**P6.4 详细记录**（2026-05-12 ssh-keygen 参数对齐）：

| 任务 | 说明 | 状态 |
|------|------|:----:|
| P6.4-a `-f` 别名 | `--output` 的 `-f` 别名，对齐 ssh-keygen | ✅ |
| P6.4-b `-C`/`--comment` | 密钥注释参数，透传全栈 (port→infra→service→CLI) | ✅ |
| P6.4-c `-N`/`--new-passphrase` | CLI passphrase 优先于 account password | ✅ |
| P6.4-d 无参交互式向导 | `assh keygen` 无参时触发 ssh-keygen 风格交互式向导 | ✅ |
| P6.4-e comment 默认值 | 未提供 `-C` 时默认 "user@host" | ✅ |

**参数规格**（对齐 ssh-keygen）：
```
assh keygen -f <path>         # 独立生成模式（-f = --output = -O）
assh keygen -C "comment"      # 密钥注释
assh keygen -N "passphrase"   # 私钥密码（覆盖 account password）
assh keygen                   # 无参 → 交互式向导
assh keygen -H host -u user -P pass  # 直连模式
```

**接口签名变更**：
```go
// port/keymanager.go
Generate(keyType string, bits int, passphrase []byte, comment string) (privPath string, pubKey []byte, fingerprint string, err error)
GenerateToPath(keyType string, bits int, passphrase []byte, comment string, outputPath string) (pubKey []byte, fingerprint string, err error)

// service/key.go
GenerateAndDeploy(name string, keyType string, bits int, comment string) error
```

**ADR-025 决策记录**：参见 [docs/adrs/025-ssh-keygen-参数对齐.md](adrs/025-ssh-keygen-参数对齐.md)
| P6.5-a | known-servers 接口扩展 | `asshc/port/repository.go` | 新增 KnownServerRecorder 接口 | ✅ |
| P6.5-b | known-servers 数据库迁移 | `asshc/infra/store/db.go` | migrate() 新增 known_servers 建表 | ✅ |
| P6.5-c | known-servers 存储实现 | `asshc/infra/store/known.go` | 新建文件：upsert/lookup 实现 | ✅ |
| P6.5-d | 直连接入点 | `cmd/connect.go` + `cmd/app.go` | 直连成功后 RecordDirectConnect() | ✅ |
| P6.5-e | keygen 关联 known-servers | `cmd/keygen.go` + `asshc/service/deploy.go` | 直连 keygen 后调用 DeployService.DeployToServer + RecordDirectConnect + UpdateKeyBackupPath | ✅ |
| P6.6-a | SSH 备份优先策略 | `asshc/infra/ssh/client.go` | Connector 持有 accountPassphrase + tryBackupKey + data/keys/{hash}/key.pem 备份优先 | ✅ |
| P6.6-b | known-servers 密钥查找 | `asshc/infra/ssh/client.go` + `asshc/service/connect.go` | 直连时查 known_servers 的 key_backup_path + ConnectDirect 新增 keyBackupPath 参数 | ✅ |
| P6.7-a | KeysDir 路径 | `config/path.go` | 新增 KeysDir 常量 | ✅ |
| P6.7-b | DI 注入 — assh | `main.go` | 创建 KeyManager + KeyService，注入 cmd.NewApp() | ✅ |
| P6.7-c | DI 注入 — assh-fs | `cmd/fs/main.go` | assh-fs 无需 keygen，传递 nil | ⭐⭐ (optional) |
| P6.7-d | App 签名变更 | `cmd/app.go` | NewApp 增加 `*service.KeyService` + `port.KeyManager` 参数 | ✅ |
| **P1** | **验证** | `go build ./... && go test ./... -v` | **编译+测试全部通过** | **必选** |

---

### ✅ Phase 6: 密钥管理 — 完成 (2026-05-13)

全部任务完成，验收通过。详见 [modules/key-management.md](modules/key-management.md)。

---

### ✅ Phase 7: 代理 & 隧道 — 完成 (2026-05-13)

全部任务完成，验收通过。

**目标**：实现 SOCKS5 代理、端口转发、SSH 隧道 + **Smart Proxy 智能代理系统**（正向代理 SOCKS5+HTTP、AutoProxy 规则引擎、透明日志、多端口规则）

**依赖**：Phase 3 (SSH连接层)、Phase 4 (CLI命令组装)
**ADR 记录**：[ADR-028: 代理&隧道基础架构](../adrs/028-proxy-tunnel-architecture.md)、[ADR-029: Smart Proxy 智能代理系统](../adrs/029-smart-proxy-system.md)

| 编号 | 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|:----:|
| P7.1 | Proxy 接口定义 | `asshc/port/proxy.go` | Proxy/PortForward/TunnelManager 接口定义 | ✅ |
| P7.2-a | SOCKS5 服务端（基础） | `asshc/infra/proxy/socks5.go` | SOCKS5 协议实现（RFC 1928 NO AUTH CONNECT） | ✅ |
| P7.2-a-2 | SOCKS5 用户名密码认证 | `asshc/infra/proxy/socks5.go` | RFC 1929 USERNAME/PASSWORD 认证扩展 | ✅ |
| P7.2-b | proxy CLI 命令 | `cmd/proxy.go` | Smart Proxy + 直连 + rule/log 子命令 | ✅ |
| P7.3-a | 本地端口转发 | `asshc/infra/proxy/port.go` | -L 本地转发（ssh -L 等价） | ✅ |
| P7.3-b | 远程端口转发 | `asshc/infra/proxy/port.go` | -R 远程转发（ssh -R 等价） | ✅ |
| P7.4 | SSH 隧道管理 | `asshc/infra/proxy/tunnel_manager.go` | TunnelManager（重连+daemon）| ✅ |
| P7.5 | 代理服务编排 | `asshc/service/proxy.go` | ProxyService 编排层 | ✅ |
| P7.6 | tunnel CLI 命令 | `cmd/tunnel.go` | tunnel start/stop/list | ✅ |
| P7.7 | 单元测试 | `*_test.go` | 46 tests（rules/logger/listener/ports） | ✅ |
| P7.8 | DI 注入 | `main.go` + `cmd/app.go` | ProxyService 创建+注入 | ✅ |
| P7.9-a | AutoProxy 规则引擎 | `asshc/port/rules.go` + `asshc/infra/proxy/rules.go` | GFWList 兼容规则解析匹配 | ✅ |
| P7.9-b | HTTP CONNECT 代理 | `asshc/infra/proxy/http_connect.go` | RFC 7231 CONNECT 处理器 | ✅ |
| P7.9-c | 透明日志系统 | `asshc/port/logger.go` + `asshc/infra/proxy/logger.go` | JSONL 格式日志 | ✅ |
| P7.9-d | --ports 规则解析 | `asshc/service/proxy_ports.go` | 范围展开/截断/多对一/一对多 | ✅ |
| P7.9-e | 统一集成层 | `asshc/infra/proxy/listener.go` | Smart Proxy 流水线编排 | ✅ |
| **P1** | **验证** | `go build ./... && go test ./...` | 编译+测试全部通过 | ✅ |

**详细设计**：见 [modules/proxy-tunnel.md](modules/proxy-tunnel.md)

**ADR 记录**：[ADR-028: 代理&隧道基础架构](../adrs/028-proxy-tunnel-architecture.md)、[ADR-029: Smart Proxy 智能代理系统](../adrs/029-smart-proxy-system.md)（已接受）

#### 参数规格

```
# SOCKS5 代理（正向模式）
assh proxy myserver                          # SOCKS5 :1080（默认）
assh proxy myserver --socks5 :1080           # 显式指定 SOCKS5 地址
assh proxy myserver --http :8080             # 叠加 HTTP CONNECT
assh proxy myserver --auth user:pass         # SOCKS5 用户名认证
assh proxy myserver --daemon                 # 后台守护
assh proxy myserver --auto-reconnect=5/10s   # 自动重连
assh proxy -H host -u root -P pass           # 直连模式

# AutoProxy 规则 + 透明日志
assh proxy myserver --autoproxy ./list.txt   # 加载规则文件（智能路由）
assh proxy myserver --log-dir ./proxy-logs   # 启用透明日志

# 反向代理模式（--ports 仅在反向中有效）
assh proxy myserver --reverse                # 反向代理模式
assh proxy myserver --reverse --tcp          # TCP 协议（默认）
assh proxy myserver --reverse --http         # HTTP 协议

assh proxy myserver --reverse \
  --ports 80:3000                            # server:80 → local:3000
assh proxy myserver --reverse \
  --ports 80,443:80                          # server:80,443 → local:80
assh proxy myserver --reverse \
  --ports 443:80,443                         # server:443 → local:80,443
assh proxy myserver --reverse \
  --ports 80-85:81-86                        # 范围展开 1:1
assh proxy myserver --reverse \
  --ports 80-88:81-84                        # 超出范围的指向最后一个

assh proxy myserver --reverse --tcp \
  --ports 80,443:80 --auth user:pass         # TCP + SOCKS5 认证
assh proxy myserver --reverse --http \
  --ports 8080:3000 --auth user:pass         # HTTP + Basic Auth

assh proxy myserver --reverse --tcp \
  --ports 80,443:80 --auth user:pass \
  --autoproxy ./list.txt --log-dir ./logs    # 反向+认证+规则+日志

# 规则/日志管理
assh proxy rule reload                       # 热重载规则
assh proxy rule status                       # 查看规则状态
assh proxy log <id>                          # 查看会话日志

# 端口转发隧道
assh tunnel start myserver -L 8080:localhost:80        # 本地转发
assh tunnel start myserver -R 8080:localhost:3000      # 远程转发
assh tunnel start myserver -L 8080:svc1:80 -L 9090:svc2:90  # 多转发
assh tunnel start myserver -L 8080:web:80 --daemon     # 后台守护
assh tunnel start myserver -L 8080:web:80 --auto-reconnect=3/5s  # 自动重连
assh tunnel stop <id>                        # 停止隧道
assh tunnel list                             # 列出活跃隧道
```

#### 端口转发规格

格式：`[bind_addr:]port:host:hostport`

| 写法 | bind_addr | port | host | hostport |
|------|-----------|------|------|----------|
| `8080:localhost:80` | 127.0.0.1 | 8080 | localhost | 80 |
| `0.0.0.0:8080:web:80` | 0.0.0.0 | 8080 | web | 80 |
| `127.0.0.1:8443:svc:443` | 127.0.0.1 | 8443 | svc | 443 |

### ✅ Phase 8: 云同步 (Qiniu) — 完成 (2026-05-16)

目标：实现七牛云数据备份同步，支持服务器配置的云端推送与拉取

依赖：Phase 2 (storage) + Phase 4 (CLI)
设计文档：`docs/worklogs/2026-05-16-Phase8-Design.md`

**设计决策**：
- 数据范围：仅服务器配置（不包含密钥/密码原文）
- 账户管理：单账户模式
- 同步策略：增量同步为主（版本号比较）
- 冲突处理：保留双方，云端版本添加 `_cloud_<ts>` 后缀

| 编号 | 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|:----:|
| P8.1 | Syncer 接口 | `asshc/port/syncer.go` | Syncer 接口 + DTOs (SyncAccount/SyncData/ServerData/SyncResult/SyncObject/SyncHistory) | ✅ |
| P8.1a | 数据库迁移 | `asshc/infra/store/db.go` | `cloud_accounts` + `sync_history` 表 | ✅ |
| P8.1b | 账户存储 | `asshc/infra/store/account.go` | GetAccount/SetAccount/DeleteAccount/TestAccount + 加密存储 | ✅ |
| P8.2 | Qiniu 同步实现 | `asshc/infra/sync/qiniu/qiniu.go` | `SyncObject` 实现 port.Syncer (Login/Push/Pull/List/Delete) | ✅ |
| P8.3 | 同步服务 | `asshc/service/sync.go` | `SyncService` + `SyncStore` 接口 + Push/Pull 合并逻辑 | ✅ |
| P8.4 | 同步命令 | `cmd/sync.go` | sync account set/show/delete/test + push/pull/history | ✅ |
| P8.5 | 清理旧代码 | 移除 `asshv2/lib/qiniu/` | 已删除 legacy 代码 | ✅ |
| P8.6 | DI 注入 | `main.go` + `cmd/app.go` | Syncer + SyncService 创建+注入 | ✅ |
| **验证** | `go build ./... && go test ./... -v` | | **编译+测试全部通过** | ✅ |

**同步命令使用**：
```bash
# 配置云账户（交互式）
assh sync account set

# 配置云账户（命令行）
assh sync account set --access-key <AK> --secret-key <SK> --bucket <bucket> --zone huadong

# 查看账户信息
assh sync account show

# 测试连接
assh sync account test

# 推送本地配置到云端
assh sync push

# 从云端拉取配置
assh sync pull

# 查看同步历史
assh sync history [-n 10]
```

### 🔄 Phase 9: 服务器健康检查 — 进行中 (2026-05-17)

目标：实现服务器批量健康检查和单一检查功能

依赖：Phase 3 (ssh) + Phase 4 (CLI)

| 编号 | 任务 | 文件 | 说明 | 状态 |
|------|------|------|------|:----:|
| P9.1 | 健康检查接口 | `asshc/port/health.go` | HealthChecker 接口定义 | ⬜ |
| P9.2 | SSH 健康检查 | `asshc/infra/ssh/health.go` | 连接检测、延迟检测实现 | ⬜ |
| P9.3 | 健康检查服务 | `asshc/service/health.go` | HealthService 编排层 | ⬜ |
| P9.4 | 检查命令 | `cmd/health.go` | health check/list 命令 | ⬜ |
| P9.5 | 列表检查 | — | 批量检查服务器组，支持并发 | ⬜ |
| P9.6 | 单一检查 | — | 指定单个服务器检查并输出详细结果 | ⬜ |
| **验证** | `go build ./... && go test ./asshc/... -v` | | **编译+测试全部通过** | ⬜ |

### 📝 Phase 10: 跳板机/堡垒机支持

目标：实现 SSH 跳板机（Jump Server/Bastion）配置和连接

依赖：Phase 3 (ssh)

| 任务 | 说明 |
|------|------|
| P10.1 跳板机配置 | Server 结构增加 `JumpHost`/`Bastion` 字段 |
| P10.2 跳板机连接 | `ConnectService` 支持多跳连接（ssh -A 转发） |
| P10.3 跳板机命令 | `assh jump` 或在 server 配置中指定 jump host |
| P10.4 连接穿透 | 通过跳板机访问内网服务器 |
| **验证**: `go test ./asshc/... -v` |

### 📝 Phase 11: 远程服务器挂载

目标：使用 sshfs/webdav/samba 挂载远程服务器文件系统到本地

依赖： Phase 3 (ssh)

| 任务 | 说明 |
|------|------|
| P11.1 挂载接口 | `asshc/port/mounter.go` — FileMounter 接口 |
| P11.2 SSHFS 挂载 | `asshc/infra/mount/sshfs.go` — sshfs 封装 |
| P11.3 WebDAV 挂载 | `asshc/infra/mount/webdav.go` — WebDAV 客户端 |
| P11.4 SMB 挂载 | `asshc/infra/mount/smb.go` — SMB/CIFS 客户端 |
| P11.5 挂载服务 | `asshc/service/mount.go` — MountService |
| P11.6 挂载命令 | `cmd/mount.go` — mount/unmount/list 命令 |
| **验证**: `go test ./asshc/... -v` |

### 📝 Phase 12: 自动更新/升级

目标：实现版本检查、手动/自动升级、强制更新

依赖： Phase 4 (CLI)

| 任务 | 说明 |
|------|------|
| P12.1 版本检查接口 | `asshc/port/updater.go` — VersionChecker 接口 |
| P12.2 更新检查命令 | `assh version --check` 或 `assh upgrade --check` |
| P12.3 升级命令 | `assh upgrade` — 下载并安装新版本 |
| P12.4 强制更新 | `assh upgrade --force` — 强制更新到指定版本 |
| P12.5 更新源配置 | 支持自定义更新源（URL/GitHub Release） |
| P12.6 回滚支持 | 升级失败时回滚到上一版本 |
| **验证**: `go test ./asshc/... -v` |

### 📝 Phase 13: 远程命令执行增强

目标：增强 `run` 和 `bc` 命令，添加超时、输出格式化、stdout/stderr 分离等功能

依赖： Phase 3 (SSH) + Phase 4 (CLI)

**现状分析：**

| 功能 | run | bc | 状态 |
|------|-----|-----|------|
| 单服务器命令执行 | ✅ | - | 已有 |
| 多服务器并发执行 | - | ✅ | 已有 |
| 命令超时 | ❌ | ❌ | 缺失 |
| stdout/stderr 分离 | ❌ | ❌ | 缺失 |
| 输出格式化 (JSON/Table) | ❌ | ✅ JSON | 部分 |
| 管道支持 (stdin) | ❌ | ❌ | 缺失 |
| 变量替换 | ❌ | ❌ | 缺失 |
| 异步/后台任务 | ❌ | ❌ | 缺失 |
| 命令历史 | ❌ | ❌ | 缺失 |

| 编号 | 任务 | 文件 | 说明 | 优先级 |
|------|------|------|------|--------|
| P13.1 | 命令超时 | `asshc/port/session.go` + `asshc/infra/ssh/session.go` | 新增 `RunWithTimeout` / `RunWithOutputTimeout` 方法 | ⏳ |
| P13.2 | stdout/stderr 分离 | `asshc/service/connect.go` | 新增 `RunWithStderr` 方法，返回 stdout + stderr | ⏳ |
| P13.3 | 输出格式化 | `cmd/connect.go` | run/bc 添加 `--format json/table/plain` 参数 | ⏳ |
| P13.4 | run 超时/格式化 | `cmd/connect.go` | run 命令添加 `--timeout` / `--format` 支持 | ⏳ |
| P13.5 | bc 超时/格式化 | `cmd/connect.go` | bc 命令添加 `--timeout` / `--format` 支持 | ⏳ |
| P13.6 | 变量替换 | `cmd/connect.go` | 支持 `{{.Host}}` / `{{.User}}` / `{{.Name}}` 替换 | ⏳ |
| **验证** | `go test ./asshc/... -v` | | | |

**接口变更：**
```go
// port/session.go
type SSHSession interface {
    // 现有方法
    Shell(client *ssh.Client) error
    Run(client *ssh.Client, cmd string) error
    RunWithOutput(client *ssh.Client, cmd string) (string, error)

    // P13.1 新增
    RunWithTimeout(client *ssh.Client, cmd string, timeout time.Duration) error
    RunWithOutputTimeout(client *ssh.Client, cmd string, timeout time.Duration) (string, error)

    // P13.2 新增
    RunWithStderr(client *ssh.Client, cmd string) (stdout, stderr string, err error)
}
```

### 📝 实验阶段: 虚拟命令投放与执行测试

目标：实验性功能，测试虚拟命令（模拟命令）的投放和执行机制

依赖： Phase 4 (CLI) + 实验性设计

> 本阶段为探索性实验，用于验证概念和用户需求，不保证最终实现。

| 任务 | 说明 |
|------|------|
| X.1 虚拟命令定义 | 定义什么是虚拟命令（不实际执行，仅记录） |
| X.2 命令投放系统 | 通过配置或远程下发虚拟命令到客户端 |
| X.3 命令执行跟踪 | 记录虚拟命令的执行状态（pending/running/completed） |
| X.4 结果收集 | 收集虚拟命令的执行结果并上报 |
| X.5 实验命令 | `cmd/experiment.go` — 投放、执行、结果查看 |
| **验证**: 手动测试 |

## 依赖顺序

```
Phase 0 (骨架)
  └── Phase 1 (基础设施)
        └── Phase 2 (数据持久化)
              └── Phase 3 (SSH 连接)
                    ├── Phase 4 (CLI 组装) ← MVP
                    ├── Phase 5 (SFTP 传输)
                    ├── Phase 6 (密钥管理)
                    ├── Phase 7 (代理/隧道)
                    ├── Phase 8 (云同步)
                    ├── Phase 9 (健康检查)
                    ├── Phase 10 (跳板机)
                    ├── Phase 11 (远程挂载)
                    ├── Phase 12 (自动更新)
                    └── Phase 13 (命令执行增强)
                          └── 实验阶段 (虚拟命令)
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
