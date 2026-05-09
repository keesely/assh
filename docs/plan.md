# assh v2.0.0 项目重构计划

## 项目概述

`assh` 是一个 Go 开发的 SSH 客户端，用于管理服务器、密码、公钥和文件传输。

### 背景

`v2.0.0` 是全新的重构项目，基于以下原则：
- 保持现有代码风格
- 使用 TDD 开发模式
- 保持 build.sh 编译方式

---

## 技术选型

| 技术 | 方案 | 说明 |
|------|------|------|
| **Go 版本** | 1.22.0 | 保持现有版本 |
| **日志库** | zerolog v1.32.0 | 高性能结构化日志 |
| **CLI 框架** | urfave/cli v1.22.2 | 保持现有 |
| **数据库** | SQLite (modernc.org/sqlite) | 纯 Go，无 CGO，跨平台编译无阻 |
| **加密方案** | RSA + AES | 保持现有 |
| **依赖管理** | go mod tidy | 现代 Go 模块管理 |

---

## 项目结构

```
asshv2/
├── main.go                    # 程序入口（组合根, DI 注入）
├── build.sh                   # 构建脚本
├── go.mod / go.sum            # 模块定义
│
├── cmd/                       # CLI 命令层 (urfave/cli)
│   ├── app.go                 # App 组装、命令注册
│   ├── server.go              # 服务器管理命令
│   ├── connect.go             # SSH 连接命令
│   ├── sftp.go                # 文件传输命令
│   ├── proxy.go               # 代理命令
│   ├── sync.go                # 云同步命令
│   ├── keygen.go              # 密钥生成命令
│   ├── upgrade.go             # 升级命令
│   └── qiniu/                 # 七牛子命令
│
├── asshc/                     # 核心业务库（私有封装）
│   ├── domain/                # 领域实体（零依赖）
│   │   ├── server.go          # Server 实体
│   │   └── errors.go          # 领域错误
│   ├── port/                  # 接口定义（依赖倒置）
│   │   ├── repository.go      # ServerRepository
│   │   ├── connector.go       # SSHConnector
│   │   ├── transfer.go        # FileTransfer
│   │   ├── proxy.go           # Proxy
│   │   ├── syncer.go          # Syncer
│   │   └── cryptor.go         # Cryptor
│   ├── service/               # 用例编排（依赖 port 接口）
│   │   ├── server.go          # 服务器 CRUD
│   │   ├── auth.go            # 认证管理
│   │   ├── connect.go         # SSH 连接
│   │   ├── transfer.go        # SFTP 传输
│   │   ├── proxy.go           # 代理
│   │   └── sync.go            # 同步
│   └── infra/                 # 接口实现
│       ├── store/             # SQLite 数据存储
│       │   ├── db.go          # 连接管理 + 迁移
│       │   ├── key.go         # AES 静态密钥管理
│       │   └── server.go      # 服务器 CRUD
│       ├── crypto/            # 加解密实现
│       │   ├── rsa.go
│       │   ├── aes.go
│       │   ├── ed25519.go
│       │   └── ecdsa.go
│       ├── ssh/               # SSH 协议
│       │   ├── client.go
│       │   └── session.go
│       ├── sftp/              # SFTP 协议
│       │   └── handler.go
│       ├── tunnel/            # SSH 隧道
│       │   └── tunnel.go
│       ├── proxy/             # 代理协议
│       │   ├── socks5.go
│       │   └── port.go
│       └── sync/              # 云同步
│           └── qiniu/
│               ├── client.go
│               ├── bucket.go
│               └── utils.go
│
├── config/                    # 配置管理（公开 API）
│   ├── config.go              # 全局配置变量
│   └── path.go                # 路径管理
│
├── log/                       # 日志模块（公开 API）
│   ├── log.go
│   └── log_test.go
│
├── README.md                  # 用户手册
├── CHANGELOG.md               # 更新日志
├── build.sh
├── go.mod / go.sum
└── docs/                      # 项目文档
    ├── plan.md                # 本文件
    ├── TODO.md                # 任务清单
    ├── architecture.md        # 架构设计
    ├── getting-started.md     # 开发指南
    ├── constraints.md         # 约束条件
    ├── storage.md             # 存储路径
    ├── bugs.md                # Bug 追踪
    └── modules/               # 模块文档
```

---

## 模块职责

| 模块 | 职责 | 依赖 |
|------|------|------|
| **cmd** | CLI 命令定义，用户交互入口 | 所有 service |
| **asshc/service** | 用例编排，业务逻辑 | port 接口 |
| **asshc/port** | 接口定义（依赖倒置） | domain |
| **asshc/domain** | 领域实体 | 无 |
| **asshc/infra** | 接口实现 | 具体外部库 |
| **config** | 配置路径、全局变量 | 无 |
| **log** | 日志记录 | 无 |

### 架构约束

- `service/` 只依赖 `port/` 接口 + `domain/` 实体，不直接依赖 `infra/`
- 所有依赖通过构造器注入
- `cmd/` 在 `main.go` 中完成组装（组合根）

```
main.go (DI 组合根)
  └── cmd/ (调用 service)
        └── service/ (实现业务逻辑)
              ├── port/ (接口契约)
              └── infra/ (接口实现)
```

---

## 功能列表

### 核心功能 (必须)

| 功能 | 说明 |
|------|------|
| SSH 连接 | 密码认证、公钥认证、交互式终端 |
| 服务器管理 | 增删改查、分组、命名规则 (group.server) |
| 数据加密 | AES-CBC 模式加密，MD5 签名验证 |
| 配置文件 | RSA 加密存储 |

### 重要功能 (可选)

| 功能 | 说明 |
|------|------|
| SFTP 传输 | push/pull、断点续传、进度显示 |
| SSH 隧道 | 本地端口转发、远程端口转发 |
| SOCKS5 代理 | 动态代理功能 |
| 云同步 | 七牛云存储备份 |

### 辅助功能

| 功能 | 说明 |
|------|------|
| SSH 密钥生成 | keygen 命令 |
| 批量命令执行 | bc 命令 |
| 压缩功能 | zip 命令 |
| 版本升级 | upgrade 命令 |

---

## 日志模块

### API 设计

```go
// 全局变量
var LogPath string
var LogLevel int

// 初始化
func SetInit()
func GetLogLevel(string) int

// 输出方法
func Print(args ...interface{})
func Printf(format string, args ...interface{})
func Println(args ...interface{})

// 级别方法
func Debug(args ...interface{}) / Debugf(format string, args ...interface{})
func Info(args ...interface{})  / Infof(format string, args ...interface{})
func Warn(args ...interface{})  / Warnf(format string, args ...interface{})
func Error(args ...interface{}) / Errorf(format string, args ...interface{})
func Fatal(args ...interface{}) / Fatalf(format string, args ...interface{})
func Panic(args ...interface{}) / Panicf(format string, args ...interface{})
```

### 日志级别常量

| 常量 | 值 | 说明 |
|------|-----|------|
| OFF   | 0  | 关闭日志 |
| FATAL | 100 | 致命错误，程序退出 |
| PANIC | 150 | 严重错误，触发 panic |
| ERROR | 200 | 错误 |
| WARN  | 300 | 警告 |
| INFO  | 400 | 信息 |
| DEBUG | 500 | 调试 |

### 技术细节

| 项目 | 方案 |
|------|------|
| 日志库 | zerolog v1.32.0 |
| 输出格式 | JSON (zerolog 默认) |
| 时间戳 | 自动包含 ISO8601 格式 |
| 文件输出 | 支持追加模式 (os.O_APPEND) |
| 目录创建 | 自动创建父目录 (os.MkdirAll) |
| 级别过滤 | 消息级别 <= LogLevel 时输出 |

---

## 开发顺序

| Phase | 模块 | 依赖 | 预计文件数 |
|-------|------|------|------------|
| 0 | 项目骨架 | 无 | 4 |
| 1 | log + config + crypto | 无 | 8 |
| 2 | domain + repository + SQLite store + service | config, log, sqlite | 6 |
| 3 | ssh connector + session + hostkey | Phase 2 (domain/repository), config, log | 7 (+ tests) |
| 4 | cmd 组装 | 所有以上 | 5 |
| 5 | sftp | ssh | 4 |
| 6 | sync/qiniu | 无 | 6 |
| 7 | proxy + tunnel | ssh | 6 |

详细任务清单见 [TODO.md](TODO.md)。

---

## 存储位置

运行时数据存储位置（v2 路径隔离在 `~/.assh/v2/` 下）：

| 路径 | 说明 |
|------|------|
| ~/.assh/v2 | 配置目录 |
| ~/.assh/v2/.rsa | RSA 私钥 |
| ~/.assh/v2/.rsa.pub | RSA 公钥 |
| ~/.assh/v2/.account | RSA 加密密码（Phase 6） |
| ~/.assh/v2/assh.yml | RSA 加密配置 |
| ~/.assh/v2/asshv2.db | SQLite 数据库（服务器数据 + AES 静态密钥） |

---

## 版本管理

- 当前版本：v2.0.0-dev (开发中)
- 语义版本：semver
- 标签格式：v2.0.0, v2.0.1, ...

---

## 文档索引

- [开发指南](getting-started.md) - 开发命令、测试
- [架构设计](architecture.md) - 数据流程、项目结构
- [关键约束](constraints.md) - Go 版本、平台限制
- [存储位置](storage.md) - 配置文件、数据文件路径
- [任务清单](TODO.md) - 分阶段开发任务
- [Bug 追踪](bugs.md) - 已知问题和修复记录

### 模块文档

- [日志模块](modules/log.md) - 日志记录
- [配置模块](modules/config.md) - 初始化和加密
- [服务器数据模块](modules/assh.md) - 服务器列表管理
- [SSH 连接模块](modules/server.md) - 连接和会话
- [SFTP 模块](modules/sftp.md) - 文件传输
- [代理模块](modules/proxy.md) - 代理和端口转发
- [CLI 命令](modules/cli.md) - 命令列表
