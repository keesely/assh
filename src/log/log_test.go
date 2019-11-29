// log_test.go kee > 2019/11/26

package log

import (
	"assh/src/assh"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	assh.SetLogPath("./test.log")
	assh.SetLogLevel("DEBUG")
	Print("======================================================================")
	Print("start.")
	Debug("debug ...")
	Println("to day", time.Now())
	Warn("warning ...")
	Info("info ... ")
	Error("error ...")
	Panic("panic to do")
	Println("hello world")
	Fatal("end.")
}
