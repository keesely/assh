# ASSH - An SSH Client

`ASSH` 是一个使用 Go 开发的 SSH 客户端，适用于服务器管理、维护和快速使用。当前版本：**v2.0.0-dev**（重构中）。

## 项目状态

| Phase | 阶段 | 状态 | 交付功能 |
|-------|------|------|----------|
| 0 | 项目骨架搭建 | ✅ 完成 | 目录结构、build.sh、main.go、CLI 框架 |
| 1 | 基础设施移植 | ✅ 完成 | log、config、crypto 模块移植 |
| 2 | 数据持久化层 | 📝 进行中 | Server 实体、加密存储、CRUD 服务 |
| 3 | SSH 连接层 | ⏳ 待开始 | SSH 客户端、会话管理、远程命令 |
| 4 | CLI 命令组装 | ⏳ 待开始 | 全部 CLI 命令、DI 注入 |
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

### 设置安全密码

```bash
assh account <password>
```

### 添加服务器

```bash
assh set group.name -H <host> -u <user> -p <password>
```

### 登陆服务器

```bash
assh login group.name
# 或快捷方式（当唯一参数时）
assh group.name
```

### 执行远程命令

```bash
assh group.name -c "命令"
```

## 命令列表

| 命令 | 功能 | 状态 |
|------|------|------|
| `version` | 显示版本 | ✅ |
| `account` | 设置安全密码 | 📝 |
| `ls` | 列出服务器 | 📝 |
| `search` | 搜索服务器 | 📝 |
| `info` | 查看服务器详情 | 📝 |
| `set` | 添加/修改服务器 | 📝 |
| `mv` | 移动/重命名服务器 | 📝 |
| `rm` | 删除服务器 | 📝 |
| `login` | 连接服务器 | 📝 |
| `bc` | 批量命令执行 | 📝 |
| `push` | 推送文件 | ⏳ |
| `pull` | 拉取文件 | ⏳ |
| `keygen` | 生成 SSH 密钥 | ⏳ |
| `sync` | 云同步 | ⏳ |
| `proxy` | 端口代理 | ⏳ |
| `hostproxy` | 域名代理 | ⏳ |
| `localproxy` | SSH 隧道 | ⏳ |
| `upgrade` | 自动升级 | ⏳ |
| `ping` | 服务器连通性 | ⏳ |
| `export/import` | 导入/导出配置 | ⏳ |

## 配置

### 存储路径

| 路径 | 说明 |
|------|------|
| `~/.assh` | 配置目录 |
| `~/.assh/.rsa` | RSA 私钥 |
| `~/.assh/.rsa.pub` | RSA 公钥 |
| `~/.assh/.account` | 加密密码 |
| `~/.assh/assh.yml` | 配置文件 |
| `~/.assh/data/servers.db` | 加密服务器数据 |

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ASSH_CONFIG_DIR` | 配置目录 | `~/.assh` |
| `ASSH_LOG_FILE` | 日志文件路径 | 空（stderr） |
| `ASSH_LOG_LEVEL` | 日志级别 | `OFF` |

### 日志级别

| 级别 | 值 | 说明 |
|------|-----|------|
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
├── main.go                    # 程序入口（组合根, DI 注入）
├── build.sh                   # 构建脚本
├── go.mod / go.sum            # 模块定义
│
├── cmd/                       # CLI 命令层
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
├── asshc/                     # 核心业务库
│   ├── domain/                # 领域实体
│   ├── port/                  # 接口定义（依赖倒置）
│   ├── service/               # 用例编排
│   └── infra/                 # 接口实现
│       ├── store/             # 加密文件存储
│       ├── crypto/            # 加解密实现
│       ├── ssh/               # SSH 协议
│       ├── sftp/              # SFTP 协议
│       ├── tunnel/            # SSH 隧道
│       ├── proxy/             # 代理协议
│       └── sync/              # 云同步
│
├── config/                    # 配置管理
├── log/                       # 日志模块
├── README.md                  # 本文件
└── CHANGELOG.md               # 更新日志
```

## 从 v1 迁移

v2.0.0 是完整重构版本，存储路径与 v1.x 兼容（`~/.assh/`）。
数据文件格式不变，可直接使用 v1 创建的配置文件。

## 开发

```bash
# 运行
cd asshv2 && go run . <命令>

# 测试
cd asshv2 && go test ./...

# 构建
cd asshv2 && sh build.sh
```

## 许可

[MIT License](../LICENSE)
