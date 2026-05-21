# Changelog

## v2.0.1 (2026-05-22)

### 安全审计修复 (Batch 1/2/4)

**来源**: [安全审计报告](docs/securitys/security-audit-2026-05-22.md) | **分支**: `hotfix/security-audit-2026-05-22`

#### P0 CRITICAL (4项)
- 修复 `RollbackTo()` 快照前密码明文写入Changelog的问题 (AES-GCM加密)
- 修复 SFTP Shell `!` 命令注入 (白名单验证 `allowedLocalCmds`)
- 修复 Host Key 验证绕过 (insecure 模式下添加warn日志)
- 新增 Argon2id KEK 保护 DEK 加密密钥 (key_derivation.go)

#### P1 HIGH (7项)
- 修复服务器名路径遍历 (`sanitizeLogFilename` 路径清理)
- PKCS1v15 导出标记 Deprecated (建议OAEP)
- 敏感内存清零 (`Store.Close()`/`Connector.ClearSensitiveData()`/`KeyManager.ClearSensitiveData()`)
- 修复读锁写入 cryptoKey (改用 `atomic.Value` + `sync.Once`)
- JumpRecorder 添加完整锁保护
- 新增密钥轮换框架 (`key_version` + `reencrypt` 命令)

#### P2 MEDIUM (9项)
- 移除死代码: 重复SFTP handler (~155行)、`reconnectLoop`、`expandPathSimple`、`keyTypeDefaultFilenames`
- `os.Exit(0)` → `cli.NewExitError` 正确错误处理
- CBC padding oracle 错误信息统一模糊化
- 新增 GCM AAD context 绑定 (`EncryptGCMWithAAD`/`DecryptGCMWithAAD`)
- 无盐 SHA256 改用 HMAC-SHA256 (认证指纹)
- `closeAndLog` 辅助函数处理 defer 错误
- 15 个编译期接口合规检查 (`var _` 断言)
- 全量添加 `log.Warnf` 记录静默忽略的错误

### Phase 10: 跳板机支持 (Jump Host)

**完成时间**: 2026-05-22 | **ADR**: ADR-030 | **设计**: [Phase10-Design.md](docs/Phase10-Design.md)

#### 领域层 (`asshc/domain/`)
- `jump.go` — JumpHistory 领域实体 + JumpHopAuth 类型

#### 接口层 (`asshc/port/`)
- `jump.go` — JumpHistoryRecorder 接口 (Record/List/Get/Delete/Clear/IncrementUse)
- `connector.go` — 新增 ConnectChain(target, chain) 多跳连接方法

#### 基础设施层
- `asshc/infra/ssh/client.go` — ConnectChain 多跳SSH实现 (`chainClient` + `dialThrough`)
  - 每跳 Keepalive 心跳
  - 带索引的错误报告 (`"jump[1] gateway: ..."`)
  - 级联关闭父连接 (资源泄漏修复)
- `asshc/infra/store/jump.go` — JumpRecorder CRUD + AES-GCM 加密存储
- `asshc/infra/store/db.go` — jump_history 表迁移

#### 服务编排层 (`asshc/service/`)
- `connect.go` — ConnectChain 服务 + parseJumpChain 解析
  - Options 默认跳板 (`Options["jump_hosts"]`)
  - 配置隔离规则 (CLI flags 不传给跳板机)
  - `-J none` 禁用跳板
  - IPv6 方括号解析 (`[::1]`)
  - `-J @id` 历史跳板链复用
- `jump.go` — JumpService 跳板历史管理 (RecordJumpChain/List/Delete/Clear)

#### CLI 命令 (`cmd/`)
- `connect.go` — login/run/bc 全部支持 `-J/--jump` 参数
- `sftp_cmd.go` — push/pull 支持 `-J` (assh-fs)
- `proxy.go` — proxy 支持 `-J`
- `tunnel.go` — tunnel 支持 `-J`
- `jump.go` — `assh jump history` 命令 (list/--clear/--delete)

#### DI 注入
- `main.go` — JumpService + JumpHistoryRecorder 注入
- `cmd/fs/main.go` + `cmd/bootstrap.go` — assh-fs 跳板支持

#### 验证
- 编译 ✅ vet ✅ 测试 ✅ 竞态检测 ✅
- P0 (11任务) + P1 (7任务) 全部完成

### Phase 9: 服务器健康检查

**完成时间**: 2026-05-17

#### 接口层 (`asshc/port/`)
- `health.go` — HealthChecker 接口 + HealthResult/HealthDetails DTOs + HealthStatus 枚举

#### 基础设施层
- `asshc/infra/ssh/health.go` — SSH 健康检查实现 (连接延迟 + 系统信息采集: uptime/load/memory/disk)

#### 服务编排层 (`asshc/service/`)
- `health.go` — HealthService 编排层 (CheckServer/CheckGroup/CheckAll/CheckServers + 并发)

#### CLI 命令
- `cmd/health.go` — health check/list 命令 (单机/批量/分组巡检, `--detail` 系统信息, `--concurrency`)

#### DI 注入
- `main.go` + `cmd/app.go` — HealthChecker + HealthService 创建注入

#### 验证
- 编译 ✅ vet ✅ 测试 ✅

### Phase 8: 云同步 (Qiniu)

**完成时间**: 2026-05-16

#### 接口层 (`asshc/port/`)
- `syncer.go` — Syncer 接口 + DTOs (SyncAccount/SyncData/ServerData/SyncResult/SyncObject)

#### 数据层 (`asshc/infra/store/`)
- `db.go` — cloud_accounts + sync_history 表迁移
- `account.go` — 云账户 CRUD + AES-GCM 加密存储

#### 基础设施层
- `asshc/infra/sync/qiniu/qiniu.go` — SyncObject 实现 port.Syncer (Login/Push/Pull/List/Delete)

#### 服务编排层 (`asshc/service/`)
- `sync.go` — SyncService (Push/Pull 合并逻辑, 冲突保留双方策略)

#### CLI 命令
- `cmd/sync.go` — sync account set/show/delete/test + push/pull/history

#### 验证
- 编译 ✅ vet ✅ 测试 ✅

## v2.0.0-dev (2026-05-13)

### Phase 7: 代理 & 隧道 (Smart Proxy 系统)

#### 基础设施层 (`asshc/infra/proxy/`)
- `socks5.go` — SOCKS5 代理 (RFC 1928 NO AUTH + RFC 1929 USERNAME/PASSWORD)
- `http_connect.go` — HTTP CONNECT 代理 (RFC 7231, Basic Auth)
- `port.go` — LocalForward / RemoteForward 端口转发
- `tunnel_manager.go` — TunnelManager (Add/Remove/Get/List/StopAll + StartAll + 重连循环 + daemon)
- `rules.go` — AutoProxy 规则引擎 (GFWList 兼容, CIDR/域名/正则/白名单)
- `logger.go` — 透明日志系统 (JSONL 格式, 按日轮转, 批次写入)
- `listener.go` — Smart Proxy 流水线编排 (SOCKS5+HTTP 统一监听, 规则引擎, 日志)

#### 接口层 (`asshc/port/`)
- `rules.go` — RuleEngine 接口 (Match/Load/Reload/DefaultAction)
- `logger.go` — ProxyLogger 接口 (LogRequest/Close) + RequestLog 结构体

#### 服务编排层 (`asshc/service/`)
- `proxy.go` — ProxyService (StartProxy/StartDirectProxy/StopProxy/StartTunnel/StopTunnel/ListTunnels/ReloadRules)
- `proxy_ports.go` — ParsePorts 多端口规则解析 (范围展开/截断/多对一/一对多)

#### CLI 命令 (`cmd/`)
- `proxy.go` — proxy 命令 (正向/反向/直连/AutoProxy/日志/daemon/reconnect)
- `tunnel.go` — tunnel 命令 (start/stop/list, 多 -L/-R)

#### DI 注入
- `main.go` — TunnelManager + ProxyService 创建注入
- `cmd/app.go` — App 新增 proxySvc 字段,注册 proxy/tunnel 命令

#### 测试
- 46 个新增单元测试 (RuleEngine × 12, ProxyLogger × 3, SmartProxy × 9, ParsePorts × 22)
- 全部通过: `go build ./... && go test ./...`

#### ADR
- ADR-028: 代理&隧道基础架构 (accepted)
- ADR-029: Smart Proxy 智能代理系统 (accepted)

### Phase 6: 密钥管理 (Key Management)

#### 接口层 (`asshc/port/`)
- `keymanager.go` — KeyManager 接口 (Generate/GenerateToPath/DeployPublicKey/Backup/Lookup)
- `repository.go` — 新增 KnownServerRecorder 接口

#### 基础设施层 (`asshc/infra/keymgr/`)
- `generate.go` — 密钥生成 (RSA 2048/4096, Ed25519, ECDSA P256/P384/P521; OpenSSH 格式; 指纹 SHA256)
  - `Generate` — 生成到默认路径 `~/.ssh/`
  - `GenerateToPath` — 生成到指定路径 (`-f`/`--output`)
  - 支持 passphrase 加密私钥
- `backup.go` — 密钥备份 (指纹命名 → 复制 → data/keys/ 索引维护)
- 56 个单元测试全部通过

#### 服务编排层 (`asshc/service/`)
- `deploy.go` — DeployService (公钥部署到 `~/.ssh/authorized_keys`, 幂等实现)
- `key.go` — KeyService (GenerateAndDeploy / HandleKeyFlag 用例编排)
- 16 个单元测试全部通过

#### CLI 命令 (`cmd/`)
- `keygen.go` — `assh keygen` 命令，三种模式：
  - `assh keygen <server>` — 生成密钥 → 部署到服务器
  - `assh keygen -f <path>` — 独立生成到指定路径
  - `assh keygen` (无参) — 交互式向导
  - ssh-keygen 参数对齐: `-f`/`-C comment`/`-N passphrase`/`-t type`/`-b bits`

#### 数据层 (`asshc/infra/store/`)
- `known.go` — known_servers 建表 + upsert/lookup 实现 (直连记录, 密钥路径关联)

#### SSH 集成 (`asshc/infra/ssh/`)
- `client.go` — 备份优先策略 (tryBackupKey: 先尝试 data/keys/ 备份密钥)

#### DI 注入
- `main.go` — KeyManager + KeyService 创建注入; ConnectService KeyBackupPath 传递
- `cmd/app.go` — App 新增 keySvc 字段, 注册 keygen 命令

#### 架构决策
- ADR-025: ssh-keygen 参数对齐 (`-f`/`-C`/`-N` 全栈透传)
- ADR-026: SSH 密钥备份优先策略 (data/keys/ → ~/.ssh/)
- ADR-027: 简化 SSH 认证策略 (去除两轮回退)

**完成时间**: 2026-05-13 | **验证**: 编译 ✅ vet ✅ 72 个测试全部通过 ✅

### Phase 5: SFTP 文件传输

#### 接口层 (`asshc/port/`)
- `transfer.go` — FileTransfer 接口 (Push/Pull/List/Remove/Mkdir + PushBatch/PullBatch)
  - `TransferProgress` 回调、`TransferResult`、`FileInfo`

#### 基础设施层 (`asshc/infra/sftp/`)
- `client.go` — SFTPSession 封装 (创建/关闭/重连; 并发读写 MaxConcurrentRequestsPerFile=64)
- `upload.go` — Push 实现 (单文件/目录递归 `-r`/断点续传 `--resume`/Hash Header/Glob 匹配/进度回调)
- `download.go` — Pull 实现 (同上对称)
- `header.go` — 256B Hash Header (Magic:4B + Version:2B + HashType:2B + OrigSize:8B + SHA256:64B + Reserved:176B)
- `ops.go` — 远程操作 (List/Remove/Mkdir)
- `progress.go` — 进度条显示 (百分比/速率/ETA/传输字节)
- `verify.go` — 传输后验证 (大小比对始终执行, SHA256 可选 --checksum)

#### 服务编排层 (`asshc/service/`)
- `transfer.go` — TransferService (PushFile/PullFile/ListRemote/RemoveRemote/MkdirRemote)
  - 覆盖策略: 询问/--force强制/--skip跳过
  - 多文件 goroutine 池并发 (默认 3)

#### CLI 命令 (`cmd/`)
- `sftp_cmd.go` — push/pull/交互式 FS 命令定义 (assh + assh-fs 共享)
  - `assh-fs push <server> <local> <remote> [-r] [--resume] [-f/--skip] [--checksum]`
  - `assh-fs pull <server> <remote> [local] [-r] [--resume]`
  - `assh-fs <server>` — 交互式 SFTP 会话 (ls/cd/get/put/chmod/...)
  - 直连参数: `-H/-u/-p/-P/-i/-k`
- `fs/main.go` — assh-fs 独立入口 (DI 组合根)
- `bootstrap.go` — NewAppComponents() 公共初始化逻辑

#### CLI 增强
- `main.go` — expandCombinedFlags() 组合短参数预处理 (RQ-001: `-qnt` → `-q -n -t`)
- SSH 认证回退: `asshc/infra/ssh/client.go` — key 失败后 password-only 回退 (BUG-010)

#### 验证
- 28 项验收测试全部通过 (F1-F28)
- 编译 ✅ vet ✅ 测试 ✅
- ADR-017~024 记录架构决策

**完成时间**: 2026-05-11 | **验收**: F1-F28 全部通过 ✅

### Phase 4: CLI 命令组装

- `cmd/server.go` — 服务器管理命令
  - `server add` — 添加服务器（`-H/-p/-u/-l/-P/-i/-k/-o/--remark/--group`）
  - `server set` — 创建/更新服务器参数(upsert)，支持参数验证（`-H/-p/-u/-l/-P/-i/-k/-o/--remark/--clear-password/--clear-key`）
  - `server ls` — 列表服务器（`--group`/`--search` 筛选）
  - `server info` — 查看服务器详情（含 Version）
  - `server rm` — 删除服务器
  - `server mv` — 重命名/移动服务器
  - `server rollback` — 回滚服务器配置（`<name> <version>` 按版本回滚，`--list` 查看变更历史）
- `cmd/connect.go` (301行) — 连接管理命令
  - `login` — SSH 登录（自动识别：`user@host` / `-H host` / 纯 name）
  - `run` — 远程执行命令
  - `bc` — 批量执行（三通道：实时日志 + stdout 标头 + JSON 汇总，goroutine 并发）
- `cmd/app.go` — 全局参数 `-v/--verbose`，`-q/--quiet`，`-F/--config`；`HideVersion` 解决 `-v` 冲突
- `main.go` — 完整 DI 注入（Store + SSH + ServerService + ConnectService）
- `asshc/domain/` — 新增 `ChangelogEntry` 类型，`Version` 字段加入 `Server` 结构体
- `asshc/port/repository.go` — 新增 `GetChangelog` / `RollbackTo` 接口方法
- `asshc/infra/store/` — 数据库迁移: `version` 列 + `server_changelog` 表；`Set` 自动版本化+快照记录；`RollbackTo` 快照回滚
- ADR-012: CLI 参数对齐与命令架构（参数映射表、bc 三通道、自动识别规则）
- 验证: 编译 ✅ vet ✅ 测试 ✅（7 包全部通过） help ✅ version ✅

### Phase 3: SSH 连接层

- `asshc/port/connector.go` — SSHConnector 接口定义（Connect/Close）
- `asshc/port/session.go` — SSHSession 接口定义（Shell/Run/RunWithOutput）
- `asshc/port/hostkey.go` — HostKeyChecker 接口定义（预留）
- `asshc/infra/ssh/` — SSH 连接实现层
  - `client.go` — SSH 客户端（认证链 Agent→私钥→密码、HostKey 策略、Keepalive 心跳）
  - `session.go` — 会话管理（pty+SIGWINCH 交互终端、远程命令执行）
  - `hostkey.go` — HostKey 验证策略（KnownHosts / Insecure）
  - `client_test.go` (7 tests)、`session_test.go` (1 test)、`hostkey_test.go` (2 tests)
- `asshc/service/connect.go` — ConnectService 编排层（ConnectByName/ConnectDirect/Shell/Run/RunWithOutput/Close）
- `asshc/service/connect_test.go` — 16 个测试（mock connector+session，覆盖正常/边界/错误场景）
- 验证: 编译 ✅ vet ✅ 测试 ✅（6 包全部通过）
- Bug 修复: Keepalive 实现 — 将心跳 goroutine 与连接超时分离，修复 Timeout 被覆盖的问题

### Phase 2: 数据持久化层

- `asshc/domain/` — 领域实体（Server/Auth）和错误定义（ErrNotFound/ErrExists/ErrInvalidName/ErrInvalidPort）
- `asshc/port/repository.go` — ServerRepository 接口（8 个方法：List/Get/Set/Delete/Move/Search/GetGroup/Close）
- `asshc/infra/store/` — SQLite 持久化层
  - `db.go` — Store 结构体 + 自动建表（servers + config 表）
  - `key.go` — AES-256-GCM 密钥自动生成/存储/加载；密码加解密
  - `server.go` — 完整 CRUD 实现（List/Get/Set/Delete/Move/Search/GetGroup）
- `asshc/service/server.go` — ServerService 用例编排（Add/Update/Remove/Move/Search/List/Get/GetGroup）
- 验证: 17 个测试全部通过，sqlite3 数据库创建和写入确认

### 日志系统简化

- 移除 dual log 输出（errorLogger），改为单一日志输出
- 移除日志级别过滤，所有级别（DEBUG/INFO/WARN/ERROR/PANIC/FATAL）均输出
- 移除 config/path.go 中的 ErrorLogPath 常量
- 清理 dead code：移除 formatLogLevel 函数及其测试

### CLI 参数优化

- 移除 -llv（日志级别）参数
- 保留 --log 参数用于指定日志路径
- 日志路径优先级：CLI --log > config.GetLogPath() > /tmp/assh.log

## v2.0.0-dev (2026-05-08)

### Phase 1: 基础设施移植

- `log/` — 基于 zerolog 的日志模块移植
  - 7 级日志（OFF/FATAL/PANIC/ERROR/WARN/INFO/DEBUG）
  - 文件输出 / stderr 输出
  - 28 个测试用例全部通过
- `config/` — 配置管理模块移植
  - 路径管理（ExpandPath、EnsureDir、FileExists）
  - 配置读写接口（日志、数据库、七牛云、密码）
  - 优化：移除 config 对 keygen 的依赖（crypto 移至 infra 层）
- `asshc/infra/crypto/` — 加解密模块移植
  - RSA 密钥生成、PEM 解析、加解密（PKCS#1.5 / OAEP）
  - AES 加解密（CBC / CTR / GCM / ECB）
  - Ed25519 密钥生成
  - ECDSA 密钥生成（P256 / P384 / P521）
  - 包名从 `keygen` 改为 `crypto`，移至 `asshc/infra/crypto/`
- `asshc/port/cryptor.go` — Cryptor 接口定义

### Phase 0: 项目骨架搭建

- 目录结构创建（asshc/domain、port、service、infra/ 分层）
- `build.sh` — 构建脚本（多平台交叉编译）
- `main.go` — 程序入口（DI 组合根）
- `cmd/app.go` — CLI 框架（urfave/cli）

### 架构变更

- 引入 `asshc/` 核心业务层，采用 port/service/infra 三层架构
- `service/` 仅依赖 `port/` 接口，不直接依赖具体实现
- 所有依赖通过构造器注入
- 移除 `config/` 中的加密方法，移至 `asshc/infra/crypto/`
