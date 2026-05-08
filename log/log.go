package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

const (
	OFF   = 0
	FATAL = 100
	PANIC = 150
	ERROR = 200
	WARN  = 300
	INFO  = 400
	DEBUG = 500
)

var (
	LogPath  string
	LogLevel = OFF

	logger zerolog.Logger
)

func init() {
	SetInit()
}

func SetInit() {
	var output interface{ Write([]byte) (int, error) }

	if LogLevel > 0 && LogPath != "" {
		absPath, err := filepath.Abs(LogPath)
		if err != nil {
			absPath = LogPath
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

	logger = zerolog.New(output).With().Timestamp().Logger()
}

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

func formatLogLevel(lv int) string {
	lvMaps := map[int]string{
		FATAL: "[FATAL] ",
		ERROR: "[ERROR] ",
		WARN:  "[WARN]  ",
		INFO:  "[INFO]  ",
		DEBUG: "[DEBUG] ",
		PANIC: "[PANIC] ",
	}
	if s, ok := lvMaps[lv]; ok {
		return s
	}
	return ""
}

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

func output(level int, msg string) string {
	if level > 0 && level <= LogLevel {
		lvl := levelToZerolog(level)
		switch lvl {
		case zerolog.FatalLevel:
			logger.Error().Str("level", "FATAL").Msg(msg)
		case zerolog.PanicLevel:
			logger.Error().Str("level", "PANIC").Msg(msg)
		case zerolog.ErrorLevel:
			logger.Error().Msg(msg)
		case zerolog.WarnLevel:
			logger.Warn().Msg(msg)
		case zerolog.InfoLevel:
			logger.Info().Msg(msg)
		case zerolog.DebugLevel:
			logger.Debug().Msg(msg)
		}
	}
	return msg
}

func exit(level int, msg string) {
	output(level, msg)
	switch level {
	case FATAL:
		os.Exit(1)
	case PANIC:
		panic(msg)
	}
}

func Print(args ...interface{}) {
	logger.Info().Msg(fmt.Sprint(args...))
}

func Printf(format string, args ...interface{}) {
	logger.Info().Msgf(format, args...)
}

func Println(args ...interface{}) {
	logger.Info().Msg(fmt.Sprint(args...))
}

func Fatal(args ...interface{}) {
	msg := fmt.Sprint(args...)
	exit(FATAL, msg)
}

func Fatalln(args ...interface{}) {
	exit(FATAL, fmt.Sprint(args...))
}

func Fatalf(format string, args ...interface{}) {
	exit(FATAL, fmt.Sprintf(format, args...))
}

func Panic(args ...interface{}) {
	msg := fmt.Sprint(args...)
	exit(PANIC, msg)
}

func Panicln(args ...interface{}) {
	exit(PANIC, fmt.Sprint(args...))
}

func Panicf(format string, args ...interface{}) {
	exit(PANIC, fmt.Sprintf(format, args...))
}

func Debug(args ...interface{}) {
	msg := fmt.Sprint(args...)
	output(DEBUG, msg)
}

func Info(args ...interface{}) {
	msg := fmt.Sprint(args...)
	output(INFO, msg)
}

func Warn(args ...interface{}) {
	msg := fmt.Sprint(args...)
	output(WARN, msg)
}

func Error(args ...interface{}) {
	msg := fmt.Sprint(args...)
	output(ERROR, msg)
}

func Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	output(DEBUG, msg)
}

func Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	output(INFO, msg)
}

func Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	output(WARN, msg)
}

func Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	output(ERROR, msg)
}
