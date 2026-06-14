# BUG-016: tunnel stop 无法停止 daemon 模式隧道

## Status
fixed

## Date
2026-06-08

## Description
`tunnel stop <id>` 命令调用 `proxySvc.StopTunnel(id)` → `tunnelMgr.Remove(id)`，只在当前进程内查找转发。对于 daemon 模式运行的隧道，`tunnel stop` 在独立进程中执行，无法访问 daemon 进程的 tunnelManager，导致报错 "tunnel forward not found"。

## Root Cause
`tunnelStopAction` 只尝试进程内停止，没有跨进程机制：
```go
func (a *App) tunnelStopAction(c *cli.Context) error {
    id := c.Args()[0]
    return a.proxySvc.StopTunnel(id)  // 只在当前进程内查找
}
```

## Solution
修改 `tunnelStopAction` 实现两级停止策略：
1. 先尝试进程内停止（原有逻辑）
2. 如果未找到，读取 PID 文件（`config.PidPath`）获取 daemon 进程 PID
3. 向 daemon 进程发送 SIGTERM 信号
4. 清理 PID 文件

```go
// Try in-process stop first
err := a.proxySvc.StopTunnel(id)
if err == nil { return nil }

// Fallback: daemon mode via PID file + SIGTERM
proc, _ := proxy.ReadPidFile(pidPath)
proc.Signal(syscall.SIGTERM)
proxy.RemovePidFile(pidPath)
```

## Modified Files
- `cmd/tunnel.go` — 修改 `tunnelStopAction`，新增 `syscall` 和 `assh/asshc/infra/proxy` 导入

## Notes
- SIGTERM 会停止 daemon 进程中的所有转发，而非单个转发
- PID 文件路径使用 `config.PidPath`（`~/.assh/v2/tunnel.pid`）
- 与 BUG-014（daemon 模式修复）配合使用
