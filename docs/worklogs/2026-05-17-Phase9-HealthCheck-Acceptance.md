# Phase 9 服务器健康检查 — 验收报告

## 概述

Phase 9 实现服务器健康检查功能：通过 SSH 连接服务器，采集连接延迟、系统运行时间、CPU 负载、内存使用和磁盘使用等健康指标，支持单机/多机/分组检查，支持并发执行和详细/表格两种输出模式。

## 验收清单

| # | 验收场景 | 命令 | 预期 | 状态 |
|---|---------|------|------|------|
| 1 | 单服务器详细检查 | `assh health check <server>` | 详细健康指标输出 | ✅ |
| 2 | 直接连接检查 | `assh health check root@host:22` | 详细健康指标输出 | ✅ |
| 3 | 多服务器逗号分隔 | `assh health check srv1,srv2` | 表格输出 + 统计摘要 | ✅ |
| 4 | 分组列表 | `assh health list --group web` | 组内服务器表格输出 | ✅ |
| 5 | 全量列表 | `assh health list` | 所有服务器表格输出 | ✅ |
| 6 | 详细模式列表 | `assh health list --detail` | 详细健康指标输出 | ✅ |
| 7 | 并发控制 | `assh health list --concurrency 5` | 并发执行，信号量控制 | ✅ |
| 8 | 超时处理 | 不可达服务器 | 显示 timeout 状态 | ✅ |
| 9 | 错误处理 | 连接失败服务器 | 显示 error 状态 | ✅ |
| 10 | 分组+详细 | `assh health list --group web --detail` | 分组内详细健康指标 | ✅ |
| 11 | 统计摘要 | 表格模式 | 底部汇总 healthy/unhealthy/timeout/error 数量 | ✅ |

## 实现文件

| 文件 | 职责 | 行数 |
|------|------|------|
| `asshc/port/health.go` | HealthChecker 接口 + DTO + HealthStatus 枚举 | ~50 |
| `asshc/infra/ssh/health.go` | SSH 健康检查实现（连接延迟 + 系统信息采集） | ~170 |
| `asshc/service/health.go` | HealthService 编排层（单机/分组/全量 + 并发控制） | ~110 |
| `cmd/health.go` | health CLI 命令（check/list + 表格/详细输出） | ~250 |
| `main.go` | DI 注入 | 修改 |
| `cmd/app.go` | App 结构体注册 | 修改 |

## 设计要点

### HealthChecker 接口定义（port/health.go）

```go
type HealthStatus int
const (
    StatusHealthy   HealthStatus = iota  // 所有指标正常
    StatusUnhealthy                       // 部分指标异常
    StatusTimeout                         // 连接超时
    StatusError                           // 连接错误
)

type HealthDetails struct {
    Uptime   string  // uptime -p
    LoadAvg  string  // /proc/loadavg
    Memory   string  // free -h
    Disk     string  // df -h /
}

type HealthResult struct {
    ServerName string
    Status     HealthStatus
    Latency    time.Duration
    Details    *HealthDetails  // 可为 nil（连接失败时）
    Err        error           // 仅 StatusError/StatusTimeout 时非 nil
}

type HealthChecker interface {
    Check(server *domain.Server, detail bool) (*HealthResult, error)
}
```

### 远程信息采集策略

- 通过 SSH session.Run() 执行 4 个命令：`uptime -p`、`cat /proc/loadavg`、`free -h`、`df -h /`
- 单个命令失败**静默跳过**，不阻断其他命令
- 所有命令都失败时返回 Unhealthy 状态
- 连接延迟通过 `time.Now().Sub(start)` 在 Connect 后测量

### 并发控制（service/health.go）

- HealthService 使用 `semaphore` channel 模式（与 Phase 8 同步相同）
- `CheckAll` 遍历所有服务器，`CheckGroup` 过滤分组
- `CheckServers` 接受指定列表，支持直接连接地址解析（`root@host:22` 格式）

### CLI 命令设计

```
assh health check <server>[,server2,...]  — 单机/多机检查
assh health list [--group <group>] [--concurrency <n>] [--detail]  — 批量检查
```

- 输出模式：默认表格模式（含统计摘要），`--detail` 切换为详细模式
- 表格模式表头：Server | Status | Latency | Uptime | Load | Memory | Disk
- 统计摘要：`Summary: X healthy, Y unhealthy, Z timeout, W error`

## Bug 修复

| 问题 | 文件 | 描述 | 修复 |
|------|------|------|------|
| 死代码 | `cmd/health.go` | `printHealthResults` 中 detail=true 时 `return` 后存在不可达的 `if detail` 分支 | 重构为两个独立函数 `printHealthResults`（表格）和 `printHealthResultsDetailed`（详细），移除无效分支 |

## 验证结果

| 检查项 | 结果 |
|--------|------|
| `go build ./...` | ✅ 通过 |
| `go vet ./...` | ✅ 通过 |
| `go test ./...` - 7 包 | ✅ 全部通过 |

## 文档更新

| 文档 | 更新内容 |
|------|----------|
| `docs/TODO.md` | Phase 9 标记完成，添加命令使用示例 |
| `AGENTS.md` | 项目状态更新（Phase 9 完成时间），已完成模块列表追加健康检查模块 |
