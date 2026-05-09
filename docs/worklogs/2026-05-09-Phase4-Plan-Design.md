# Phase 4 规划设计方案工作报告

> 日期: 2026-05-09
> 分支: dev/v2.0.0-phase-4
> 作者: PM Agent

---

## 1. 工作概览

本次工作完成了 Phase 4 (CLI 命令组装) 从需求确认到方案设计的完整流程：

| 阶段 | 步骤 | 成果 |
|------|------|------|
| 需求确认 | 6 轮逐项问卷 | 确认所有 CLI 规格 |
| 术语定义 | 术语表 | 统一项目术语 |
| 架构决策 | ADR-012 | CLI 参数对齐与命令架构 |
| 方案设计 | Phase4-Design.md | 完整设计文档 |
| 文档对齐 | 7 个文档同步 | 全部更新 |

---

## 2. 需求确认过程

### 2.1 问卷清单

| # | 问题 | 结论 |
|---|------|------|
| Q1 | server add 密码输入 | 支持 `-P/--password` 明文 / `-k/--key` 密钥 / 无则交互提示 |
| Q2 | login 直连格式 | 自动识别：`user@host` / `-H -u` 分离参数 / 纯 name 查找 |
| Q3 | bc 输出格式 | 三通道：实时日志 + stdout 标头 + JSON 汇总 + `--log` 自定义 |
| Q4 | Options 配置 | 独立 flag (优先) + `-o/--option` 通用格式 |
| Q5 | 配置路径 | `-F/--config` 参数 > 环境变量 > 默认路径 |
| Q6 | 参数对齐 ssh | 完整参数映射表，兼容 ssh 命名 |

### 2.2 参数对齐映射表

最终确认的参数映射（已对齐 Linux ssh）：

| assh 参数 | ssh 对应 | 语义 |
|-----------|----------|------|
| `-p/--port` | `-p` | 端口 |
| `-u/-l/--user/--login` | `-l` | 用户名 |
| `-i/--identity-file` 别名 `-k/--key` | `-i` | 密钥文件 |
| `-P/--password` | assh 特有 | 密码 |
| `-o/--option` | `-o` | 配置项 |
| `-v/--verbose` | `-v` | 详细输出 |
| `-q/--quiet` | `-q` | 静默模式 |
| `-H/--host` | assh 特有 | 主机地址 |
| `-c/--connect-timeout` | `-o ConnectTimeout` | 连接超时 |
| `-F/--config` | `-F` | 配置文件 |

---

## 3. 架构决策 (ADR-012)

**ID**: ADR-012  
**标题**: CLI 参数对齐与命令架构  
**状态**: accepted  

核心决策：
1. **参数兼容对齐** — 主力参数与 ssh 一致，assh 特有参数合理命名
2. **混合命令层次** — server 子命令集 + login/run/bc 顶级命令
3. **login 自动识别规则** — `@`符号 / `-H` / 纯名称三路判定
4. **bc 三通道输出架构** — 实时日志 + stdout 标头 + JSON 汇总
5. **参数作用域划分** — 全局参数 app 级、连接参数命令级

---

## 4. 设计要点

### 4.1 bc 三通道架构

```
bc "uptime" --servers prod.web-01,prod.web-02
         │
         ├── 通道1: 实时日志 /tmp/assh/bclogs/{server}.log
         ├── 通道2: stdout [server] prefix 实时输出
         └── 通道3: JSON 汇总（全部完成后，--log 指定路径）
```

### 4.2 login 自动识别逻辑

```
login [target]
  ├── target 含 "@" → user@host 直连
  ├── 有 -H/--host → 分离参数直连
  └── 纯 name → 数据库按名查找
```

### 4.3 DI 注入变更

```
main.go
  ├── store.NewStore(dbPath) → port.ServerRepository
  ├── service.NewServerService(repo) → *ServerService
  ├── service.NewConnectService(connector, session, repo) → *ConnectService (repo 不再为 nil)
  └── cmd.NewApp(version, build, connectSvc, serverSvc) → *App
```

---

## 5. 文档对齐清单

- [x] `docs/TODO.md` — Phase 4 子任务 15 项细化 + 验收清单 10 条
- [x] `docs/plan.md` — 无需更新（已反映 Phase 4）
- [x] `AGENTS.md` — 状态更新、模块清单、Phase 4 任务
- [x] `docs/architecture.md` — 无需更新（DI 图已完整）
- [x] `docs/adrs.md` — ADR-012 索引 + Phase 4 摘要
- [x] `docs/adrs/012-CLI参数对齐与命令架构.md` — 新建
- [x] `docs/Phase4-Design.md` — 新建（完整设计文档）
- [ ] `docs/modules/cli.md` — 待 Phase 4 实现时同步更新
- [x] `docs/worklogs/2026-05-09-Phase4-Plan-Design.md` — 本文件

---

## 6. Git 操作记录

```
assh-builder/ (master):
  f4ff2b4 docs: mark Phase 3 as completed, update status and set Phase 4 as current

asshv2/ (dev/v2.0.0-phase-3 → master → dev/v2.0.0-phase-4):
  21a7ee1 docs: add Phase 3 SSH connection layer to CHANGELOG, fix .gitignore
  6cb05cb chore: merge Phase 3 SSH connection layer into master (merge)
  v2.0.0-phase-3 tag created
  dev/v2.0.0-phase-4 branch created from master
```

---

## 7. 后续步骤建议

1. **实现 P4.1** — `cmd/server.go` (server add/ls/info/rm/mv)
2. **实现 P4.2** — `cmd/connect.go` (login/run/bc)
3. **修改 P4.3** — `cmd/app.go` 命令注册
4. **修改 P4.4** — `main.go` DI 注入
5. **验证** — 全量编译 + 测试 + CLI 可用性
