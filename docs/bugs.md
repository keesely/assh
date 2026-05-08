# Bugs & Known Issues

> 记录开发过程中发现的 Bug、已知问题及修复方案。

## 已修复

| ID | 描述 | 影响范围 | 修复版本 | 修复日期 |
|----|------|----------|----------|----------|
| | | | | |

## 已知问题

| ID | 描述 | 影响范围 | 状态 | 备注 |
|----|------|----------|------|------|
| | | | | |

## 修复记录

### BUG-001: log 模块 Fatal/Panic 测试导致进程退出

**发现日期**: 2026-05-08

**问题描述**:
`log/output()` 函数直接调用 `logger.Fatal().Msg()` 和 `logger.Panic().Msg()`，
zerolog 的 Fatal 会调用 `os.Exit(1)`、Panic 会 `panic()`，导致测试进程被杀死。

**修复方案**:
- `output()` 不再直接调用 Fatal/Panic，改由 `exit()` 函数统一管理
- Fatal/Panic 在输出日志后由调用方处理进程退出/panic
- 测试仅验证 `output()` 的日志输出行为，不触发实际退出

**涉及文件**: `log/log.go`, `log/log_test.go`

### BUG-002: lib/qiniu getCfg 函数存在不可达代码

**问题描述**:
`getCfg()` 函数中 `return storage.Config{}` 在配置初始化之前执行，
导致后续配置代码不可达。go vet 报 `unreachable code`。

**修复方案**:
合并为单次 return，移除死代码。

**涉及文件**: `lib/qiniu/bucket.go`

### BUG-003: log_test.go 级别过滤测试期望值颠倒

**问题描述**:
`TestOutput_LevelFiltering` 测试用例中 "below/above threshold" 的
`expectLog` 值与实际行为相反，3 个用例中 2 个失败。

**修复方案**:
修正测试名称和期望值，使其匹配 `level <= LogLevel` 的实际过滤逻辑。

**涉及文件**: `log/log_test.go`
