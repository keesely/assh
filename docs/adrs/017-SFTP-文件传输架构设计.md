# ADR-017: SFTP 文件传输架构设计

## Status
accepted (amended 2026-05-10: 增加二进制拆分决策)

## Date
2026-05-10 (initial)
2026-05-10 (binary-split amendment)

## Context
Phase 5 需要实现 SFTP 文件传输功能，支持文件推送（push）和拉取（pull）。

核心需求：
1. 基于 SSH 连接实现安全的文件传输
2. 支持递归目录传输、断点续传、并发传输、进度显示、通配符匹配
3. 提供 push/pull 命令 + 交互式 SFTP 会话
4. 支持已保存服务器配置和直连两种连接方式
5. 支持文件覆盖策略（询问/强制覆盖/跳过）

需要选择合适的 Go SFTP 库来实现底层协议，并遵循项目现有的 port/service/infra 三层架构。

## Decision

### 技术选型
采用 `github.com/pkg/sftp` v1.13.7+ 作为 SFTP 协议实现库。

理由：
- Go 生态中最成熟的 SFTP 库，广泛使用
- 原生支持 `golang.org/x/crypto/ssh`，可直接复用现有 SSH 连接
- 支持并发读写操作，性能良好
- 纯 Go 实现，无 CGO 依赖，保持跨平台编译能力
- 维护活跃，社区成熟

### 架构设计（初始方案 — 单体）
前期设计为将 SFTP 命令嵌入 `assh` 单体二进制：

```
cmd/sftp.go                    # CLI 命令：push/pull/sftp
  └── service/transfer.go      # TransferService：传输用例编排
        ├── port/transfer.go   # FileTransfer 接口（已有）
        └── infra/sftp/        # pkg/sftp 封装实现
              ├── client.go    # SFTP 连接管理
              ├── upload.go    # 上传实现
              ├── download.go  # 下载实现
              ├── ops.go       # List/Remove/Mkdir
              └── progress.go  # 进度显示
```

### 修正方案：二进制拆分 (Binary Split)

**决策时间**：2026-05-10（Phase 5 需求细化阶段）

经过需求分析，决定将文件传输功能拆分为独立的二进制 `assh-fs`，与 SSH 客户端 `assh` 共享核心模块。

**架构层级**：
```
assh (主二进制)
  ├── main.go                 # assh 入口点
  ├── cmd/app.go              # assh CLI 命令注册
  ├── cmd/connect.go          # login/run/bc 命令
  ├── cmd/server.go           # server 管理命令
  └── cmd/bootstrap.go        # 共享初始化逻辑（DB/Crypto/Log）

assh-fs (文件系统工具二进制)
  ├── cmd/fs/main.go          # assh-fs 入口点
  ├── cmd/sftp_cmd.go         # push/pull/sftp CLI 命令定义
  └── cmd/bootstrap.go        # 共享初始化逻辑（同上）

共享核心 (asshc/)
  ├── domain/
  ├── port/
  │   ├── repository.go       # ServerRepository 接口
  │   ├── connector.go        # SSHConnector 接口
  │   ├── transfer.go         # FileTransfer 接口
  │   └── ...
  ├── service/
  │   ├── server.go           # ServerService
  │   ├── connect.go          # ConnectService
  │   ├── transfer.go         # TransferService（新增）
  │   └── ...
  └── infra/
      ├── store/              # SQLite 持久化
      ├── crypto/             # 加解密
      ├── ssh/                # SSH 连接
      └── sftp/               # SFTP 文件传输（新增）
            ├── client.go     # SFTP 连接管理
            ├── upload.go     # 上传实现
            ├── download.go   # 下载实现
            ├── ops.go        # List/Remove/Mkdir
            ├── progress.go   # 进度显示
            └── verify.go     # 传输后校验
```

**拆分理由**：
1. 职责分离：SSH 客户端（交互式 shell/命令执行）与文件传输工具（批量/后台传输）是不同的使用场景
2. 未来可扩展：`assh-fs` 后续可加入挂载功能（Phase 12 sshfs/webdav/samba），保持二进制单一职责
3. 关注点隔离：文件传输特有的依赖（pkg/sftp）、flag 参数（大量短参冲突）不影响 assh 主二进制
4. 复用核心：两个二进制共享 `asshc/` 下所有 domain/port/service/infra 模块，通过 `cmd/bootstrap.go` 统一初始化

**共享方式**：
- `cmd/bootstrap.go` 抽取出 `NewAppComponents()` 函数，返回 DB/SSH/Service 的初始化组件
- `main.go` 和 `cmd/fs/main.go` 各自调用 `NewAppComponents()` 然后注册自己的命令集
- 核心差异：`cmd/bootstrap.go` 编译时在两个 binary 中各自独立链接

### 文件覆盖策略
- 默认：交互式询问用户
- `-f/--force`：强制覆盖已存在文件
- `--skip`：跳过已存在文件

### 连接方式
- 已保存服务器：通过 server name 从数据库获取配置
- 直连：通过 `-H`/`-u`/`-p`/`-P`/`-i`/`-k` 参数直接指定连接参数

## Consequences

### 正面影响
- 新增依赖 `github.com/pkg/sftp`，但纯 Go 无 CGO，保持跨平台编译能力
- SFTP 连接复用现有 SSH 连接（通过 `ssh.Client` 创建 `sftp.Client`），无需额外认证
- 接口已预定义，infra 层实现与现有 SSH 实现模式一致
- 进度显示回调机制已在接口层预留
- **二进制拆分**：`assh` 和 `assh-fs` 互不干扰，各自的 flag 命名无冲突顾虑
- **体积控制**：SFTP 相关依赖仅链接到 `assh-fs`，`assh` 主二进制体积不受影响
- **未来扩展**：Phase 12 挂载功能可直接添加到 `assh-fs`，无需修改 `assh`

### 负面影响
- `-e`（原 `--resume`）作为唯一可用的短参，需注意未来扩展
- 断点续传采用偏移量分块方案，依赖 pkg/sftp 的 `ReadAt`/`WriteAt`/`Seek` 能力
- 传输后验证（大小比对 + SHA256）增加传输完成后的处理时间
- `--checksum` 涉及远程 SSH exec 执行 `sha256sum`，需要额外的 SSH 会话
- 大文件传输时内存占用需关注
- 新增测试类型（mock + local SFTP server + 集成测试）
- **构建复杂度增加**：需支持双二进制构建（build.sh 改造）
- **用户认知成本**：需要理解 `assh` 和 `assh-fs` 是两个不同命令

### 影响范围
| 范围 | 文件 |
|------|------|
| 新增依赖 | go.mod (github.com/pkg/sftp v1.13.7+) |
| 新建文件 | cmd/fs/main.go，cmd/sftp_cmd.go，cmd/bootstrap.go，infra/sftp/ 下 6 文件 |
| 新增入口点 | cmd/fs/main.go — assh-fs 独立入口 |
| 共享初始化 | cmd/bootstrap.go — NewAppComponents() 被两个 binary 共用 |
| DI 注入 | main.go（assh）+ cmd/fs/main.go（assh-fs）各自注入 |
| App 签名变更 | 无变更（assh 命令不变，assh-fs 独立注册） |
| 构建脚本 | build.sh 需支持双二进制并行构建 |

## Related
- ADR-011: SSH 连接层架构设计
- ADR-019: 传输文件 Hash Header — 断点续传安全增强（本 ADR 的补充）
- Phase 3: SSH 连接层依赖
- docs/modules/sftp.md — SFTP 模块详细设计
- docs/architecture.md — 二进制拆分架构图
- build.sh — 双二进制构建脚本
- cmd/bootstrap.go — 共享初始化逻辑
