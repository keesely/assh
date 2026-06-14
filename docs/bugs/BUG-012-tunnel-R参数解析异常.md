# BUG-012: tunnel 命令 -R 参数解析异常

## Status

fixed

## Date

2026-05-29

## Description

执行 `assh tunnel ali.24 -R 1198::4096` 时报错：
```
error: at least one -L or -R flag is required
```

预期行为：应该识别 `-R` 参数并启动远程端口转发（远程 1198 → 本地 4096）。

实际行为：参数解析器未能正确识别 `-R` 标志，认为没有提供 `-L` 或 `-R` 参数。

### 复现步骤

```bash
./assh tunnel ali.24 -R 1198::4096
```

### 环境信息

- 版本: v2.1.0-dev
- 平台: darwin (MacOS)

## Root Cause

urfave/cli v1 的已知 Bug：当命令定义了 `Subcommands` 时，主命令的 `Flags` 不会被解析。

原 `tunnel` 命令同时有 `Flags`（`-L`, `-R` 等）和 `Subcommands`（`list`, `start`, `stop`, `rm`），导致 `-L`/`-R` 标志无法被解析。

## Solution

将 `tunnel` 的子命令（`list`, `start`, `stop`, `rm`）从 `Subcommands` 移出，注册为顶级命令：
- `tunnel-list` — 列出保存的隧道配置
- `tunnel-start <name|id>` — 启动保存的隧道
- `tunnel-stop <id>` — 停止隧道
- `tunnel-rm <name|id>` — 删除保存的隧道

`tunnel` 主命令不再有 `Subcommands`，flags 正确解析。

## Affected Files

- `cmd/tunnel.go` — tunnel 命令定义和参数解析
- `cmd/app.go` — 注册命令调整
