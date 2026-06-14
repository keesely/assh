# BUG-001: log模块Fatal/Panic测试导致进程退出

## Status
fixed

## Date
2026-05-08

## Description
`log/output()` 函数直接调用 `logger.Fatal().Msg()` 和 `logger.Panic().Msg()`，zerolog 的 Fatal 会调用 `os.Exit(1)`、Panic 会 `panic()`，导致测试进程被杀死。

## Solution
- `output()` 不再直接调用 Fatal/Panic，改由 `exit()` 函数统一管理
- Fatal/Panic 在输出日志后由调用方处理进程退出/panic
- 测试仅验证 `output()` 的日志输出行为，不触发实际退出

## Affected Files
- `log/log.go`
- `log/log_test.go`