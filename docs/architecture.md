# Architecture

## 数据流程

```
用户输入 → asshv2/cmd/ 解析 CLI 参数
           ↓
       main.go (DI 组合根) 创建具体实现
           ↓
       service/ (业务用例，调用 port 接口)
           ↓
       infra/ (接口实现，执行具体操作)
           ↓
       SSH 连接 / SFTP 传输 / 加密存储 / 云同步
```

## 依赖注入流程

```
main.go
  ├── NewCryptor()          → asshc/infra/crypto
  ├── NewServerRepository() → asshc/infra/store
  ├── NewServerService()    → asshc/service (receives port interfaces)
  ├── NewConnectService()   → asshc/service
  └── cmd.NewApp()          → cmd/ (receives service interfaces)
```

## 项目结构

```
asshv2/
├── main.go              # 程序入口（组合根）
├── cmd/                 # CLI 命令层 (urfave/cli)
│   ├── app.go           # App 组装、命令注册
│   ├── server.go        # 服务器管理命令
│   ├── connect.go       # SSH 连接命令
│   ├── sftp.go          # 文件传输命令
│   ├── proxy.go         # 代理命令
│   ├── sync.go          # 云同步命令
│   ├── keygen.go        # 密钥生成命令
│   ├── upgrade.go       # 升级命令
│   └── qiniu/           # 七牛子命令
├── asshc/               # 核心业务库
│   ├── domain/          # 领域实体
│   ├── port/            # 接口定义
│   ├── service/         # 用例编排
│   └── infra/           # 接口实现
│       ├── store/       #   加密文件存储
│       ├── crypto/      #   加解密
│       ├── ssh/         #   SSH 协议
│       ├── sftp/        #   SFTP 协议
│       ├── tunnel/      #   SSH 隧道
│       ├── proxy/       #   代理协议
│       └── sync/        #   云同步
├── config/              # 配置管理
├── log/                 # 日志模块
└── docs/                # 文档
```

## 架构原则

1. **依赖倒置**: `service/` 依赖 `port/` 接口，不依赖 `infra/` 实现
2. **构造器注入**: 所有依赖通过构造函数传入
3. **组合根**: `main.go` 是唯一组装所有依赖的地方
4. **封装边界**: `asshc/` 内的实现细节对其他包不可见（通过 Go 包设计约束）
