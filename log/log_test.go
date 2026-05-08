package log

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestLogLevelConstants(t *testing.T) {
	tests := []struct {
		name     string
		expected int
		actual   int
	}{
		{"OFF", OFF, 0},
		{"FATAL", FATAL, 100},
		{"PANIC", PANIC, 150},
		{"ERROR", ERROR, 200},
		{"WARN", WARN, 300},
		{"INFO", INFO, 400},
		{"DEBUG", DEBUG, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("expected %s=%d, got %d", tt.name, tt.expected, tt.actual)
			}
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"OFF uppercase", "OFF", OFF},
		{"FATAL uppercase", "FATAL", FATAL},
		{"PANIC uppercase", "PANIC", PANIC},
		{"ERROR uppercase", "ERROR", ERROR},
		{"WARN uppercase", "WARN", WARN},
		{"INFO uppercase", "INFO", INFO},
		{"DEBUG uppercase", "DEBUG", DEBUG},
		{"OFF lowercase", "off", OFF},
		{"FATAL lowercase", "fatal", FATAL},
		{"PANIC lowercase", "panic", PANIC},
		{"ERROR lowercase", "error", ERROR},
		{"WARN lowercase", "warn", WARN},
		{"INFO lowercase", "info", INFO},
		{"DEBUG lowercase", "debug", DEBUG},
		{"OFF mixed case", "Off", OFF},
		{"FATAL mixed case", "FaTaL", FATAL},
		{"Invalid input", "INVALID", OFF},
		{"Empty string", "", OFF},
		{"Number string", "123", OFF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("GetLogLevel(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"FATAL", FATAL, "[FATAL] "},
		{"PANIC", PANIC, "[PANIC] "},
		{"ERROR", ERROR, "[ERROR] "},
		{"WARN", WARN, "[WARN]  "},
		{"INFO", INFO, "[INFO]  "},
		{"DEBUG", DEBUG, "[DEBUG] "},
		{"Invalid level", 999, ""},
		{"Zero", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("formatLogLevel(%d) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLevelToZerolog(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected zerolog.Level
	}{
		{"FATAL", FATAL, zerolog.FatalLevel},
		{"PANIC", PANIC, zerolog.PanicLevel},
		{"ERROR", ERROR, zerolog.ErrorLevel},
		{"WARN", WARN, zerolog.WarnLevel},
		{"INFO", INFO, zerolog.InfoLevel},
		{"DEBUG", DEBUG, zerolog.DebugLevel},
		{"OFF", OFF, zerolog.NoLevel},
		{"Invalid", 999, zerolog.NoLevel},
		{"Negative", -1, zerolog.NoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := levelToZerolog(tt.input)
			if result != tt.expected {
				t.Errorf("levelToZerolog(%d) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSetInit_Default(t *testing.T) {
	originalLevel := LogLevel
	originalPath := LogPath
	defer func() {
		LogLevel = originalLevel
		LogPath = originalPath
	}()

	LogLevel = 0
	LogPath = ""
	SetInit()

	var buf bytes.Buffer
	logger.Output(&buf)
	_, _ = logger.Write([]byte("test"))
	_ = buf.Len()
}

func TestSetInit_WithFile(t *testing.T) {
	originalLevel := LogLevel
	originalPath := LogPath
	defer func() {
		LogLevel = originalLevel
		LogPath = originalPath
	}()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	LogLevel = INFO
	LogPath = tmpFile
	SetInit()

	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}
	if info.IsDir() {
		t.Error("expected file, got directory")
	}
}

func TestSetInit_CreateDir(t *testing.T) {
	originalLevel := LogLevel
	originalPath := LogPath
	defer func() {
		LogLevel = originalLevel
		LogPath = originalPath
	}()

	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "dirs", "test")
	tmpFile := filepath.Join(nestedDir, "test.log")

	LogLevel = INFO
	LogPath = tmpFile
	SetInit()

	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}
	if info.IsDir() {
		t.Error("expected file, got directory")
	}
}

func TestLogLevelFiltering(t *testing.T) {
	tests := []struct {
		name      string
		level     int
		testFunc  func()
		shouldLog bool
	}{
		{
			name:      "DEBUG when level OFF",
			level:     OFF,
			testFunc:  func() { output(DEBUG, "debug message") },
			shouldLog: false,
		},
		{
			name:      "DEBUG when level ERROR",
			level:     ERROR,
			testFunc:  func() { output(DEBUG, "debug message") },
			shouldLog: false,
		},
		{
			name:      "ERROR when level DEBUG",
			level:     DEBUG,
			testFunc:  func() { output(ERROR, "error message") },
			shouldLog: true,
		},
		{
			name:      "INFO when level WARN",
			level:     WARN,
			testFunc:  func() { output(INFO, "info message") },
			shouldLog: false,
		},
		{
			name:      "WARN when level INFO",
			level:     INFO,
			testFunc:  func() { output(WARN, "warn message") },
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalLevel := LogLevel
			LogLevel = tt.level
			defer func() { LogLevel = originalLevel }()

			var buf bytes.Buffer
			oldLogger := logger
			logger = zerolog.New(&buf).With().Timestamp().Logger()

			tt.testFunc()

			logger = oldLogger

			logged := buf.Len() > 0
			if logged != tt.shouldLog {
				t.Errorf("expected logged=%v, got %v", tt.shouldLog, logged)
			}
		})
	}
}

func TestPrint(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := logger

	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Print("test message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected log to contain 'test message', got: %s", buf.String())
	}

	buf.Reset()
	Print("a", "b", "c")
	if !strings.Contains(buf.String(), "abc") {
		t.Errorf("expected log to contain 'abc', got: %s", buf.String())
	}
}

func TestPrintf(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := logger

	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Printf("test %s", "message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected log to contain 'test message', got: %s", buf.String())
	}

	buf.Reset()
	Printf("value: %d", 123)
	if !strings.Contains(buf.String(), "value: 123") {
		t.Errorf("expected log to contain 'value: 123', got: %s", buf.String())
	}
}

func TestPrintln(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := logger

	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Println("test message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected log to contain 'test message', got: %s", buf.String())
	}
}

func TestDebug(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = DEBUG
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Errorf("expected log to contain 'debug message', got: %s", buf.String())
	}
}

func TestDebugf(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = DEBUG
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Debugf("debug %s %d", "message", 42)
	if !strings.Contains(buf.String(), "debug message 42") {
		t.Errorf("expected log to contain 'debug message 42', got: %s", buf.String())
	}
}

func TestInfo(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = INFO
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Info("info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Errorf("expected log to contain 'info message', got: %s", buf.String())
	}
}

func TestInfof(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = INFO
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Infof("info %s", "message")
	if !strings.Contains(buf.String(), "info message") {
		t.Errorf("expected log to contain 'info message', got: %s", buf.String())
	}
}

func TestWarn(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = WARN
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Errorf("expected log to contain 'warn message', got: %s", buf.String())
	}
}

func TestWarnf(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = WARN
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Warnf("warn %s", "message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Errorf("expected log to contain 'warn message', got: %s", buf.String())
	}
}

func TestError(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = ERROR
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Errorf("expected log to contain 'error message', got: %s", buf.String())
	}
}

func TestErrorf(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = ERROR
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	Errorf("error %s", "message")
	if !strings.Contains(buf.String(), "error message") {
		t.Errorf("expected log to contain 'error message', got: %s", buf.String())
	}
}

func TestOutput_FatalLevel(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = FATAL
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	output(FATAL, "fatal message")
	if !strings.Contains(buf.String(), "fatal message") {
		t.Errorf("expected log to contain 'fatal message', got: %s", buf.String())
	}
}

func TestOutput_PanicLevel(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = PANIC
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	output(PANIC, "panic message")
	if !strings.Contains(buf.String(), "panic message") {
		t.Errorf("expected log to contain 'panic message', got: %s", buf.String())
	}
}

func TestPanicFunc(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = PANIC
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()

	Panic("panic message")
}

func TestPanicfFunc(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = PANIC
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()

	Panicf("panic %s", "message")
}

func TestOutput_ReturnsMessage(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = DEBUG
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	result := output(DEBUG, "test message")
	if result != "test message" {
		t.Errorf("expected return 'test message', got %q", result)
	}
}

func TestOutput_LevelFiltering(t *testing.T) {
	originalLevel := LogLevel

	tests := []struct {
		name        string
		logLevel    int
		outputLevel int
		expectLog   bool
	}{
		{"warn below debug level logs", DEBUG, WARN, true},
		{"debug equals threshold logs", DEBUG, DEBUG, true},
		{"debug above warn level filtered", WARN, DEBUG, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LogLevel = tt.logLevel
			defer func() { LogLevel = originalLevel }()

			var buf bytes.Buffer
			originalLogger := logger
			logger = zerolog.New(&buf).With().Timestamp().Logger()
			defer func() { logger = originalLogger }()

			output(tt.outputLevel, "test message")
			logged := buf.Len() > 0
			if logged != tt.expectLog {
				t.Errorf("expected logged=%v, got %v", tt.expectLog, logged)
			}
		})
	}
}
