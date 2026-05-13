# Architecture

## 数据流程

```
用户输入 → assh / assh-fs 解析 CLI 参数
           ↓
       main.go / cmd/fs/main.go (DI 组合根) 创建具体实现
           ↓
       service/ (业务用例，调用 port 接口)
           ↓
       infra/ (接口实现，执行具体操作)
           ↓
       SSH 连接 / SFTP 传输 / 加密存储 / 云同步
```

## 二进制架构

项目构建为两个独立的可执行命令，共享同一套核心模块：

```
assh (main.go)                    assh-fs (cmd/fs/main.go)
  SSH 连接/管理                     文件系统操作
  ├── login/run/bc                ├── push
  ├── server add/set/ls/...       ├── pull
  ├── version                     └── <server> (交互式 FS)
       │                                    │
       └────────── 共用核心层 ──────────────┘
              ┌──────────────────┐
              │  asshc/          │
              │  config/         │
              │  log/            │
              │  go.mod          │
              └──────────────────┘
```

## 依赖注入流程

### assh (main.go)
```
main.go
  ├── NewServerRepository() → asshc/infra/store
  ├── NewServerService()    → asshc/service
  ├── NewSSHConnector()     → asshc/infra/ssh
  ├── NewSSHSession()       → asshc/infra/ssh
  ├── NewConnectService()   → asshc/service
  └── cmd.NewApp()          → cmd/ (SSH 命令 + 服务器管理)
```

### assh-fs (cmd/fs/main.go)
```
cmd/fs/main.go
  ├── NewServerRepository() → asshc/infra/store
  ├── NewServerService()    → asshc/service (用于服务器名查找)
  ├── NewSSHConnector()     → asshc/infra/ssh
  ├── NewFileTransfer()     → asshc/infra/sftp
  ├── NewTransferService()  → asshc/service
  └── cmd.NewFSApp()        → cmd/ (push/pull/交互式FS命令)
```

## 项目结构

```
asshv2/
├── main.go              # assh 入口（DI 组合根）
├── cmd/
│   ├── app.go           # assh App 组装、命令注册
│   ├── server.go        # 服务器管理命令
│   ├── connect.go       # SSH 连接命令
│   ├── sftp_cmd.go      # push/pull/FS 命令定义（共享）
│   ├── fs/
│   │   └── main.go      # assh-fs 入口（DI 组合根）NEW
│   ├── bootstrap.go     # 公共初始化逻辑（DB/Service）NEW
│   ├── keygen.go        # 密钥生成命令
│   ├── upgrade.go       # 升级命令
│   └── qiniu/           # 七牛子命令
├── asshc/               # 核心业务库（两部分共享）
│   ├── domain/          # 领域实体
│   ├── port/            # 接口定义
│   ├── service/         # 用例编排
│   └── infra/           # 接口实现
│       ├── store/       #   SQLite 数据存储（db/key/server/known）
│       ├── crypto/      #   加解密
│       ├── keymgr/      #   密钥管理（generate/backup/deploy）
│       ├── ssh/         #   SSH 协议
│       ├── sftp/        #   SFTP 协议
│       ├── tunnel/      #   SSH 隧道
│       ├── proxy/       #   代理协议
│       └── sync/        #   云同步
├── config/              # 配置管理（共享）
├── log/                 # 日志模块（共享）
└── docs/                # 文档
```

## 架构原则

1. **依赖倒置**: `service/` 依赖 `port/` 接口，不依赖 `infra/` 实现
2. **构造器注入**: 所有依赖通过构造函数传入
3. **组合根**: `main.go` 是唯一组装所有依赖的地方
4. **封装边界**: `asshc/` 内的实现细节对其他包不可见（通过 Go 包设计约束）
