# BUG-013: proxy --http --auth 未进行 Basic Auth 校验

## Status

fixed

## Date

2026-05-29

## Description

执行 `assh proxy ali.24 --http :8009 --ports 8009:4096 --auth kee:admin` 时：
```
{"level":"info","time":"2026-05-29T23:56:08+08:00","message":"Starting remote http listen on 0.0.0.0:8009 -> local 127.0.0.1:4096"}
{"level":"debug","time":"2026-05-29T23:56:08+08:00","message":"tunnel forward [84a0fd68a21491d6] added (type=remote)"}
{"level":"debug","time":"2026-05-29T23:56:08+08:00","message":"remote forward [84a0fd68a21491d6] started: 0.0.0.0:8009 <- (SSH) <- 127.0.0.1:4096"}
{"level":"debug","time":"2026-05-29T23:56:08+08:00","message":"tunnel forward [84a0fd68a21491d6] started"}
```

**问题**：指定了 `--auth kee:admin` 参数，但 HTTP 代理未进行 Basic Auth 校验。客户端无需提供认证信息即可访问代理服务。

**预期行为**：HTTP 代理应该要求客户端提供 `Authorization: Basic <base64(user:pass)>` 头，未认证时返回 401 Unauthorized。

### 复现步骤

```bash
# 终端 1: 启动代理
./assh proxy ali.24 --http :8009 --ports 8009:4096 --auth kee:admin

# 终端 2: 无认证访问（应该返回 401，实际成功）
curl http://localhost:8009/
```

### 环境信息

- 版本: v2.1.0-dev
- 平台: darwin (MacOS)

## Root Cause

两个问题叠加：

1. **`startRemoteListenMode` 忽略 `--auth`**：使用 SSH Remote Forward（纯 TCP 转发），完全忽略了 `--auth` 参数
2. **`httpHandshake` 认证检查顺序错误**：先检查 `method != "CONNECT"` 返回 405，再检查 auth。客户端发 `GET / HTTP/1.1` 时，在检查 auth 之前就被拒绝了

## Solution

1. 新增 `startRemoteHTTPProxyWithAuth`：当 `--http` + `--auth` 时，启动本地 HTTP CONNECT 代理 + SSH Remote Forward
2. 修复 `httpHandshake`：先读取所有 header（提取 Proxy-Authorization），先检查 auth，再检查 method。未认证请求返回 407，而非 405

## Affected Files

- `asshc/service/proxy.go` — 远程监听模式 + HTTP 代理 + Auth 支持
- `asshc/infra/proxy/http_connect.go` — HTTP CONNECT 代理实现（已有 Basic Auth 支持）
