# BUG-005: assh info 不显示 Password 和 KeyFile 详情

## Status
fixed

## Date
2026-05-10

## Description
`assh info <servername>` 的 Auth 行将 Password 和 KeyFile 合并为一行显示，
密码仅显示 `password` 字样而不显示实际值，与 v1 的 `info` 输出不一致。

**v1 输出：**
```
Server Password:   secret123
Server PemKey:     /path/to/key
```

**修复前 v2 输出：**
```
Auth:    password
```

## Solution
将 `printServerDetail()` 中的 Auth 单行显示拆分为 Password 和 KeyFile 两行，
与 v1 行为对齐：
- 有密码 → 显示 `Password:<value>`
- 有密钥 → 显示 `KeyFile: <path>`
- 两者皆无 → 显示 `Auth:    none`

## Affected Files
- `asshv2/cmd/server.go` — `printServerDetail()` 方法

## Related
- v1 参考：`assh/cmd/server.go` 中 `printServerInfo()` 直接显示 Password 和 PemKey 字段
