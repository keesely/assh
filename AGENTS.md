# An SSH Client

ASSH: 使用Go 开发的SSH客户端，适用于服务器管理、维护和快速使用。当前版本：`v2.0.0`

## 项目状态 & 技术选型

- **Version**: v2.0.0 (Phase 4 完成, Phase 5 待开始 2026/05/09)
- **上一稳定版本**: v2.0.0 (Phase 1 完成)
- **Go Version**: 1.22.0+
- **Encrypt**: RSA + AES
- **数据库**: SQLite (modernc.org/sqlite, 纯 Go 无 CGO)
- **日志库**: zerolog v1.32.0
- **Cli 框架**: urfave/cli v1.22.2

## 工作区间

主要工作目录为 `asshv2/`，通过 `git worktree` 管理，指向 `assh/.git/worktrees/asshv2`。
**所有开发工作均在 `asshv2/` 内进行**，该目录包含完整的 Go 项目结构。
主仓库 `assh-builder/` 用于项目级配置管理。

## 项目设计

### 架构分层

采用 **port / service / infra** 三层架构：

```
cmd/ (CLI 命令)
  └── service/ (用例编排, 依赖 port 接口)
        ├── port/ (接口定义, 依赖倒置)
        └── infra/ (接口实现, 依赖具体库)
```

关键约束：
- `service/` 只依赖 `port/` 接口 + `domain/` 实体，不直接依赖 `infra/`
- 所有依赖通过构造器注入（组合根在 `main.go`）

### 项目结构

```
asshv2/
├── main.go              # 程序入口（DI 组合根）
├── cmd/                 # CLI 命令层
├── asshc/               # 核心业务
│   ├── domain/          #   领域实体
│   ├── port/            #   接口定义
│   ├── service/         #   用例编排
│   └── infra/           #   接口实现
├── config/              # 配置管理
├── log/                 # 日志模块
├── README.md            # 用户手册
└── CHANGELOG.md         # 更新日志
```

### 开发阶段 (Phase 0-7)

| Phase | 内容 | 状态 |
|-------|------|------|
| 0 | 项目骨架搭建 | ✅ 完成 |
| 1 | 基础设施移植 (log/config/crypto) | ✅ 完成 |
| 2 | 数据持久化层 (SQLite + AES-GCM) | ✅ 完成 |
| 3 | SSH 连接层 | ✅ 完成 |
| 4 | CLI 命令组装 | ✅ 完成 |
| 5 | SFTP 文件传输 | ⏳ |
| 6 | 代理 & 隧道 (SOCKS5/端口转发/SSH隧道) | ⏳ |
| 7 | 云同步 (Qiniu) | ⏳ |
| 8 | 服务器健康检查 | ⏳ |
| 9 | 跳板机/堡垒机支持 | ⏳ |
| 10 | 远程服务器挂载 (sshfs/webdav/samba) | ⏳ |
| 11 | 自动更新/升级 | ⏳ |
| X | 实验阶段：虚拟命令投放与执行 | ⏳ |

### 分支命名规范

```
dev/v2.0.0-phase-X  # 阶段开发分支 (如 dev/v2.0.0-phase-2)
feat/<feature>      # 特性分支 (如 feat/config, feat/ssh)
bugfix/<issue>     # Bug 修复分支
```

详细任务见 [docs/TODO.md](docs/TODO.md)。

### 已完成模块

- [x] `log/` — 日志模块（zerolog 实现）
- [x] `config/` — 配置管理模块（config.go + path.go）
- [x] `asshc/infra/crypto/` — 加解密实现（RSA/AES/Ed25519/ECDSA）
- [x] `asshc/port/cryptor.go` — Cryptor 接口
- [x] `cmd/app.go` — CLI 框架
- [x] `asshc/domain/` — 领域实体（Server/Auth）和错误定义
- [x] `asshc/port/repository.go` — ServerRepository 接口
- [x] `asshc/infra/store/` — SQLite 持久化（建表/CRUD/AES-GCM 加密）
- [x] `asshc/service/server.go` — ServerService 用例编排
- [x] `asshc/port/connector.go` — SSHConnector 接口
- [x] `asshc/port/session.go` — SSHSession 接口
- [x] `asshc/port/hostkey.go` — HostKeyChecker 接口（预留）
- [x] `asshc/infra/ssh/` — SSH 连接实现（认证链/HostKey/Keepalive/Shell/Run）
- [x] `asshc/service/connect.go` — ConnectService（连接/会话编排）
- [x] `asshc/service/connect_test.go` — 16 个测试用例覆盖 mock 测试
- [x] `cmd/server.go` — 服务器管理命令（add/set/ls/info/rm/mv/rollback）
- [x] `cmd/connect.go` — 连接命令（login/run/bc）含 bc 三通道输出
- [x] `main.go` — 完整 DI 注入（Store + SSH + Service）
- [x] `cmd/app.go` — 全局参数 `-v/-q/-F` 注册 + 子命令注册
- [x] `asshc/domain/changelog.go` — ChangelogEntry 类型、变更类型常量
- [x] `asshc/infra/store/` — 版本管理 + server_changelog 表 + 快照回滚

## 开发命令

- **编译构建**: `cd asshv2 && sh build.sh`
- **单平台构建**: `cd asshv2 && sh build.sh <os> <arch> <alias>`
- **运行**: `cd asshv2 && go run . <命令>`
- **测试**: `cd asshv2 && go test ./... -v`
- **静态检查**: `cd asshv2 && go vet ./...`

## 工作流规则

- **自动编译检查**: 每次修改代码文件后（包括 PM Agent 直接编辑或 build agent 执行），必须立即执行 `cd asshv2 && go build ./...` 确认编译通过，否则不得进入下一步。

## 文档

- [项目需求文档](docs/demand.md) - 项目需求原始文档(草稿)
- [项目重构计划](docs/plan.md) - v2.0.0 重构计划
- [开发任务](docs/TODO.md) - 分阶段任务清单
- [Bug 追踪](docs/bugs.md) - 已知问题和修复记录

## 只读区域

以下目录为项目旧代码和备份，**禁止修改、创建或删除** 任何文件，仅允许阅读：
- `assh/` - 旧版代码
- `backup/` - 备份文件
