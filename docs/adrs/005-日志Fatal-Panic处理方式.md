# ADR-005: 日志 Fatal/Panic 处理方式（exit 函数分离）

## Status
accepted

## Date
2026-05-08

## Context
原 output() 直接调用 zerolog.Fatal()/Panic() 会触发 os.Exit(1)/panic()，导致测试进程被杀死。

## Decision
output() 函数仅执行日志输出（Fatal/Panic 降级为 Error 级别），新增 exit() 函数处理后续的 os.Exit(1) 或 panic()，Fatal/Panic 包装函数调用 exit() 而非 output()。

## Consequences
1. output() 函数可测试，不触发进程终止
2. 测试覆盖度提升
3. Fatal/Panic 行为需要调用方配合（不能直接从 output 路径触发）
4. 需更新测试用例适配