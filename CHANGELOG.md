# Changelog

## v2.0.0-dev (2026-05-09)

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
