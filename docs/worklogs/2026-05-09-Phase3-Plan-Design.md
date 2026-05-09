# Phase 3 设计工作日志

> 日期: 2026-05-09
> 工作内容: Phase 3 (SSH 连接层) 需求分析 + 架构设计 + 方案定稿

---

## 一、分析过程

### 1.1 审查现有代码和文档

- 阅读 `docs/TODO.md` — Phase 3 原始任务规划
- 阅读 `docs/plan.md` — 技术选型和项目结构
- 阅读 `AGENTS.md` — 项目状态和已完成的模块
- 阅读已完成模块的源码：
  - `asshc/domain/server.go` — Server/Auth 实体
  - `asshc/port/repository.go` — ServerRepository 接口
  - `asshc/service/server.go` — ServerService 模式参考
  - `asshc/infra/store/` — SQLite 持久化实现模式
- 阅读 v1 旧版 SSH 代码：
  - `assh/use-agent/lib/asshc/client.go` — SSH 客户端封装
  - `assh/use-agent/lib/asshc/shell.go` — 交互式终端
  - `assh/cmd/command.go` — CLI 命令（Connection/BatchCommand）

### 1.2 设计需求确认（通过 question 多轮确认）

| 问题 | 用户决策 | 理由 |
|------|----------|------|
| 接口粒度 | **B — 分离** SSHConnector + SSHSession | 单一职责，独立测试 |
| SSH Agent | **A — 保留** | 保持 v1 用户习惯 |
| HostKey 策略 | **C — 可配置 + hook** | 默认安全，灵活降级，可扩展 |
| Keepalive | **C — 从配置读取** | 灵活适应不同网络环境 |
| Service 层 | **C — 完整封装** | CLI 层零 ssh import |

### 1.3 设计方案演变

**初期考虑的统一接口方案（被否决）：**
```go
type SSHConnector interface {
    Connect(server) (*ssh.Client, error)
    Close(client) error
    Shell(client) error
    Run(client, cmd) error
}
```
- 问题：职责过多，连接与会话生命周期不同，mock 时需实现全部方法

**最终采用的分离接口方案：**
- `SSHConnector` 负责连接生命周期
- `SSHSession` 负责会话操作
- 连接可以复用创建多个会话
- 各接口可独立 mock 测试

---

## 二、创建文档清单

| 文档 | 操作 | 说明 |
|------|------|------|
| `docs/adrs/017-SSH连接层架构设计.md` | ✅ 新建 | ADR-011: SSH 架构决策 |
| `docs/Phase3-Design.md` | ✅ 新建 | 完整设计方案（接口/实现/流程/任务） |
| `docs/modules/server.md` | ✅ 重写 | SSH 模块详细文档 |
| `docs/TODO.md` | ✅ 更新 | Phase 3 任务细化（P3.1-P3.4） |
| `docs/architecture.md` | ✅ 更新 | DI 注入流程增加 SSH 模块 |
| `docs/plan.md` | ✅ 更新 | Phase 3 文件计数修正 |
| `docs/constraints.md` | ✅ 更新 | 新增 term 依赖说明 |
| `docs/adrs.md` | ✅ 更新 | 新增 ADR-011 索引 |
| `AGENTS.md` | ✅ 更新 | 模块状态同步 |

---

## 三、架构变更传导影响分析

### 新增模块

```
asshc/port/
  ├── connector.go    (新增) SSHConnector 接口
  ├── session.go      (新增) SSHSession 接口
  └── hostkey.go      (新增) HostKeyChecker 接口（预留）

asshc/infra/ssh/
  ├── client.go       (新增) SSH 连接实现
  ├── session.go      (新增) 会话管理实现
  ├── hostkey.go      (新增) HostKey 验证实现
  ├── client_test.go  (新增) 连接测试
  ├── session_test.go (新增) 会话测试
  └── hostkey_test.go (新增) HostKey 测试

asshc/service/
  └── connect.go      (新增) ConnectService

main.go               (修改) DI 注入
```

### 依赖关系

```
service/connect.go
  ├── port.SSHConnector  ← infra/ssh/client.go
  ├── port.SSHSession    ← infra/ssh/session.go
  └── port.ServerRepository ← infra/store/server.go
```

### 无影响模块

- `asshc/domain/` — 无需新增领域实体
- `asshc/infra/crypto/` — 无需修改
- `asshc/infra/store/` — 无需修改
- `log/`, `config/` — 无需修改

---

## 四、工作任务总览

| 任务 | 文件 | 工作量估计 | 说明 |
|------|------|-----------|------|
| P3.1-a | `port/connector.go` | 小 (~15 行) | 接口定义 |
| P3.1-b | `port/session.go` | 小 (~20 行) | 接口定义 |
| P3.1-c | `port/hostkey.go` | 小 (~20 行) | 接口定义（预留） |
| P3.2-a | `infra/ssh/client.go` | 中 (~120 行) | 认证链/HostKey/Keepalive |
| P3.2-b | `infra/ssh/session.go` | 中 (~100 行) | Shell/Run/RunWithOutput |
| P3.2-c | `infra/ssh/hostkey.go` | 小 (~60 行) | KnownHosts/Insecure 策略 |
| P3.2-d | `infra/ssh/client_test.go` | 中 (~100 行) | mock SSH server 测试 |
| P3.2-e | `infra/ssh/session_test.go` | 中 (~80 行) | 会话测试 |
| P3.2-f | `infra/ssh/hostkey_test.go` | 小 (~40 行) | HostKey 测试 |
| P3.3-a | `service/connect.go` | 中 (~80 行) | ConnectService |
| P3.3-b | `service/connect_test.go` | 中 (~120 行) | mock 测试 |
| P3.4-a | `main.go` | 小 (~10 行) | DI 注入 |

---

## 五、可复用经验

1. **question 工具用于需求确认非常有效** — 逐项确认避免歧义
2. **v1 代码分析** — 旧版代码是重要的需求来源，特别是 `client.go` 和 `shell.go` 展示了完整的 SSH 认证链和交互终端实现
3. **ADR 先行** — 架构决策先记录 ADR，设计方案再写详细文档，避免返工
4. **分层文档结构** — `phase plan → ADR → module doc` 三层结构清晰

---

## 六、Git 操作记录

```bash
# 当前分支
git branch --show-current
# → branch info (未创建 Phase 3 分支，待 build agent 启动时创建)
```
