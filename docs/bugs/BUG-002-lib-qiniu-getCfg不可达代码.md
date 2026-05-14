# BUG-002: lib/qiniu getCfg不可达代码

## Status
fixed

## Date
2026-05-08

## Description
`getCfg()` 函数中 `return storage.Config{}` 在配置初始化之前执行，导致后续配置代码不可达。go vet 报 `unreachable code`。

## Solution
合并为单次 return，移除死代码。

## Affected Files
- `lib/qiniu/bucket.go`