# An SSH Client

ASSH: 使用Go 开发的SSH客户端，适用于服务器管理、维护和快速使用。当前版本：`v2.0.0`

## 项目状态 & 技术选型

- **Version**: v2.0.0 (Phase 3 开发中 2026/05/09)
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
| 3 | SSH 连接层 | 📝 当前 |
| 4 | CLI 命令组装 | ⏳ |
| 5 | SFTP 文件传输 | ⏳ |
| 6 | 云同步 (Qiniu) + AuthService | ⏳ |
| 7 | 代理 & 隧道 | ⏳ |

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

## 开发命令

- **编译构建**: `cd asshv2 && sh build.sh`
- **单平台构建**: `cd asshv2 && sh build.sh <os> <arch> <alias>`
- **运行**: `cd asshv2 && go run . <命令>`
- **测试**: `cd asshv2 && go test ./... -v`
- **静态检查**: `cd asshv2 && go vet ./...`

## 文档

- [项目需求文档](docs/demand.md) - 项目需求原始文档(草稿)
- [项目重构计划](docs/plan.md) - v2.0.0 重构计划
- [开发任务](docs/TODO.md) - 分阶段任务清单
- [Bug 追踪](docs/bugs.md) - 已知问题和修复记录

## 只读区域

以下目录为项目旧代码和备份，**禁止修改、创建或删除** 任何文件，仅允许阅读：
- `assh/` - 旧版代码
- `backup/` - 备份文件
