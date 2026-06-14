# Phase 4: CLI 命令组装设计方案

> 日期: 2026-05-09
> 状态: 已更新 (2026-05-09 新增 set/rollback/changelog)
> ADR: ADR-012 (CLI 参数对齐与命令架构), ADR-013 (配置版本管理与回滚)

---

## 1. 术语表

| 术语 | 英文 | 定义 | 示例 |
|------|------|------|------|
| **服务器名** | Server Name | 格式 `[group.]name`，首点号前为组名，后为服务器名 | `prod.web-01` → group=`prod`, name=`web-01` |
| **直连** | Direct Connect | 不通过数据库，直接使用 `-H/--host` `-u/--user` 等参数建立 SSH 连接 | `assh root@10.0.0.1` |
| **按名登录** | Named Login | 通过服务器名从数据库获取服务器信息进行 SSH 连接 | `assh prod.web-01` |
| **远程执行** | Remote Exec | 在已建立的 SSH 连接上执行单条命令 | `assh run prod.web-01 "uptime"` |
| **批量执行** | Batch Exec | 在多个服务器上并行执行命令 | `assh bc "uptime" --servers s1,s2` |
| **Options** | Options | 服务器或 SSH 的扩展配置项，通过 `-o key=value` 传递 | `-o keepalive=30` |
| **全局参数** | Global Flag | 适用于所有命令的顶层参数 | `-v/--verbose`, `-F/--config` |

---

## 2. 命令结构

### 2.1 命令树

```
assh [global flags] <command> [command flags] [args]

global flags:
  -v, --verbose        详细输出
  -q, --quiet          静默模式
  -F, --config string  配置文件路径（默认: ~/.assh/v2/assh.yml）

commands:
  server               服务器管理（子命令集）
    add                添加服务器
    set                创建/更新服务器（upsert, 含参数验证）
    ls                 列出服务器
    info               查看服务器详情
    rm                 删除服务器
    mv                 重命名/移动服务器
    rollback           回滚服务器配置（--list 查看历史, <version> 回滚）

  login [flags]         SSH 登录（自动识别按名/直连）
  run [flags] <name> <command>   远程执行命令
  bc [flags] <command>           批量执行命令
  version              版本信息
```

### 2.2 命令详情

#### server add — 添加服务器

```
assh server add <name> [flags]

位置参数:
  name                  服务器名（格式: [group.]name）

Flags:
  -H, --host string     主机地址（必填）
  -p, --port int        端口（默认: 22）
  -u, --user string     用户名（默认: root）
  -l, --login string    用户名（同 --user）
  -P, --password string 密码（明文传递，不提供则交互提示）
  -i, --identity-file string  密钥文件路径
  -k, --key string      密钥文件路径（同 --identity-file）
  -o, --option strings  Options（可多次使用）
  --remark string       备注信息
  --group string        分组（可覆盖 name 中的分组）

示例:
  assh server add prod.web-01 -H 10.0.0.1
  assh server add mybox -H 192.168.1.100 -p 2222 -u admin -P secret
  assh server add db-master -H 10.0.0.2 -i ~/.ssh/id_rsa -o keepalive=30
```

#### server ls — 列表服务器

```
assh server ls [flags]

Flags:
  --group string    按分组筛选
  --search string   按关键词搜索（名称/主机/备注）

示例:
  assh server ls
  assh server ls --group prod
  assh server ls --search web

输出格式:
  Group     Name         Host         Port  User   Ver   Auth
  prod      web-01       10.0.0.1     22    root   3     key
  prod      web-02       10.0.0.2     22    root   2     password
  dev       db-01        10.0.1.1     22    dev    5     key
```

#### server info — 查看服务器详情

```
assh server info <name>

位置参数:
  name          服务器名

示例:
  assh server info prod.web-01

输出格式（多行详情）:
  Name:     prod.web-01
  Host:     10.0.0.1
  Port:     22
  User:     root
  Auth:     key (/home/user/.ssh/id_rsa)
  Remark:   Production web server
  Options:  keepalive=30
  Version:  3
```

#### server rm — 删除服务器

```
assh server rm <name>

位置参数:
  name          服务器名

示例:
  assh server rm prod.web-01
```

#### server mv — 重命名/移动服务器

```
assh server mv <from> <to>

位置参数:
  from          原服务器名
  to            新服务器名

示例:
  assh server mv prod.web-01 prod.web-01-old   # 重命名
  assh server mv prod.web-01 dev.web-01        # 移动分组
```

#### login — SSH 登录（自动识别）

```
assh login [target] [flags]

参数自动识别规则:
  1. 包含 "@" → 解析为 user@host 直连
  2. 包含 "-H/--host" → 分离参数直连
  3. 纯名称 → 从数据库按名查找

位置参数:
  target        服务器名 或 user@host（可选，与 --host 互斥）

Flags:
  -H, --host string            主机地址（与 target 互斥）
  -p, --port int               端口（默认: 22）
  -u, --user string            用户名（默认: root）
  -l, --login string           用户名（同 --user）
  -P, --password string        密码
  -i, --identity-file string   密钥文件
  -k, --key string             密钥文件（同 --identity-file）
  -c, --connect-timeout int    连接超时秒数
  -o, --option strings         Options

示例:
  assh login prod.web-01                       # 按名
  assh login root@10.0.0.1 -p 2222             # user@host 直连
  assh login -H 10.0.0.1 -u admin -P secret    # 分离参数直连
```

#### run — 远程执行命令

```
assh run <name> <command> [flags]

位置参数:
  name          服务器名
  command       要执行的命令

Flags: （同 login 的连接参数，用于覆盖数据库中的连接配置）
  -p, --port int               端口
  -u, --user string            用户名
  -l, --login string           用户名（同 --user）
  -P, --password string        密码
  -i, --identity-file string   密钥文件
  -k, --key string             密钥文件

示例:
  assh run prod.web-01 "uptime"
  assh run prod.web-01 "df -h" -u admin
```

#### bc — 批量执行命令

```
assh bc <command> [flags]

位置参数:
  command       要执行的命令

Flags:
  --servers string     服务器列表（逗号分隔）
  --group string       按分组执行
  -p, --port int        端口（覆盖默认）
  -u, --user string     用户名（覆盖默认）
  -i, --identity-file string  密钥文件
  -l, --login string    用户名（同 --user）
  --log string          输出日志路径（默认: ./bc-result-{timestamp}.json）

示例:
  assh bc "uptime" --servers prod.web-01,prod.web-02
  assh bc "df -h" --group prod
  assh bc "systemctl status nginx" --servers web-01,web-02,db-01 --log ./nginx-status.json
```



## 3. bc 三通道输出架构

### 3.1 架构图

```
bc "uptime" --servers prod.web-01,prod.web-02
         │
         ├─ goroutine 1 ── prod.web-01 ──┐
         │                                ├── 通道1: 实时日志
         │                                │    /tmp/assh/bclogs/prod.web-01.log
         │                                │    /tmp/assh/bclogs/prod.web-02.log
         │                                │
         ├─ goroutine 2 ── prod.web-02 ──┼── 通道2: stdout 实时
         │                                │    [prod.web-01] 10:30:45 up 30 days
         │                                │    [prod.web-02] 10:30:46 up 15 days
         │                                │
         │                                └── 通道3: JSON 汇总（全部完成后）
         │                                     ./bc-result-20260509T103000.json
         │                                     {
         │                                       "command": "uptime",
         │                                       "servers": [...],
         │                                       "summary": {...}
         │                                     }
```

### 3.2 JSON 汇总格式

```json
{
  "command": "uptime",
  "timestamp": "2026-05-09T10:30:00+08:00",
  "total": 2,
  "success": 2,
  "failed": 0,
  "duration_seconds": 1.23,
  "servers": [
    {
      "name": "prod.web-01",
      "host": "10.0.0.1",
      "port": 22,
      "user": "root",
      "exit_code": 0,
      "stdout": "10:30:45 up 30 days, 1:23, 0 users, load average: 0.00, 0.01, 0.05",
      "stderr": "",
      "duration_seconds": 0.45,
      "error": ""
    },
    {
      "name": "prod.web-02",
      "host": "10.0.0.2",
      "port": 22,
      "user": "root",
      "exit_code": 0,
      "stdout": "10:30:46 up 15 days, 5:01, 0 users, load average: 0.10, 0.08, 0.05",
      "stderr": "",
      "duration_seconds": 0.52,
      "error": ""
    }
  ]
}
```

---

## 4. DI 注入设计

### 4.1 依赖图

```go
main.go
  │
  ├── config.GetDbPath() → expandPath → dbPath
  │
  ├── store.NewStore(dbPath) → *store.Store (implements port.ServerRepository)
  │
  ├── sshinfra.NewConnector() → *ssh.Client (implements port.SSHConnector)
  ├── sshinfra.NewSession() → *ssh.Session (implements port.SSHSession)
  │
  ├── service.NewServerService(repo) → *service.ServerService
  ├── service.NewConnectService(connector, session, repo) → *service.ConnectService
  │
  └── cmd.NewApp(version, build, connectSvc, serverSvc) → *cmd.App
```

### 4.2 App 结构体变更

```go
// cmd/app.go — 当前
type App struct {
    cli        *cli.App
    version    string
    build      string
    connectSvc *service.ConnectService
}

// cmd/app.go — 变更后
type App struct {
    cli         *cli.App
    version     string
    build       string
    connectSvc  *service.ConnectService
    serverSvc   *service.ServerService
}
```

---

## 5. 文件清单与任务分解

### P4.1 `cmd/server.go` — 服务器管理命令（新建）

| 子任务 | 说明 | 测试 |
|--------|------|------|
| P4.1-a add 子命令 | 解析 name + flags → `serverSvc.AddServer()` | 添加成功、重复、参数缺失 |
| P4.1-b ls 子命令 | 列出/筛选 → `serverSvc.ListServers()` / `GetGroup()` / `SearchServers()` | 全部列表、分组筛选、搜索 |
| P4.1-c info 子命令 | 查看详情 → `serverSvc.GetServer()` | 存在、不存在 |
| P4.1-d rm 子命令 | 删除 → `serverSvc.RemoveServer()` | 存在、不存在 |
| P4.1-e mv 子命令 | 重命名 → `serverSvc.MoveServer()` | 成功、冲突、不存在 |
| P4.1-f 注册到 app | `registerServerCommands()` 关联到 `app.Commands` | help 显示 |
| P4.1-g set 子命令 | `assh server set <name> [flags]` → upsert (`SetServer`) | 创建新服务器、更新已存在、参数验证 |
| P4.1-h rollback 子命令 | `assh server rollback <name> <version>` → `RollbackServer()` | 版本回滚、--list 查看历史 |
| P4.1-i 版本管理 | `asshc/infra/store/` Set 自动版本递增 + 快照 + changelog | 版本连续递增、rollback 正确记录 |

### P4.2 `cmd/connect.go` — 连接管理命令（新建）

| 子任务 | 说明 | 测试 |
|--------|------|------|
| P4.2-a login 命令 | 参数自动识别 → `connectSvc.ConnectByName()` / `ConnectDirect()` → `Shell()` | 按名、直连、user@host、参数缺失 |
| P4.2-b run 命令 | 按名连接 → `run` → 关闭 | 正常执行、命令为空、server 不存在 |
| P4.2-c bc 命令 | 批量执行 + 三通道输出（goroutine 并发） | 2 台服务器、空结果、全部失败 |
| P4.2-d 注册到 app | `registerConnectCommands()` 关联子命令 | help 显示 |

### P4.3 `cmd/app.go` — 命令注册调整（修改）

| 子任务 | 说明 |
|--------|------|
| P4.3-a `NewApp` 签名变更 | 接收 `*service.ServerService` |
| P4.3-b 全局参数注册 | `-v`, `-q`, `-F/--config` |
| P4.3-c 子命令注册 | 调用 `registerServerCommands()` + `registerConnectCommands()` |
| P4.3-d `beforeAction` 完善 | 数据库路径初始化、日志初始化 |

### P4.4 `main.go` — DI 注入（修改）

| 子任务 | 说明 |
|--------|------|
| P4.4-a Store 初始化 | `store.NewStore(dbPath)`，错误处理 |
| P4.4-b ServerService 注入 | `service.NewServerService(repo)` |
| P4.4-c ConnectService repo 补全 | 将 `nil` 替换为真实 repo |
| P4.4-d App 构建 | 传递 `serverSvc` 给 `cmd.NewApp()` |

### P4.5 验证

| 验证项 | 命令 |
|--------|------|
| 编译 | `cd asshv2 && go build ./...` |
| 静态检查 | `cd asshv2 && go vet ./...` |
| 单元测试 | `cd asshv2 && go test ./... -count=1` |
| CLI 可用 | `cd asshv2 && go run . help` |
| CLI 版本 | `cd asshv2 && go run . version` |

---

## 6. 验收标准

### 6.1 功能验收

| # | 场景 | 操作 | 预期结果 |
|---|------|------|----------|
| 1 | 添加服务器 | `assh server add test-box -H 127.0.0.1 -P test` | 添加成功，可查看到 |
| 2 | 列出服务器 | `assh server ls` | 列表显示分组和服务器 |
| 3 | 查看详情 | `assh server info test-box` | 显示完整服务器信息 |
| 4 | 删除服务器 | `assh server rm test-box` | 删除成功，列表不显示 |
| 5 | 移动服务器 | `assh server mv test-box new.test-box` | 名称变更，信息保留 |
| 6 | 按名登录 | `assh login test-box`（直连 mock） | 连接建立 |
| 7 | user@host | `assh login root@10.0.0.1`（直连 mock） | 自动解析 |
| 8 | 远程执行 | `assh run test-box "echo ok"` | 输出 "ok" |
| 9 | 批量执行 | `assh bc "echo ok" --servers s1,s2` | 每台输出标头 + 结果 |
| 10 | 帮助信息 | `assh help` | 显示所有命令和全局参数 |

### 6.2 边界验收

| # | 场景 | 操作 | 预期结果 |
|---|------|------|----------|
| 11 | 空名添加 | `assh server add "" -H 127.0.0.1` | 错误提示 |
| 12 | 缺少 host | `assh server add test-box` | 错误提示 |
| 13 | 重复添加 | 重复添加同名服务器 | 错误提示 |
| 14 | 删除不存在 | `assh server rm not-exists` | 错误提示 |
| 15 | login 空参数 | `assh login` | 错误提示 |
| 16 | run 空命令 | `assh run test-box ""` | 错误提示 |
| 17 | bc 无服务器 | `assh bc "echo ok"` | 错误提示（--servers/--group 至少一个） |
| 18 | 配置文件路径 | `assh -F /tmp/custom.yml server ls` | 使用自定义配置 |
| 19 | 详细模式 | `assh -v server ls` | 输出调试信息 |
| 20 | 静默模式 | `assh -q server ls` | 最小化输出 |

---

## 7. 架构约束检查

| 约束 | 合规 | 说明 |
|------|------|------|
| `cmd/` 不直接依赖 `infra/` | ✅ | 通过 `service/` 间接调用 |
| `cmd/` 只调用 `service/` 方法 | ✅ | 不直接操作数据库或 SSH |
| `main.go` 是唯一 DI 组合根 | ✅ | 所有依赖注入在 main 中完成 |
| 参数与 ssh 对齐 | ✅ | 参见 ADR-012 参数映射表 |
| urfave/cli 框架 | ✅ | 保持现有框架不变 |

---

## 8. set/rollback 命令（Phase4 新增）

### 8.1 set — 创建/更新服务器（upsert）

```
assh server set <name> [flags]

行为（upsert）:
  - 服务器不存在 → 使用提供的参数创建新服务器
  - 服务器已存在 → 仅修改显式指定的字段
  - 每次修改自动记录快照到 server_changelog 表，版本递增

位置参数:
  name                  服务器名

Flags:
  -H, --host string     主机地址（新建必填）
  -p, --port int        端口（校验范围 1-65535）
  -u, --user string     用户名
  -l, --login string    用户名（同 --user）
  -P, --password string 密码
  -i, --identity-file string  密钥文件路径
  -k, --key string      密钥文件路径（同 --identity-file）
  -o, --option strings  添加/替换 Options
  --remark string       备注信息
  --clear-password      清除密码
  --clear-key           清除密钥文件

示例:
  assh server set prod.web-01 -H 10.0.0.1 -u admin -P secret  # 新建
  assh server set prod.web-01 -p 2222 --remark "updated"       # 更新
  assh server set prod.web-01 --clear-password                 # 清除密码
```

### 8.2 rollback — 回滚服务器配置

```
assh server rollback <name> <version>
assh server rollback <name> --list

行为:
  - 从 server_changelog 中读取指定版本的快照（JSON）
  - 恢复服务器配置到该版本状态
  - 生成新的 rollback 类型 changelog 条目（不覆盖历史）

位置参数:
  name          服务器名
  version       目标版本号（与 --list 配合使用）

Flags:
  --list        显示该服务器的所有变更版本

示例:
  assh server rollback prod.web-01 --list    # 查看历史
  assh server rollback prod.web-01 2         # 回滚到 v2

Changelog 输出:
  Version  Type         Date
  -------- ------------ -------------------
  1        create       2026-05-09 08:11:29
  2        update       2026-05-09 08:11:30
  3        update       2026-05-09 08:11:30
  4        rollback     2026-05-09 08:11:43
```

### 8.3 数据存储

新增 `server_changelog` 表记录完整配置快照：

```sql
CREATE TABLE server_changelog (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    group_name  TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL,
    change_type TEXT NOT NULL,         -- 'create', 'update', 'rollback'
    snapshot    TEXT NOT NULL,         -- JSON 快照
    created_at  TEXT DEFAULT (datetime('now'))
);
```

`servers` 表新增 `version` 列：

```sql
ALTER TABLE servers ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
```

详细架构决策见 [ADR-013](docs/adrs/013-配置版本管理与回滚.md)。
