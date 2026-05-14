# ASSH - An SSH Client

`ASSH` 是一个使用 Go 开发的 SSH 客户端，适用于服务器管理、维护和快速使用。
采用 **port / service / infra** 三层架构，基于 SQLite 加密存储服务器配置，支持密码和密钥认证。

当前版本：**v2.0.0**（Phase 7 完成）

---

## 特性一览

| 特性 | 说明 |
|------|------|
| 🔐 **服务器管理** | 增删改查、分组、命名规则 (group.server) |
| 🔑 **多认证方式** | SSH Agent / 私钥 / 密码，自动回退 |
| 🗄️ **加密存储** | AES-256-GCM + SQLite，静态密钥自动管理 |
| 📋 **版本回滚** | 每次配置变更自动快照，支持回滚到任意版本 |
| 📂 **文件传输** | push/pull、递归目录、断点续传、进度显示、SHA256 校验 |
| 🚀 **密钥管理** | RSA/Ed25519/ECDSA 密钥生成、部署、备份 |
| 🔄 **代理 & 隧道** | SOCKS5/HTTP CONNECT 代理、AutoProxy 规则引擎、端口转发 |
| ⚡ **批量执行** | 多服务器并发命令执行、三通道输出 |
| 🔌 **直连模式** | 不依赖已保存配置，直接指定 host/user/password 连接 |
| 🔧 **双二进制** | `assh`（连接管理）+ `assh-fs`（文件操作）各自独立编译 |

---

## 安装

### 编译安装

```bash
cd asshv2 && sh build.sh
```

编译产物在 `asshv2/build/` 目录，包含 `assh` 和 `assh-fs` 两个二进制文件。

### 单平台构建

```bash
cd asshv2 && sh build.sh <os> <arch> <alias>
# 示例：sh build.sh linux amd64 linux
```

---

## 快速开始

### 添加服务器

```bash
# 添加新服务器
assh server add myserver -H 192.168.1.100 -u root -P mypassword -p 22

# 使用分组名（命名规则 group.server）
assh server add prod.web -H 10.0.0.1 -u admin -P secret

# 使用密钥认证
assh server add myserver -H example.com -u deploy -i ~/.ssh/id_rsa

# 设置 Keepalive 心跳
assh server set myserver -o keepalive=30

# 带备注添加
assh server add dev.node -H 10.0.0.2 -u dev -P devpass --remark "开发环境"
```

### 登录服务器

```bash
# 通过已保存的名称登录
assh login myserver
assh login prod.web

# 直接指定主机和用户（自动识别 user@host）
assh login root@192.168.1.100

# 使用 -H 参数分离登录
assh login -H 192.168.1.100 -u root -P password

# 快捷方式：直接输入服务器名即可登录
assh myserver
```

### 执行远程命令

```bash
# 在单台服务器上执行
assh run myserver "uptime"
assh run myserver "df -h | grep sda"

# 批量执行（多服务器并发 + 三通道输出）
assh bc "systemctl status nginx" --servers web1,web2,web3

# 按分组批量执行
assh bc "free -m" --group prod

# 自定义输出路径
assh bc "date" --servers s1,s2,s3 --log ./result.json
```

---

## 服务器管理

### 服务器 CRUD

```bash
# 添加服务器
assh server add <name> -H <host> [-p port] [-u user] [-P password] [-i keyfile] [--group group] [--remark remark]

# 创建或更新服务器（upsert）
assh server set <name> [-H host] [-p port] [-u user] [-P password] [-i keyfile] [-o option] [--remark remark]
assh server set <name> --clear-password   # 清除密码
assh server set <name> --clear-key        # 清除密钥文件

# 列出服务器
assh server ls                    # 全部列表
assh server ls --group prod       # 按分组筛选
assh server ls --search example   # 关键词搜索

# 查看服务器详情
assh server info myserver

# 删除服务器
assh server rm myserver

# 重命名/移动服务器
assh server mv myserver prod.web
```

### 版本回滚

每次服务器配置变更自动生成快照，支持回滚：

```bash
# 查看变更历史
assh server rollback myserver --list

# 回滚到指定版本
assh server rollback myserver 2
```

---

## 文件传输（SFTP）

提供独立的 `assh-fs` 二进制，支持文件推送和拉取，包含递归传输、断点续传、进度显示。

### 推送文件

```bash
# 推送单文件
assh-fs push myserver ./local/file.txt /remote/path/

# 递归推送目录
assh-fs push -r myserver ./local/dir/ /remote/dir/

# 断点续传
assh-fs push --resume myserver ./large.tar.gz /remote/
assh-fs push -e myserver ./large.iso /remote/     # 短参

# 强制覆盖
assh-fs push -f myserver ./file.txt /remote/

# 跳过已存在
assh-fs push --skip myserver ./file.txt /remote/

# Glob 匹配推送（平铺到目标目录）
assh-fs push myserver "*.log" /tmp/logs/

# 传输后 SHA256 校验
assh-fs push --checksum myserver ./important.iso /remote/

# 直连模式推送
assh-fs push -H 192.168.1.1 -u root -P mypass ./file /remote/
```

### 拉取文件

```bash
# 拉取单文件
assh-fs pull myserver /remote/file.txt ./local/

# 递归拉取目录
assh-fs pull -r myserver /remote/dir/ ./local/

# 断点续传拉取
assh-fs pull --resume myserver /remote/large.tar.gz ./

# Glob 匹配拉取（平铺到本地目录）
assh-fs pull myserver "/var/log/*.log" ./logs/

# 直连模式拉取
assh-fs pull -H 192.168.1.1 -u root -P mypass /remote/file ./
```

### 交互式 SFTP 会话

```bash
# 进入交互式文件管理
assh-fs myserver

# 支持的命令：ls / cd / get / put / chmod / rename / ln / df / umask / lls / lcd / lpwd / exit
```

### 进度显示格式

```
( 1/5) [project.tar.gz] .............. 58%  2.1MB/s  ETA 4s
( 2/5) [config.json] ................. 100%  5.3MB/s  ETA 0s

verifying: project.tar.gz
  ✓ size match: 12.3MB / 12.3MB
  ✓ sha256: a3f2c8d1... (--checksum 开启时)
```

---

## 密钥管理

支持 RSA/Ed25519/ECDSA 密钥的生成、部署和备份。

### 基本用法

```bash
# 生成密钥并部署到服务器
assh keygen myserver

# 指定密钥类型和位数
assh keygen myserver --type ed25519
assh keygen myserver --type rsa --bits 4096
assh keygen myserver -t ecdsa -b 256

# 指定密钥注释（默认 user@host）
assh keygen myserver -C "my-key-comment"
```

### 独立生成模式

```bash
# 生成到指定路径
assh keygen -f ~/.ssh/mykey --type ed25519

# 加密码保护私钥
assh keygen -f ~/.ssh/mykey -N "passphrase123"

# 等价写法
assh keygen -f ~/.ssh/mykey -t rsa -b 4096 -C "my-key" -N "secret"
```

### 交互式向导

```bash
# 无参运行进入 ssh-keygen 风格交互向导
assh keygen
```

### 直连模式

```bash
# 在远程服务器上生成并部署密钥
assh keygen -H 192.168.1.1 -u root -P password -t ed25519
```

### 密钥备份

- 生成的密钥自动备份到 `~/.assh/v2/data/keys/{fingerprint}/` 目录
- SSH 连接时优先使用备份密钥（`data/keys/` → `~/.ssh/`）
- 直连登录的服务器自动记录到 known-servers 表，便于后续使用

---

## 代理 & 隧道

支持 SOCKS5 代理、HTTP CONNECT 代理、本地/远程端口转发，以及 Smart Proxy 智能代理系统。

### SOCKS5 正向代理

```bash
# 启动 SOCKS5 代理（默认 :1080）
assh proxy myserver

# 指定监听地址
assh proxy myserver --socks5 :1080

# 叠加 HTTP CONNECT 代理
assh proxy myserver --http :8080

# SOCKS5 用户名密码认证
assh proxy myserver --auth user:pass

# 后台守护运行
assh proxy myserver --daemon

# 自动重连（3 次尝试，5 秒间隔）
assh proxy myserver --auto-reconnect=3/5s

# 直连模式启动代理
assh proxy -H 192.168.1.1 -u root -P password --socks5 :1080
```

### Smart Proxy 智能代理

结合 AutoProxy 规则引擎和透明日志的智能路由系统：

```bash
# 加载规则文件（智能路由：匹配规则的域名走代理，其余直连）
assh proxy myserver --autoproxy ./gfwlist.txt

# 启用透明日志（JSONL 格式，按日轮转）
assh proxy myserver --log-dir ./proxy-logs

# 规则引擎 + 日志 + 代理同时启用
assh proxy myserver --autoproxy ./list.txt --log-dir ./logs --daemon

# 热重载规则
assh proxy rule reload

# 查看规则状态
assh proxy rule status

# 查看会话日志
assh proxy log <session-id>
```

### 反向代理模式

将远程服务器的端口映射到本地（类似 ssh -R）：

```bash
# 反向代理模式（默认 TCP）
assh proxy myserver --reverse

# 指定端口映射（server:80 → local:3000）
assh proxy myserver --reverse --ports 80:3000

# 多端口映射（server:80,443 → local:80）
assh proxy myserver --reverse --ports 80,443:80

# 端口范围展开 1:1
assh proxy myserver --reverse --ports 80-85:81-86

# 反向代理 + HTTP 协议 + Basic Auth
assh proxy myserver --reverse --http --ports 8080:3000 --auth user:pass

# 反向代理 + 规则 + 日志
assh proxy myserver --reverse --tcp --ports 80,443:80 --auth user:pass --autoproxy ./list.txt --log-dir ./logs
```

### 端口转发隧道

```bash
# 本地端口转发（ssh -L 等价）
assh tunnel start myserver -L 8080:localhost:80

# 远程端口转发（ssh -R 等价）
assh tunnel start myserver -R 8080:localhost:3000

# 指定绑定地址
assh tunnel start myserver -L 0.0.0.0:8080:web:80

# 多端口转发
assh tunnel start myserver -L 8080:svc1:80 -L 9090:svc2:90

# 后台守护运行
assh tunnel start myserver -L 8080:web:80 --daemon

# 自动重连
assh tunnel start myserver -L 8080:web:80 --auto-reconnect=3/5s

# 隧道管理
assh tunnel stop <id>    # 停止隧道
assh tunnel list          # 列出活跃隧道
```

---

## 命令列表

### assh（连接管理）

| 命令 | 功能 | 阶段 |
|------|------|:----:|
| `version` | 显示版本号 | ✅ Phase 0 |
| `server add` | 添加服务器 | ✅ Phase 4 |
| `server set` | 创建/更新服务器（upsert） | ✅ Phase 4 |
| `server ls` | 列出服务器 | ✅ Phase 4 |
| `server info` | 查看服务器详情（含模糊匹配建议） | ✅ Phase 4 |
| `server rm` | 删除服务器 | ✅ Phase 4 |
| `server mv` | 重命名/移动服务器 | ✅ Phase 4 |
| `server rollback` | 回滚服务器配置 | ✅ Phase 4 |
| `login` | SSH 登录（名称 / user@host / -H） | ✅ Phase 4 |
| `run` | 远程执行命令 | ✅ Phase 4 |
| `bc` | 批量并发执行命令（三通道输出） | ✅ Phase 4 |
| `keygen` | 生成/部署 SSH 密钥 | ✅ Phase 6 |
| `proxy` | 启动代理（SOCKS5/HTTP/反向/规则引擎） | ✅ Phase 7 |
| `tunnel` | 端口转发隧道管理 | ✅ Phase 7 |
| `proxy rule` | 规则管理（reload/status） | ✅ Phase 7 |
| `proxy log` | 查看代理日志 | ✅ Phase 7 |

### assh-fs（文件操作）

| 命令 | 功能 | 阶段 |
|------|------|:----:|
| `push` | 推送文件到远程服务器 | ✅ Phase 5 |
| `pull` | 从远程服务器拉取文件 | ✅ Phase 5 |
| `<server>` | 交互式 SFTP 会话 | ✅ Phase 5 |

### 全局标志

| 标志 | 说明 |
|------|------|
| `-v` / `--verbose` | 详细日志输出（DEBUG 级别） |
| `-q` / `--quiet` | 关闭日志输出 |
| `-F` / `--config` | 指定配置文件路径 |
| `-V` / `--version` | 打印版本信息 |

> **提示**：支持组合短参数，例如 `-qv` 等价于 `-q -v`。

---

## 项目状态

| Phase | 内容 | 状态 | 完成日期 |
|-------|------|:----:|:--------:|
| 0 | 项目骨架搭建 | ✅ 完成 | — |
| 1 | 基础设施移植 (log/config/crypto) | ✅ 完成 | — |
| 2 | 数据持久化层 (SQLite + AES-256-GCM) | ✅ 完成 | — |
| 3 | SSH 连接层 (认证链/HostKey/Keepalive) | ✅ 完成 | — |
| 4 | CLI 命令组装 (服务器管理/连接/版本回滚) | ✅ 完成 | 2026-05-10 |
| 5 | SFTP 文件传输 (push/pull/递归/断点续传/校验) | ✅ 完成 | 2026-05-11 |
| 6 | 密钥管理 (keygen/部署/备份/known-servers) | ✅ 完成 | 2026-05-13 |
| 7 | 代理 & 隧道 (SOCKS5/HTTP CONNECT/AutoProxy/端口转发) | ✅ 完成 | 2026-05-13 |
| **8** | **云同步 (Qiniu)** | **🔄 进行中** | — |
| 9 | 服务器健康检查 | ⏳ 待开始 | — |
| 10 | 跳板机/堡垒机支持 | ⏳ 待开始 | — |
| 11 | 远程服务器挂载 (sshfs/webdav/samba) | ⏳ 待开始 | — |
| 12 | 自动更新/升级 | ⏳ 待开始 | — |
| 13 | 远程命令执行增强 (超时/输出格式化/变量替换) | ⏳ 待开始 | — |
| X | 实验阶段：虚拟命令投放与执行 | ⏳ 待开始 | — |

---

## 配置

### 存储路径（v2 路径隔离）

| 路径 | 说明 |
|------|------|
| `~/.assh/v2/` | 配置目录 |
| `~/.assh/v2/asshv2.db` | SQLite 数据库（服务器数据 + AES-256 加密密钥） |
| `~/.assh/v2/.rsa` | RSA 私钥 |
| `~/.assh/v2/.rsa.pub` | RSA 公钥 |
| `~/.assh/v2/.account` | 加密密码文件 |
| `~/.assh/v2/assh.yml` | 配置文件 |
| `~/.assh/v2/data/keys/` | 密钥备份目录（按指纹命名） |

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ASSH_CONFIG_DIR` | 配置目录 | `~/.assh/v2` |

### 日志级别

| 级别 | 值 | 说明 |
|------|:---:|------|
| OFF | 0 | 关闭日志 |
| FATAL | 100 | 致命错误 |
| PANIC | 150 | 严重错误 |
| ERROR | 200 | 错误 |
| WARN | 300 | 警告 |
| INFO | 400 | 信息 |
| DEBUG | 500 | 调试 |

---

## 项目结构

```
asshv2/
├── main.go                  # assh 入口（DI 组合根）
├── build.sh                 # 构建脚本（构建 assh + assh-fs）
├── go.mod / go.sum          # 模块定义
│
├── cmd/                     # CLI 命令层 (urfave/cli)
│   ├── app.go               # App 组装、全局标志、命令注册
│   ├── server.go            # 服务器管理命令（add/set/ls/info/rm/mv/rollback）
│   ├── connect.go           # SSH 连接命令（login/run/bc）
│   ├── sftp_cmd.go          # push/pull/FS 命令定义（assh-fs 共享）
│   ├── proxy.go             # 代理命令（正向/反向/Smart Proxy）
│   ├── tunnel.go            # 隧道命令（start/stop/list）
│   ├── keygen.go            # 密钥生成命令
│   ├── bootstrap.go         # 公共初始化逻辑
│   └── fs/
│       └── main.go          # assh-fs 入口（DI 组合根）
│
├── asshc/                   # 核心业务库（assh + assh-fs 共享）
│   ├── domain/              # 领域实体（零依赖）
│   │   ├── server.go        # Server / Auth 实体
│   │   ├── errors.go        # 领域错误定义
│   │   └── changelog.go     # 变更日志条目
│   ├── port/                # 接口定义（依赖倒置）
│   │   ├── repository.go    # ServerRepository 接口
│   │   ├── connector.go     # SSHConnector 接口
│   │   ├── session.go       # SSHSession 接口
│   │   ├── hostkey.go       # HostKeyChecker 接口
│   │   ├── transfer.go      # FileTransfer 接口
│   │   ├── proxy.go         # Proxy/PortForward 接口
│   │   ├── rules.go         # RuleEngine 接口（AutoProxy）
│   │   ├── logger.go        # ProxyLogger 接口
│   │   ├── keymanager.go    # KeyManager 接口
│   │   └── cryptor.go       # Cryptor 接口
│   ├── service/             # 用例编排（依赖 port 接口）
│   │   ├── server.go        # 服务器 CRUD + 版本回滚
│   │   ├── connect.go       # SSH 连接与会话编排
│   │   ├── transfer.go      # SFTP 传输服务
│   │   ├── proxy.go         # 代理服务编排
│   │   ├── proxy_ports.go   # 多端口规则解析
│   │   ├── key.go           # 密钥管理服务
│   │   └── deploy.go        # 公钥部署服务
│   └── infra/               # 接口实现
│       ├── store/           # SQLite 持久化
│       │   ├── db.go        # 连接管理 + 自动迁移
│       │   ├── key.go       # AES-256-GCM 密钥管理
│       │   ├── server.go    # 服务器 CRUD + 变更日志 + 回滚
│       │   └── known.go     # known-servers 表（直连记录）
│       ├── crypto/          # 加解密实现
│       │   ├── rsa.go       # RSA 密钥生成/PEM/加解密
│       │   ├── aes.go       # AES-CBC/CTR/GCM/ECB
│       │   ├── ed25519.go   # Ed25519 密钥
│       │   └── ecdsa.go     # ECDSA-P256/P384/P521 密钥
│       ├── ssh/             # SSH 协议实现
│       │   ├── client.go    # 连接器（认证链 + Keepalive + 备份密钥优先）
│       │   ├── session.go   # 会话管理（PTY + 命令执行）
│       │   └── hostkey.go   # HostKey 校验策略
│       ├── sftp/            # SFTP 协议实现
│       │   ├── client.go    # SFTP 连接封装
│       │   ├── upload.go    # 文件上传（递归/续传/Glob）
│       │   ├── download.go  # 文件下载（递归/续传/Glob）
│       │   ├── header.go    # 256B Hash Header
│       │   ├── ops.go       # 远程操作（List/Remove/Mkdir）
│       │   ├── progress.go  # 进度条显示
│       │   └── verify.go    # 传输后验证（大小/SHA256）
│       ├── proxy/           # 代理协议实现
│       │   ├── socks5.go    # SOCKS5 代理（RFC 1928 + RFC 1929）
│       │   ├── http_connect.go # HTTP CONNECT 代理（RFC 7231）
│       │   ├── port.go      # LocalForward / RemoteForward
│       │   ├── tunnel_manager.go # 隧道管理（重连+daemon）
│       │   ├── rules.go     # AutoProxy 规则引擎
│       │   ├── logger.go    # 透明日志系统
│       │   └── listener.go  # Smart Proxy 流水线编排
│       └── keymgr/          # 密钥管理实现
│           ├── generate.go  # 密钥生成（RSA/Ed25519/ECDSA）
│           └── backup.go    # 密钥备份
│
├── config/                  # 配置管理
│   ├── config.go            # 全局配置变量
│   └── path.go              # 路径工具（展开/创建/检查）
│
├── log/                     # 日志模块（基于 zerolog）
│   └── log.go               # 7 级日志 API
│
├── README.md                # 本文件
├── CHANGELOG.md             # 更新日志
└── docs/                    # 项目文档
    ├── TODO.md              # 开发任务
    ├── plan.md              # 重构计划
    ├── architecture.md      # 架构设计
    └── ...
```

---

## 架构设计

采用 **port / service / infra** 三层架构，构建为两个独立二进制共享同一核心：

```
assh (main.go)              assh-fs (cmd/fs/main.go)
  SSH 连接/管理              文件系统操作（SFTP + 挂载）
  ├── login/run/bc          ├── push/pull
  ├── server add/set/...    └── <server> (交互式 FS)
  └── 共用核心层 ──────────────┘
        ┌──────────────────┐
        │ cmd/bootstrap.go │ → NewAppComponents() 共享初始化
        │ asshc/           │ → 三层核心业务
        └──────────────────┘
```

### 依赖原则

```
main.go (DI 组合根)
  └── cmd/ (调用 service)
        └── service/ (业务逻辑，依赖 port 接口)
              ├── port/ (接口契约)
              └── infra/ (接口实现)
```

- `service/` 只依赖 `port/` 接口 + `domain/` 实体，不直接依赖 `infra/`
- 所有依赖通过构造器注入
- `cmd/bootstrap.go` 提供 `NewAppComponents()` 共享 DB/SSH 初始化

### 认证链

SSH 连接时按以下顺序尝试认证：
1. **备份密钥优先**（如果 `known_servers` 表有记录，优先使用 `data/keys/` 的备份）
2. **SSH Agent**（通过 `SSH_AUTH_SOCK` 环境变量）
3. **密钥文件**（`--identity-file` / `--key`）
4. **密码**（`--password` 或交互式输入）
5. **认证回退**（密钥认证失败后自动回退到密码认证）

### 数据加密

- 服务器密码使用 **AES-256-GCM** 加密后存入 SQLite
- 加密密钥首次运行自动生成，存储在数据库的 `config` 表中
- 密钥通过 Base64 编码持久化

### 版本管理

- 每次服务器配置变更自动生成快照，记录到 `server_changelog` 表
- 支持回滚到任意历史版本
- 版本号自动递增

---

## 从 v1 迁移

v2.0.0 是完整重构版本，存储路径与 v1.x 不同（`~/.assh/v2/` 而非 `~/.assh/`）。
数据不直接兼容，需手动迁移或重新配置。

---

## 开发

```bash
# 运行
cd asshv2 && go run . <命令>

# 测试（7 个测试包覆盖）
cd asshv2 && go test ./... -v

# 静态检查
cd asshv2 && go vet ./...

# 构建
cd asshv2 && sh build.sh
```

---

## 许可

[MIT License](../LICENSE)
