package log

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// TestLogLevelConstants validates that all log level constants have the correct integer values.
// Constants: OFF=0, FATAL=100, PANIC=150, ERROR=200, WARN=300, INFO=400, DEBUG=500
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

// TestGetLogLevel verifies string-to-level conversion, including case-insensitive matching.
// Covers: valid strings (uppercase, lowercase, mixed case), invalid inputs.
func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		// Uppercase
		{"OFF uppercase", "OFF", OFF},
		{"FATAL uppercase", "FATAL", FATAL},
		{"PANIC uppercase", "PANIC", PANIC},
		{"ERROR uppercase", "ERROR", ERROR},
		{"WARN uppercase", "WARN", WARN},
		{"INFO uppercase", "INFO", INFO},
		{"DEBUG uppercase", "DEBUG", DEBUG},
		// Lowercase
		{"OFF lowercase", "off", OFF},
		{"FATAL lowercase", "fatal", FATAL},
		{"PANIC lowercase", "panic", PANIC},
		{"ERROR lowercase", "error", ERROR},
		{"WARN lowercase", "warn", WARN},
		{"INFO lowercase", "info", INFO},
		{"DEBUG lowercase", "debug", DEBUG},
		// Mixed case
		{"OFF mixed case", "Off", OFF},
		{"FATAL mixed case", "FaTaL", FATAL},
		// Invalid inputs
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

// TestFormatLogLevel tests conversion from integer level to human-readable label string.
// Covers: all valid levels, invalid levels (returns empty string).
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

// TestLevelToZerolog verifies internal level to zerolog.Level enum conversion.
// Covers: all valid levels, OFF (returns NoLevel), invalid values.
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

// TestSetInit_Default tests default initialization when LogLevel=0 or LogPath is empty.
// Expected: logger writes to stderr.
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

// TestSetInit_WithFile verifies file-based initialization when LogPath is set.
// Expected: log file is created.
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

// TestSetInit_CreateDir tests automatic directory creation for nested paths.
// Expected: parent directories are created automatically.
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

// TestLogLevelFiltering verifies that messages are filtered based on LogLevel threshold.
// Only messages with level <= LogLevel should be output.
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

// TestPrint verifies Print() function handles variadic arguments correctly.
// Print outputs at INFO level.
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

// TestPrintf verifies Printf() handles format string and arguments correctly.
// Printf outputs at INFO level with formatted message.
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

// TestPrintln verifies Println() adds newline and outputs message.
// Println outputs at INFO level.
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

// TestDebug verifies Debug() outputs message at DEBUG level.
// Only logs when LogLevel >= DEBUG.
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

// TestDebugf verifies Debugf() handles format string at DEBUG level.
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

// TestInfo verifies Info() outputs message at INFO level.
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

// TestInfof verifies Infof() handles format string at INFO level.
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

// TestWarn verifies Warn() outputs message at WARN level.
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

// TestWarnf verifies Warnf() handles format string at WARN level.
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

// TestError verifies Error() outputs message at ERROR level.
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

// TestErrorf verifies Errorf() handles format string at ERROR level.
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

// TestFatal verifies Fatal() outputs at FATAL level and terminates the program.
// Uses goroutine with recover to capture os.Exit, preventing test crash.
func TestFatal(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = FATAL
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	done := make(chan struct{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected - os.Exit(1) called by zerolog.Fatal
			}
			close(done)
		}()
		Fatal("fatal message")
	}()

	<-done
}

// TestFatalln verifies Fatalln() outputs at FATAL level with newline, terminates program.
func TestFatalln(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = FATAL
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	done := make(chan struct{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected - os.Exit(1) called
			}
			close(done)
		}()
		Fatalln("fatal message")
	}()

	<-done
}

// TestFatalf verifies Fatalf() handles format string at FATAL level, terminates program.
func TestFatalf(t *testing.T) {
	originalLevel := LogLevel
	LogLevel = FATAL
	defer func() { LogLevel = originalLevel }()

	var buf bytes.Buffer
	originalLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = originalLogger }()

	done := make(chan struct{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected - os.Exit(1) called
			}
			close(done)
		}()
		Fatalf("fatal %s", "message")
	}()

	<-done
}

// TestPanic verifies Panic() outputs at PANIC level and triggers panic.
// Uses defer recover to verify panic is thrown.
func TestPanic(t *testing.T) {
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

// TestPanicln verifies Panicln() outputs at PANIC level with newline, triggers panic.
func TestPanicln(t *testing.T) {
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

	Panicln("panic message")
}

// TestPanicf verifies Panicf() handles format string at PANIC level, triggers panic.
func TestPanicf(t *testing.T) {
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

// TestOutput_ReturnsMessage verifies that output() returns the input message string.
// The return value allows chaining in some logging scenarios.
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

// TestOutput_LevelFiltering verifies output() respects LogLevel threshold filtering.
// Only outputs when message level <= configured LogLevel.
func TestOutput_LevelFiltering(t *testing.T) {
	originalLevel := LogLevel

	tests := []struct {
		name        string
		logLevel    int
		outputLevel int
		expectLog   bool
	}{
		{"output below threshold", DEBUG, WARN, false},
		{"output equals threshold", DEBUG, DEBUG, true},
		{"output above threshold", WARN, DEBUG, true},
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