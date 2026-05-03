# AGENTS.md

## 项目概述

`assh` 是一个 Go 开发的 SSH 客户端，用于管理服务器、密码、公钥和文件传输。

## 项目结构

项目代码位于 `assh/` 子目录下：

```
assh/                   # Go 项目根目录
├── main.go             # 程序入口
├── cmd/                # CLI 命令定义 (urfave/cli)
│   ├── command.go      # 主命令定义和核心逻辑
│   ├── server.go       # 服务器管理命令实现
│   ├── zip.go          # 压缩功能
│   ├── upgrade.go      # 升级功能
│   └── qiniu/          # 七牛云存储集成
├── asshc/              # 核心业务库
│   ├── assh.go         # 服务器数据管理（增删改查、加密存储）
│   ├── config.go       # 配置管理（初始化、密码、密钥）
│   ├── server.go       # SSH 连接和会话管理
│   ├── sftp.go         # SFTP 文件传输
│   ├── proxy.go        # 代理和端口转发
│   ├── utils.go        # 工具函数
│   ├── keygen/         # SSH 密钥生成
│   └── localproxy.go   # 本地代理
├── log/                # 日志模块
├── build/              # 构建相关
└── build.sh            # 构建脚本
```

## 开发命令

- **编译构建**: `sh assh/build.sh`（内部会执行 `go mod tidy`）
- **单平台构建**: `sh assh/build.sh <os> <arch> <alias>`
- **运行**: `cd assh && go run . <命令>`

## 测试

- 运行测试: `cd assh && go test ./...`
- 单独测试: `cd assh && go test asshc/keygen -v`

## 核心模块分析

### 1. config.go - 配置模块

初始化和数据加密：

- **初始化流程**：创建 `~/.assh` 目录，自动生成 RSA 密钥对 (`~/.assh/.rsa`)
- **密码管理**：`account` 命令设置安全密码，用于加密服务器数据
- **数据存储位置**：`~/.assh/data/servers.db`（AES 加密）
- **配置文件**：`~/.assh/assh.yml`（RSA 加密）

### 2. assh.go - 服务器数据模块

管理服务器列表的核心数据结构：

- **数据结构**：`map[string]map[string]*Server`（群组 -> 服务器名 -> 服务器信息）
- **命名格式**：`group.server_name`（群组可省略）
- **数据加密**：AES-CBC 模式加密，带 MD5 签名验证
- **密钥管理**：每个服务器可独立关联公钥文件

### 3. server.go - SSH 连接模块

基于 `golang.org/x/crypto/ssh` 实现：

- **认证方式**：密码认证 + 公钥认证
- **交互式终端**：支持 TTY 和终端窗口大小调整
- **心跳保活**：支持 `ServerAliveInterval` 配置
- **远程命令执行**：通过 `-c` 参数

### 4. sftp.go - 文件传输模块

基于 `github.com/pkg/sftp` 实现：

- **推送**：`push` 命令，支持文件和目录
- **拉取**：`pull` 命令
- **断点续传**：大文件分块上传，显示进度和速率

### 5. proxy.go - 代理模块

支持多种代理模式：

- **SOCKS5 代理**：`proxy` 命令
- **端口转发**：`hostproxy` 命令（远程端口 → 本地端口）
- **SSH 隧道**：`localproxy` 命令

### 6. cmd/ - CLI 命令层

使用 `urfave/cli` 框架构建：

| 命令 | 功能 |
|------|------|
| `account` | 设置安全密码 |
| `ls/search` | 列出/搜索服务器 |
| `info` | 查看服务器详情 |
| `set` | 添加/修改服务器 |
| `rm` | 删除服务器 |
| `login` | 连接到服务器 |
| `bc` | 批量命令执行 |
| `push/pull` | 文件传输 |
| `keygen` | 生成 SSH 密钥 |
| `sync` | 云端同步（七牛云） |
| `export/import` | 导入导出配置 |
| `upgrade` | 检测更新 |
| `ping` | 检测服务器连通性 |
| `proxy/hostproxy/localproxy` | 代理功能 |

## 关键约束

- Go 版本: 1.22.0
- 平台: 不支持 Windows 编译
- 测试文件: `assh/asshc/keygen/keygen_test.go`, `assh/log/log_test.go`

## 数据流程

```
用户输入 → assh/cmd/command.go 解析
          ↓
      assh/asshc/config.go 初始化（检查密码、加载配置）
          ↓
      assh/asshc/assh.go 加载服务器数据（解密）
          ↓
      assh/asshc/server.go 建立 SSH 连接
          ↓
      终端会话 / SFTP 传输 / 远程命令执行
```

## 存储位置

- 配置目录：`~/.assh`
- 数据文件：`~/.assh/data/servers.db`（AES 加密）
- 密码文件：`~/.assh/.account`（RSA 加密）
- 密钥文件：`~/.assh/.rsa`, `~/.assh/.rsa.pub`
- 配置文件：`~/.assh/assh.yml`（RSA 加密）