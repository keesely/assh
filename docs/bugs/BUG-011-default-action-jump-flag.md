# BUG-011: 默认 Action 中 -J 在 target 后未解析

## Status
fixed

## Date
2026-05-19

## Description

使用默认 Action（`assh <target> -J <jump>`）时，`-J` flag 放置在 target 之后不会被解析，导致连接直连到目标而非经过跳板机。

**复现**：
```bash
./assh ali.sg -J ali.24     # ❌ -J 未解析，直连
./assh -J ali.24 ali.sg     # ✅ -J 正确解析，走跳板
```

**根因**：urfave/cli v1 在默认 Action 中不解析位置参数（target）后的全局 flags。

## Solution

使用已有的 v1 兼容模式 `resolveFlag(c, "jump", "J")` 统一处理：
1. 优先使用 `c.IsSet("jump")` 检查 CLI 是否已解析
2. 未解析时回退到 `lookupShortFlag(c, "J")`，手动扫描 `c.Args()` 中 `-J <value>` 格式

## Affected Files
- `cmd/connect.go` — `loginCore()` 中 `-J` flag 获取方式改为 `resolveFlag`
