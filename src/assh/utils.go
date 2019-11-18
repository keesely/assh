package assh

import (
	"log"
)

func check(err error, msg string) {
	if err != nil {
		log.Fatalf("%s fail: %v", msg, err)
	}
}
