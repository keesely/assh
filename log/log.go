// Package log 提供基于 zerolog 的日志记录功能。
//
// 支持 OFF/FATAL/PANIC/ERROR/WARN/INFO/DEBUG 七级日志级别，
// 提供格式化输出（*f 后缀）和标准输出（*ln 后缀）的便捷方法。
// 日志可同时输出到文件和标准错误流。
package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

// 日志级别常量，按严重程度从高到低排列。
const (
	OFF   = 0   // 关闭日志
	FATAL = 100 // 致命错误（输出后调用 os.Exit(1)）
	PANIC = 150 // 恐慌错误（输出后触发 panic）
	ERROR = 200 // 一般错误
	WARN  = 300 // 警告信息
	INFO  = 400 // 普通信息
	DEBUG = 500 // 调试信息
)

var (
	LogPath  string       // 日志文件路径（空字符串表示输出到 stderr）
	LogLevel = OFF        // 当前日志级别，低于该级别的日志被过滤

	logger zerolog.Logger // 全局 zerolog 日志器实例
)

func init() {
	SetInit()
}

// SetInit 初始化或重新初始化日志器。
// 根据 LogPath 的值决定输出到文件或 stderr。
func SetInit() {
	logger = createLogger(LogPath)
}

// createLogger 创建 zerolog 日志器实例。
// 如果 logPath 非空，尝试创建日志文件并输出到文件；
// 否则输出到标准错误流。
func createLogger(logPath string) zerolog.Logger {
	var output interface{ Write([]byte) (int, error) }

	if logPath != "" {
		absPath, err := filepath.Abs(logPath)
		if err != nil {
			absPath = logPath
		}

		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		}

		file, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
			output = os.Stderr
		} else {
			output = file
		}
	} else {
		output = os.Stderr
	}

	return zerolog.New(output).With().Timestamp().Logger()
}

// GetLogLevel 将日志级别名称字符串转换为对应的整数值。
// 支持的名称（不区分大小写）：OFF, FATAL, PANIC, ERROR, WARN, INFO, DEBUG。
func GetLogLevel(lv string) int {
	lv = strings.ToUpper(lv)
	lvMaps := map[string]int{
		"OFF":   OFF,
		"FATAL": FATAL,
		"PANIC": PANIC,
		"ERROR": ERROR,
		"WARN":  WARN,
		"INFO":  INFO,
		"DEBUG": DEBUG,
	}
	if lvInt, ok := lvMaps[lv]; ok {
		return lvInt
	}
	return OFF
}

// levelToZerolog 将内部日志级别转换为 zerolog 的 Level 类型。
func levelToZerolog(lv int) zerolog.Level {
	switch lv {
	case FATAL:
		return zerolog.FatalLevel
	case PANIC:
		return zerolog.PanicLevel
	case ERROR:
		return zerolog.ErrorLevel
	case WARN:
		return zerolog.WarnLevel
	case INFO:
		return zerolog.InfoLevel
	case DEBUG:
		return zerolog.DebugLevel
	default:
		return zerolog.NoLevel
	}
}

// output 根据日志级别输出消息到日志器，同时返回消息字符串。
func output(level int, msg string) string {
	switch level {
	case FATAL:
		logger.Error().Str("level", "FATAL").Msg(msg)
	case PANIC:
		logger.Error().Str("level", "PANIC").Msg(msg)
	case ERROR:
		logger.Error().Msg(msg)
	case WARN:
		logger.Warn().Msg(msg)
	case INFO:
		logger.Info().Msg(msg)
	case DEBUG:
		logger.Debug().Msg(msg)
	}
	return msg
}

// exit 输出日志并根据级别执行退出操作（FATAL 退出进程，PANIC 触发 panic）。
func exit(level int, msg string) {
	output(level, msg)
	switch level {
	case FATAL:
		os.Exit(1)
	case PANIC:
		panic(msg)
	}
}

// Print 记录 INFO 级别日志。
func Print(args ...interface{}) {
	logger.Info().Msg(fmt.Sprint(args...))
}

// Printf 以格式化方式记录 INFO 级别日志。
func Printf(format string, args ...interface{}) {
	logger.Info().Msgf(format, args...)
}

// Println 记录 INFO 级别日志（等价于 Print）。
func Println(args ...interface{}) {
	logger.Info().Msg(fmt.Sprint(args...))
}

// Fatal 记录致命错误日志并终止程序。
func Fatal(args ...interface{}) {
	msg := fmt.Sprint(args...)
	exit(FATAL, msg)
}

// Fatalln 记录致命错误日志并终止程序。
func Fatalln(args ...interface{}) {
	exit(FATAL, fmt.Sprint(args...))
}

// Fatalf 以格式化方式记录致命错误日志并终止程序。
func Fatalf(format string, args ...interface{}) {
	exit(FATAL, fmt.Sprintf(format, args...))
}

// Panic 记录恐慌日志并触发 panic。
func Panic(args ...interface{}) {
	msg := fmt.Sprint(args...)
	exit(PANIC, msg)
}

// Panicln 记录恐慌日志并触发 panic。
func Panicln(args ...interface{}) {
	exit(PANIC, fmt.Sprint(args...))
}

// Panicf 以格式化方式记录恐慌日志并触发 panic。
func Panicf(format string, args ...interface{}) {
	exit(PANIC, fmt.Sprintf(format, args...))
}

// Debug 记录 DEBUG 级别日志。
func Debug(args ...interface{}) {
	msg := fmt.Sprint(args...)
	output(DEBUG, msg)
}

// Info 记录 INFO 级别日志。
func Info(args ...interface{}) {
	msg := fmt.Sprint(args...)
	output(INFO, msg)
}

// Warn 记录 WARN 级别日志。
func Warn(args ...interface{}) {
	msg := fmt.Sprint(args...)
	output(WARN, msg)
}

// Error 记录 ERROR 级别日志。
func Error(args ...interface{}) {
	msg := fmt.Sprint(args...)
	output(ERROR, msg)
}

// Debugf 以格式化方式记录 DEBUG 级别日志。
func Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	output(DEBUG, msg)
}

// Infof 以格式化方式记录 INFO 级别日志。
func Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	output(INFO, msg)
}

// Warnf 以格式化方式记录 WARN 级别日志。
func Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	output(WARN, msg)
}

// Errorf 以格式化方式记录 ERROR 级别日志。
func Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	output(ERROR, msg)
}