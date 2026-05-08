# Log Module

**文件**: `asshv2/log/log.go`

## 依赖

- `github.com/rs/zerolog` - 高性能结构化日志库

## 功能

基于 zerolog 实现的日志模块：

- **日志级别**: OFF, FATAL, PANIC, ERROR, WARN, INFO, DEBUG
- **输出**: 支持文件输出和 stderr 输出
- **API**: 兼容原有接口 (Debug/Info/Warn/Error/Fatal/Panic 及其格式化版本)

## 配置

```go
log.LogLevel = log.DEBUG  // 设置日志级别
log.LogPath = "/path/to/logfile"  // 设置日志文件路径
log.SetInit()  // 重新初始化
```

## 日志级别常量

| 常量 | 值 |
|------|-----|
| OFF   | 0  |
| FATAL | 100 |
| PANIC | 150 |
| ERROR | 200 |
| WARN  | 300 |
| INFO  | 400 |
| DEBUG | 500 |