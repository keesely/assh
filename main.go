package main

import (
	"assh/src"
	"os"
)

//main
func main() {
	app := src.NewCli(os.Args)
	app.Run()
}
