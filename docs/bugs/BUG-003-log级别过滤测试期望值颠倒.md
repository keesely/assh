# BUG-003: log级别过滤测试期望值颠倒

## Status
fixed

## Date
2026-05-08

## Description
`TestOutput_LevelFiltering` 测试用例中 "below/above threshold" 的 `expectLog` 值与实际行为相反，3 个用例中 2 个失败。

## Solution
修正测试名称和期望值，使其匹配 `level <= LogLevel` 的实际过滤逻辑。

## Affected Files
- `log/log_test.go`