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

### 📝 Phase 2: 数据持久化层

目标：实现加密的服务器数据存储

依赖：Phase 1 (config, crypto)

| 任务 | 文件 | 说明 |
|------|------|------|
| P2.1 领域实体 | `asshc/domain/server.go` | Server 类型定义（Group, Name, Host, Port, User, Auth, Remark） |
| P2.2 领域错误 | `asshc/domain/errors.go` | 哨兵错误（ErrNotFound, ErrExists, ErrInvalidName 等） |
| P2.3 Repository 接口 | `asshc/port/repository.go` | ServerRepository 接口（List/Get/Set/Delete/Move） |
| P2.4 加密文件存储 | `asshc/infra/store/filestore.go` | 实现 ServerRepository，AES 加密存储 |
| P2.5 服务器服务 | `asshc/service/server.go` | ServerService 用例编排 |
| P2.6 认证服务 | `asshc/service/auth.go` | AuthService 密码设置/验证 |
| **验证**: `go test ./asshc/... -v` | |

### 📝 Phase 3: SSH 连接层

目标：实现 SSH 登录和远程命令执行

依赖：Phase 2 (domain/repository)

| 任务 | 文件 | 说明 |
|------|------|------|
| P3.1 Connector 接口 | `asshc/port/connector.go` | SSHConnector 接口 |
| P3.2 SSH 客户端 | `asshc/infra/ssh/client.go` | golang.org/x/crypto/ssh 封装 |
| P3.3 会话管理 | `asshc/infra/ssh/session.go` | 交互终端、keepalive |
| P3.4 连接服务 | `asshc/service/connect.go` | ConnectService 登录/命令 |
| **验证**: `go test ./asshc/... -v` | |

### 📝 Phase 4: CLI 命令组装

目标：组装完整 CLI，实现 MVP

依赖：Phase 2 + Phase 3

| 任务 | 文件 | 说明 |
|------|------|------|
| P4.1 服务器命令 | `cmd/server.go` | add/ls/info/rm/mv |
| P4.2 连接命令 | `cmd/connect.go` | login/bc |
| P4.3 全局入口 | `cmd/app.go` | 补全命令注册 |
| P4.4 DI 注入 | `main.go` | 完整依赖注入 |
| **验证**: `go run . help` | 全命令可见 |

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
