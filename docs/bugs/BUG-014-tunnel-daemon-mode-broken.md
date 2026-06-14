# BUG-014: tunnel --daemon 模式无法正常工作

## Status
fixed (2026-06-04)

## Date
2026-06-04

## Description

使用 `--daemon/-d` 参数运行 tunnel 命令时，存在多个严重问题：

### 复现步骤

```bash
./assh tunnel ali.24 -R 12496::4096 -d
```

### 问题 1: daemon 子进程立即退出

**现象**: daemon 模式下 fork 的子进程在启动隧道后立即退出，隧道无法持续运行。

**根因**: `Daemonize()` 使用 `os.StartProcess` fork 子进程，但子进程从 `Daemonize()` 返回后，执行流直接走到 `startTunnelWithClient()` 的 `return nil`，然后层层返回直到 `main()` 结束，子进程退出。

**执行流**:
```
Daemonize() → os.StartProcess 产生子进程
  → 子进程从 Daemonize() 返回 nil
  → startTunnelWithClient() return nil
  → StartTunnel() return nil
  → tunnelAction() return nil
  → urfave/cli 框架处理完毕
  → main() 结束
  → 子进程退出
```

**关键代码** (`asshc/infra/proxy/tunnel_manager.go`):
```go
func Daemonize() error {
    exe, err := os.Executable()
    proc, err := os.StartProcess(exe, os.Args, &os.ProcAttr{
        Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},  // 继承父进程的 stdin/stdout/stderr
    })
    log.Debugf("daemonized child PID: %d", proc.Pid)
    os.Exit(0)  // 父进程退出
    return nil   // unreachable
}
```

**问题**:
1. 子进程没有阻塞机制（`waitForSignal()` 只在 `!opts.Daemon` 分支执行）
2. 没有调用 `syscall.Setsid()` 创建新会话
3. stdout/stderr 继承自父进程，终端关闭时子进程会收到 SIGPIPE

### 问题 2: 日志输出到前台终端

**现象**: daemon 模式下，日志仍然输出到启动命令的终端。

**根因**: `Daemonize()` 将子进程的 stdout/stderr 继承为父进程的同一个文件描述符：
```go
Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}
```

**期望**: 日志应输出到默认日志文件位置（如 `~/.assh/v2/logs/`），不直接输出到前台。

### 问题 3: PID 文件未被使用

**现象**: 无法通过 PID 文件管理和停止 daemon 进程。

**根因**: `WritePidFile()` 和 `ReadPidFile()` 已实现，但 `Daemonize()` 从未调用 `WritePidFile()`。

**影响**: daemon 模式下启动的隧道无法被外部停止（`tunnel stop` 只能停止同一进程内的转发）。

### 问题 4: 自动重连是空壳

**现象**: `--auto-reconnect` 参数被解析但不生效。

**根因**: `SetReconnect()` 只设置标志和配置，没有启动任何监控 goroutine 来执行重连逻辑。`done` channel 永远不会被 close，导致 `Close()` 总是超时。

### 问题 5: daemon 模式下 mu.Lock() 未释放

**现象**: daemon 模式下 `startTunnelWithClient()` 中的 mutex 锁永远不会被释放。

**根因**:
```go
if opts.Daemon {
    proxy.Daemonize()  // 父进程 os.Exit(0)，子进程从这里继续
    // 直接 return，没有 Unlock
}

if !opts.Daemon {
    s.mu.Unlock()
    waitForSignal(...)
    s.mu.Lock()
}
return nil  // daemon 模式下 mu 从未 Unlock
```

**影响**: 如果子进程后续需要操作（如信号处理调用 `StopProxy()`），会死锁。

## Affected Files

- `asshv2/asshc/infra/proxy/tunnel_manager.go` — Daemonize() 实现
- `asshv2/asshc/service/proxy.go` — startTunnelWithClient() daemon 分支
- `asshv2/cmd/tunnel.go` — tunnel 命令入口

## Expected Behavior

1. `--daemon` 模式下，隧道应在后台持续运行
2. 日志输出到默认日志文件，不输出到前台终端
3. daemon 进程应写入 PID 文件，支持外部管理
4. 终端关闭不应影响 daemon 进程

## Fix (2026-06-04)

分支: `bugfix/BUG-014-tunnel-daemon`

### 修复内容

1. **Daemonize() 重写** (`asshc/infra/proxy/tunnel_manager.go`)
   - 使用 `syscall.Setsid` 创建新会话，脱离终端控制
   - 将 stdin/stdout/stderr 重定向到 `/dev/null`（不再继承父进程 fd）
   - 子进程写入 PID 文件（支持自定义路径，默认 `~/.assh/v2/tunnel.pid`）
   - 新增 `WaitForSignal()` 通用阻塞函数，替代原来的内联 `waitForSignal()`
   - 新增 `RemovePidFile()` 清理函数

2. **mu.Lock() 死锁修复** (`asshc/service/proxy.go`)
   - `startTunnelWithClient()`: daemon 和非 daemon 路径统一先 Unlock 再阻塞
   - `startRemoteListenMode()`: 同上
   - `startProxyWithClient()`: 同上
   - 删除旧的 `waitForSignal()` 内联函数（已被 `proxy.WaitForSignal` 替代）

3. **PID 文件管理**
   - `config/path.go`: 新增 `PidPath = "~/.assh/v2/tunnel.pid"` 默认路径
   - `cmd/tunnel.go`: 新增 `--pid-file` flag，传递到 `ProxyOptions.PidPath`
   - `cmd/proxy.go`: 新增 `--pid-file` flag，传递到 `ProxyOptions.PidPath`
   - 退出时自动清理 PID 文件

4. **日志处理**
   - daemon 模式下 stdout/stderr 重定向到 `/dev/null`
   - 日志输出由 `log.LogPath` 控制（如需文件日志，设置 `-v` 自动输出到 `/tmp/assh.log`）

### 未修复项（已知限制）

- **自动重连**: `--auto-reconnect` 仍为配置解析骨架，无实际重连 goroutine（属于 NEXT-001.13 范围）
- **tunnel-stop 跨进程**: daemon 模式下 `tunnel stop` 仍只能停止同进程内的转发，跨进程停止需通过 `kill $(cat ~/.assh/v2/tunnel.pid)` 实现

## References

- [NEXT-001 设计文档](../worklogs/2026-05-29-NEXT-001-Design.md) — 服务管理设计
- [TODO.md](../TODO.md) — NEXT-001.6 ~ NEXT-001.13 统一服务管理
