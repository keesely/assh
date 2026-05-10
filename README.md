# ASSH - An SSH Client

`ASSH` 是一个使用 Go 开发的 SSH 客户端，适用于服务器管理、维护和快速使用。
采用 **port / service / infra** 三层架构，基于 SQLite 加密存储服务器配置，支持密码和密钥认证。

当前版本：**v2.0.0**（Phase 4 完成）

## 项目状态

| Phase | 阶段 | 状态 | 交付功能 |
|-------|------|:----:|----------|
| 0 | 项目骨架搭建 | ✅ 完成 | 目录结构、build.sh、main.go、CLI 框架 |
| 1 | 基础设施移植 | ✅ 完成 | log、config、crypto 模块 |
| 2 | 数据持久化层 | ✅ 完成 | SQLite 存储 + AES 加密 + 服务器 CRUD |
| 3 | SSH 连接层 | ✅ 完成 | SSH 客户端、会话管理、认证链 |
| 4 | CLI 命令组装 | ✅ 完成 | 全部 CLI 命令、DI 注入、版本回滚 |
| 5 | SFTP 文件传输 | ⏳ 待开始 | push/pull、断点续传 |
| 6 | 云同步 (Qiniu) | ⏳ 待开始 | 七牛云同步备份 |
| 7 | 代理 & 隧道 | ⏳ 待开始 | SOCKS5、端口转发、SSH 隧道 |

## 安装

### 编译安装

```bash
cd asshv2 && sh build.sh
```

编译产物在 `asshv2/build/` 目录。

### 单平台构建

```bash
cd asshv2 && sh build.sh <os> <arch> <alias>
# 示例：sh build.sh linux amd64 linux
```

## 快速开始

### 添加服务器

```bash
# 添加新服务器（未指定密码时将交互式提示输入）
assh server add myserver -H 192.168.1.100 -u root -P mypassword -p 22

# 使用分组
assh server add prod.web -H 10.0.0.1 -u admin -P secret --group prod

# 使用密钥认证
assh server add myserver -H example.com -u deploy -i ~/.ssh/id_rsa

# 设置自定义选项（如 Keepalive 心跳）
assh server set myserver -o keepalive=30
```

### 查看和管理服务器

```bash
# 列出所有服务器
assh server ls

# 按分组筛选
assh server ls --group prod

# 搜索服务器
assh server ls --search example

# 查看服务器详情
assh server info myserver

# 更新服务器参数（upsert）
assh server set myserver -p 2222 --remark "生产环境Web服务器"

# 重命名/移动服务器
assh server mv myserver prod.web

# 删除服务器
assh server rm myserver
```

### 登录服务器

```bash
# 通过已保存的名称登录
assh login myserver

# 直接指定主机和用户
assh login root@192.168.1.100

# 或使用 -H 参数
assh login -H 192.168.1.100 -u root -P password
```

### 执行远程命令

```bash
# 在单台服务器上执行命令
assh run myserver "uptime"

# 批量执行（多服务器并发）
assh bc "df -h" --servers web1,web2,web3

# 按分组批量执行
assh bc "systemctl status nginx" --group prod
```

### 版本回滚

```bash
# 查看变更历史
assh server rollback myserver --list

# 回滚到指定版本
assh server rollback myserver 2
```

## 命令列表

| 命令 | 功能 | 状态 |
|------|------|:----:|
| `version` | 显示版本号 | ✅ |
| `server add` | 添加服务器 | ✅ |
| `server set` | 创建/更新服务器（upsert） | ✅ |
| `server ls` | 列出服务器（支持 --group/--search） | ✅ |
| `server info` | 查看服务器详情 | ✅ |
| `server rm` | 删除服务器 | ✅ |
| `server mv` | 重命名/移动服务器 | ✅ |
| `server rollback` | 回滚服务器配置（--list 查看历史） | ✅ |
| `login` | SSH 登录（名称 / user@host / -H） | ✅ |
| `run` | 远程执行命令 | ✅ |
| `bc` | 批量并发执行命令 | ✅ |
| `push` | 推送文件 | ⏳ Phase 5 |
| `pull` | 拉取文件 | ⏳ Phase 5 |
| `keygen` | 生成 SSH 密钥 | ⏳ |
| `sync` | 云同步 | ⏳ Phase 6 |
| `proxy` | 端口代理 | ⏳ Phase 7 |

### 全局标志

| 标志 | 说明 |
|------|------|
| `-v` / `--verbose` | 详细日志输出（DEBUG 级别） |
| `-q` / `--quiet` | 关闭日志输出 |
| `-F` / `--config` | 指定配置文件路径 |
| `-V` / `--version` | 打印版本信息 |

## 配置

### 存储路径（v2 路径隔离）

| 路径 | 说明 |
|------|------|
| `~/.assh/v2/` | 配置目录 |
| `~/.assh/v2/asshv2.db` | SQLite 数据库（服务器数据 + AES 加密密钥） |
| `~/.assh/v2/.rsa` | RSA 私钥 |
| `~/.assh/v2/.rsa.pub` | RSA 公钥 |
| `~/.assh/v2/.account` | 加密密码文件 |
| `~/.assh/v2/assh.yml` | 配置文件 |

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

## 项目结构

```
asshv2/
├── main.go                  # 程序入口（DI 组合根）
├── build.sh                 # 构建脚本（多平台交叉编译）
├── go.mod / go.sum          # 模块定义
│
├── cmd/                     # CLI 命令层 (urfave/cli)
│   ├── app.go               # App 组装、全局标志、命令注册
│   ├── server.go            # 服务器管理命令（add/set/ls/info/rm/mv/rollback）
│   └── connect.go           # SSH 连接命令（login/run/bc）
│
├── asshc/                   # 核心业务库
│   ├── domain/              # 领域实体（零依赖）
│   │   ├── server.go        # Server / Auth 实体
│   │   ├── errors.go        # 领域错误定义
│   │   └── changelog.go     # 变更日志条目
│   ├── port/                # 接口定义（依赖倒置）
│   │   ├── repository.go    # ServerRepository 接口
│   │   ├── connector.go     # SSHConnector 接口
│   │   ├── session.go       # SSHSession 接口
│   │   ├── hostkey.go       # HostKeyChecker 接口
│   │   └── cryptor.go       # Cryptor 接口
│   ├── service/             # 用例编排（依赖 port 接口）
│   │   ├── server.go        # 服务器 CRUD + 版本回滚
│   │   └── connect.go       # SSH 连接与会话编排
│   └── infra/               # 接口实现
│       ├── store/           # SQLite 持久化
│       │   ├── db.go        # 连接管理 + 自动迁移
│       │   ├── key.go       # AES-256-GCM 密钥管理
│       │   └── server.go    # 服务器 CRUD + 变更日志 + 回滚
│       ├── crypto/          # 加解密实现
│       │   ├── rsa.go       # RSA 密钥生成/PEM/加解密
│       │   ├── aes.go       # AES-CBC/CTR/GCM/ECB
│       │   ├── ed25519.go   # Ed25519 密钥
│       │   └── ecdsa.go     # ECDSA-P256/P384/P521 密钥
│       └── ssh/             # SSH 协议实现
│           ├── client.go    # 连接器（认证链 + Keepalive）
│           ├── session.go   # 会话管理（PTY + 命令执行）
│           └── hostkey.go   # HostKey 校验策略
│
├── config/                  # 配置管理
│   ├── config.go            # 全局配置变量
│   └── path.go              # 路径工具（展开/创建/检查）
│
├── log/                     # 日志模块（基于 zerolog）
│   └── log.go               # 7 级日志 API
│
├── lib/                     # 第三方库封装
│   └── qiniu/               # 七牛云存储（Phase 6 预热）
│       ├── qiniu.go
│       ├── bucket.go
│       └── utils.go
│
├── README.md                # 本文件
├── CHANGELOG.md             # 更新日志
└── docs/                    # 项目文档
    └── ...
```

## 架构设计

采用 **port / service / infra** 三层架构，遵循依赖倒置原则：

```
main.go (DI 组合根)
  └── cmd/ (调用 service)
        └── service/ (业务逻辑，依赖 port 接口)
              ├── port/ (接口契约)
              └── infra/ (接口实现)
```

- `service/` 只依赖 `port/` 接口 + `domain/` 实体，不直接依赖 `infra/`
- 所有依赖通过构造器注入
- `cmd/` 在 `main.go` 中完成组装（组合根）

### 认证链

SSH 连接时按以下顺序尝试认证：
1. **SSH Agent**（通过 `SSH_AUTH_SOCK` 环境变量）
2. **密钥文件**（`--identity-file` / `--key`）
3. **密码**（`--password` 或交互式输入）

### 数据加密

- 服务器密码使用 **AES-256-GCM** 加密后存入 SQLite
- 加密密钥自动生成并存储在数据库的 `config` 表中
- 密钥通过 Base64 编码持久化

### 版本管理

- 每次服务器配置变更自动生成快照，记录到 `server_changelog` 表
- 支持回滚到任意历史版本
- 版本号自动递增

## 从 v1 迁移

v2.0.0 是完整重构版本，存储路径与 v1.x 不同（`~/.assh/v2/` 而非 `~/.assh/`）。
数据不直接兼容，需手动迁移或重新配置。

## 开发

```bash
# 运行
cd asshv2 && go run . <命令>

# 测试（48+ 个测试用例覆盖）
cd asshv2 && go test ./...

# 静态检查
cd asshv2 && go vet ./...

# 构建
cd asshv2 && sh build.sh
```

## 许可

[MIT License](../LICENSE)
