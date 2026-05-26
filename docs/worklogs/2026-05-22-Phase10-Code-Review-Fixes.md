# Phase 10 代码审查修复报告

> 日期: 2026-05-22
> 分支: dev/v2.0.1-phase-10
> 提交: 4702046

## 背景

P10 阶段代码审查发现 3 个需要修复的问题（1 个严重 P0、1 个 P0 遗漏、1 个 P1 不完善）。

## 修复详情

### 1. [P0] ConnectChain 资源泄漏

**问题**: `ConnectChain` 直接返回 `*ssh.Client`，跳板机的 SSH 连接从未关闭。

**方案演变**:
1. ❌ 初案: 使用 `ChainClient` 包装类型（已定义但未使用），通过类型断言在 `Close` 中级联关闭
2. ❌ 编译失败: Go 不允许将 `*ChainClient`（嵌入 `*ssh.Client` 的结构体指针）作为 `*ssh.Client` 返回
3. ✅ 终案: `Connector` 内部使用 `chainParents map[*ssh.Client][]*ssh.Client` 追踪跳板链
   - `ConnectChain` 注册 `targetClient → parents` 映射
   - `Close` 时查表级联关闭（目标→最后一跳→...→第一跳）
   - 移除了未使用的 `ChainClient` 死代码

**文件**: `asshc/infra/ssh/client.go`

### 2. [P0] ConnectByName Options["jump_hosts"]

**问题**: `ConnectByName` 未检查 `server.Options["jump_hosts"]`，用户无法通过配置实现自动跳板。

**修复**: 新增 `getJumpHostsFromOptions()` 辅助函数，`ConnectByName` 在直连前检查配置。

```go
// 使用方式:
assh set internal-web -o jump_hosts=bastion,gateway
assh login internal-web  // 自动走跳板链
```

**文件**: `asshc/service/connect.go`

### 3. [P1] RecordJumpChain 认证信息不完整

**问题**:
- 所有跳板硬编码为 `Type: "server"`，无法区分 `direct` 类型
- 未记录 `Password`/`KeyFile` 认证信息

**修复**: 根据 `hop.Name` 字段自动判断类型：
- `hop.Name != ""` → `type: "server"`, `ServerRef: hop.Name`
- `hop.Name == ""` → `type: "direct"`
- 同时记录 `hop.Auth.Password`/`KeyFile`

**文件**: `asshc/service/jump.go`

## 验证结果

| 项目 | 结果 |
|------|------|
| `go build ./...` | ✅ 通过 |
| `go test ./...` (7 包) | ✅ 全部通过 |
| `go vet ./...` | ✅ 无警告 |

## 文档对齐清单

- [x] `docs/TODO.md` — P10.9 标记 ✅ + P10.10/P10.11 新增
- [x] `AGENTS.md` — 状态已同步（P1 完成）
- [ ] `docs/adrs.md` — 无需更新（不属于架构变更）
- [x] 本工作报告

## Git 日志

```
4702046 fix: P10.10 跳板机 P0/P1 修复 — 资源泄漏/Options跳板/历史认证
```
