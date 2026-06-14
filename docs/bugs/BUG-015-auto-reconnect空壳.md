# BUG-015: auto-reconnect 空壳：SetReconnect 只设标志无监控协程

## Status
fixed

## Date
2026-06-08

## Description
`--auto-reconnect` 参数被正确解析，`SetReconnect()` 被调用，但只设置了 `m.reconnect=true` 和配置，没有启动任何监控协程来检测断线和执行重连。`Close()` 方法引用的 `m.done` channel 也从未被关闭，导致关闭时总是超时 5 秒。

## Root Cause
`tunnel_manager.go` 中的 `SetReconnect()` 方法只设置了标志位，没有实现重连逻辑：
```go
func (m *tunnelManager) SetReconnect(cfg ReconnectConfig) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.reconnect = true
    m.reconnectCfg = cfg
    // 缺少：启动监控协程
}
```

## Solution
1. `tunnelManager` 新增 `reconnectFunc` 字段，由 service 层提供重连回调
2. `SetReconnect()` 签名变更为 `SetReconnect(cfg, reconnectFn)`，启动 `reconnectLoop()` 协程
3. `reconnectLoop()` 监控 `ssh.Client.Wait()` 检测断线，断线后调用 `tryReconnect()` 指数退避重连
4. 重连成功后调用 `StartAll()` 恢复端口转发
5. `Close()` 改为检查 `m.done` 是否为 nil（未启用重连时直接返回）
6. service 层 `StartTunnel`/`StartTunnelWithChain` 提供重连回调（重新获取 server 信息并连接）

## Modified Files
- `asshc/infra/proxy/tunnel_manager.go` — 新增 `reconnectFn` 字段、`reconnectLoop()`、`tryReconnect()`，修改 `SetReconnect()` 和 `Close()`
- `asshc/service/proxy.go` — 更新 `tunnelManagerExt` 接口、`startTunnelWithClient` 签名、`StartTunnel`/`StartTunnelWithChain` 提供重连回调

## Notes
- 代理模式（SOCKS5）的 auto-reconnect 暂不支持，因为需要单独重启 SOCKS5 代理，已记录 warning
- proxy 模式下 `startProxyWithClient` 中的 `SetReconnect` 调用已移除，避免接口不匹配
