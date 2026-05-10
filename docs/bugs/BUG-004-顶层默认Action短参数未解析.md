# BUG-004: 顶层默认 Action 短参数（-p/-P/-u 等）未解析

## Status
fixed

## Date
2026-05-10

## Description
在 v2 实现 v1 兼容的顶层默认登录行为（ADR-016）后，以下命令中的 `-p` 参数未被正确解析：

```
assh root@120.77.34.9 -p 9999 -P "password"
```

始终使用默认端口 22 连接，而非指定的 9999。

**根因**：urfave/cli v1 在默认 Action（`app.Action`）中，遇到位置参数后会停止解析全局 flags。当执行 `assh root@120.77.34.9 -p 9999` 时，`root@120.77.34.9` 被视为位置参数（尝试匹配命令名），`-p 9999` 之后的参数不会被解析，`c.Int("port")` 始终返回默认值 0，进而被 `port <= 0 { port = 22 }` 覆盖。

同样受影响的参数：`-P/--password`、`-i/--identity-file`、`-k/--key`、`-H/--host`、`-u/--user`、`-l/--login`。

## Solution
将 v1 的 `lookupShortFlag` 手动解析方案移植到 v2，新增三个函数：

- **`lookupShortFlag(c, shortName)`** — 从 `c.Args()` 中遍历查找 `-p 9999`（空格分隔）和 `-p9999`（紧凑格式）两种形式
- **`resolveFlag(c, longName, shortName)`** — 优先使用 cli 正常解析的值（`c.IsSet` 为 true），未设置时回落手动查找
- **`resolveIntFlag(c, longName, shortName)`** — resolveFlag 的整数版本

`loginCore` 中原来直接调用 `c.Int("port")`/`c.String("password")` 改为调用 `resolveIntFlag`/`resolveFlag`，确保无论通过 `login` 命令还是默认 Action 都能正确解析参数。

同时在 `setupGlobalFlags()` 中注册了全部连接参数（`-p/-u/-l/-P/-i/-k/-H`）为全局 flags，保证 `--long-name` 形式也能正常使用。

## Affected Files
- `asshv2/cmd/connect.go` — 新增 `resolveFlag`、`resolveIntFlag`、`lookupShortFlag`；`loginCore` 改用回落解析
- `asshv2/cmd/app.go` — `setupGlobalFlags()` 注册全部连接参数为全局 flags

## Related
- ADR-016: 顶层默认登录行为（v1 兼容）
