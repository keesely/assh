// log.go kee > 2019/11/26

package log

import (
	"github.com/keesely/kiris"
	"log"
	"os"
	"strings"
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
	LogFile  = ""
	LogLevel = OFF
	logger   *log.Logger
)

func init() {
	SetInit()
}

func SetInit() {
	if LogLevel > 0 && LogFile != "" {
		var (
			fl  *os.File
			err error
		)
		LogFile = kiris.RealPath(LogFile)
		if _, err := os.Stat(LogFile); err != nil {
			fl, err = os.Create(LogFile)
		} else {
			fl, err = os.OpenFile(LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		}

		if err != nil {
			log.Fatal(err)
		}

		logger = log.New(fl, "", log.LstdFlags)
		//logger.SetFlags(log.Lshortfile | log.Lmicroseconds)
	} else {
		logger = &log.Logger{}
	}
}

func GetLogLevel(lv string) int {
	lv = strings.ToUpper(lv)
	lvMaps := map[string]int{
		"OFF":   OFF,
		"FATAL": FATAL,
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
		FATAL: ("\033[0;31m[FATAL] \033[0m"),
		ERROR: ("\033[0;35m[ERROR] \033[0m"),
		WARN:  ("\033[0;33m [WARN] \033[0m"),
		INFO:  ("\033[0;36m [INFO] \033[0m"),
		DEBUG: ("\033[0;37m[DEBUG] \033[0m"),
		PANIC: ("\033[5;35m[Panic] \033[0;35;47m"),
	}
	if output, ok := lvMaps[lv]; ok {
		return output
	}
	return ""
}

func Print(args ...interface{}) {
	logger.SetPrefix("\033[0m")
	logger.Print(args...)
}

func Printf(format string, args ...interface{}) {
	logger.SetPrefix("\033[0m")
	logger.Printf(format, args...)
}

func Println(args ...interface{}) {
	logger.SetPrefix("\033[0m")
	logger.Println(args...)
}

func Fatal(args ...interface{}) {
	logger.SetPrefix(formatLogLevel(FATAL))
	logger.Fatal(args...)
}

func Fatalln(args ...interface{}) {
	logger.SetPrefix(formatLogLevel(FATAL))
	logger.Fatalln(args...)
}

func Fatalf(format string, args ...interface{}) {
	logger.SetPrefix(formatLogLevel(FATAL))
	logger.Fatalf(format, args...)
}

func Panic(args ...interface{}) {
	logger.SetPrefix(formatLogLevel(PANIC))
	logger.Panic(args...)
}

func Panicln(args ...interface{}) {
	logger.SetPrefix(formatLogLevel(PANIC))
	logger.Panicln(args...)
}

func Panicf(format string, args ...interface{}) {
	logger.SetPrefix(formatLogLevel(PANIC))
	logger.Panicf(format, args...)
}

func Debug(args ...interface{}) {
	logger.SetPrefix(formatLogLevel(DEBUG))
	logger.Print(args...)
}

func Info(args ...interface{}) {
	logger.SetPrefix(formatLogLevel(INFO))
	logger.Print(args...)
}

func Warn(args ...interface{}) {
	logger.SetPrefix(formatLogLevel(WARN))
	logger.Print(args...)
}

func Error(args ...interface{}) {
	logger.SetPrefix(formatLogLevel(ERROR))
	logger.Print(args...)
}

func Debugf(format string, args ...interface{}) {
	logger.SetPrefix(formatLogLevel(DEBUG))
	logger.Printf(format, args...)
}

func Infof(format string, args ...interface{}) {
	logger.SetPrefix(formatLogLevel(INFO))
	logger.Printf(format, args...)
}

func Warnf(format string, args ...interface{}) {
	logger.SetPrefix(formatLogLevel(WARN))
	logger.Printf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	logger.SetPrefix(formatLogLevel(ERROR))
	logger.Printf(format, args...)
}
