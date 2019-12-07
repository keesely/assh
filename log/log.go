// log.go kee > 2019/11/26

package log

import (
	"fmt"
	"github.com/keesely/kiris"
	"log"
	"os"
	"strings"
	"time"
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
	LogPath       = ""
	LogLevel      = OFF
	LogLevelPrint = WARN
	logger        *log.Logger
)

func init() {
	SetInit()
}

func SetInit() {
	if LogLevel > 0 && LogPath != "" {
		var (
			fl  *os.File
			err error
		)
		LogPath = kiris.RealPath(LogPath)
		if _, err := os.Stat(LogPath); err != nil {
			fl, err = os.Create(LogPath)
		} else {
			fl, err = os.OpenFile(LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		}

		if err != nil {
			log.Fatal(err)
		}

		logger = log.New(fl, "", log.LstdFlags)
		//logger.SetFlags(log.Lshortfile | log.Lmicroseconds)
	} else {
		logger = log.New(os.Stderr, "", log.LstdFlags)
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
		WARN:  ("\033[0;33m[WARN]  \033[0m"),
		INFO:  ("\033[0;36m[INFO]  \033[0m"),
		DEBUG: ("\033[0;37m[DEBUG] \033[0m"),
		PANIC: ("\033[5;35m[Panic] \033[0m"),
	}
	if Output, ok := lvMaps[lv]; ok {
		return Output
	}
	return ""
}

func Print(args ...interface{}) {
	logger.Print(args...)
}

func Printf(format string, args ...interface{}) {
	logger.Printf(format, args...)
}

func Println(args ...interface{}) {
	logger.Println(args...)
}

func Fatal(args ...interface{}) {
	echo(FATAL, args)
}

func Fatalln(args ...interface{}) {
	echo(FATAL, args)
}

func Fatalf(format string, args ...interface{}) {
	echof(FATAL, format, args)
}

func Panic(args ...interface{}) {
	echo(PANIC, args)
}

func Panicln(args ...interface{}) {
	echo(PANIC, args)
}

func Panicf(format string, args ...interface{}) {
	echof(PANIC, format, args)
}

func Debug(args ...interface{}) {
	echo(DEBUG, args)
}

func Info(args ...interface{}) {
	echo(INFO, args)
}

func Warn(args ...interface{}) {
	echo(WARN, args)
}

func Error(args ...interface{}) {
	echo(ERROR, args)
}

func Debugf(format string, args ...interface{}) {
	echof(DEBUG, format, args)
}

func Infof(format string, args ...interface{}) {
	echof(INFO, format, args)
}

func Warnf(format string, args ...interface{}) {
	echof(WARN, format, args)
}

func Errorf(format string, args ...interface{}) {
	echof(ERROR, format, args)
}

func output(level int, s string) string {
	//logger.SetPrefix("\033[0m")
	if level > 0 && level <= LogLevel {
		//Lvf := formatLogLevel(level)
		//logger.SetPrefix(Lvf)
		logger.Output(2, s)

		if level <= LogLevelPrint {
			//fmt.Println(Lvf, time.Now().Format("2006/01/02 15:04:05"), s)
			fmt.Println(time.Now().Format("2006/01/02 15:04:05"), s)
		}

		if PANIC == level {
			panic(s)
		} else if FATAL == level {
			os.Exit(1)
		}
	}
	return s
}

func echo(level int, args []interface{}) {
	lv := formatLogLevel(level)
	output(level, fmt.Sprint(lv, fmt.Sprint(args...)))
}

func echof(level int, format string, args []interface{}) {
	lv := formatLogLevel(level)
	output(level, fmt.Sprint(lv, fmt.Sprintf(format, args...)))
}
