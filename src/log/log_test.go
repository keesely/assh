// log_test.go kee > 2019/11/26

package log

import (
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	LogFile = "./test.log"
	LogLevel = DEBUG
	SetInit()
	Print("======================================================================")
	Println("start.")
	Debug("debug ...")
	Println("to day", time.Now())
	Warn("warning ...")
	Info("info ... ")
	Error("error ...")
	Println("hello world")
	//Panic("panic to do")
	Fatal("end.")
}
