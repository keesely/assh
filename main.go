package main

import (
	"assh/cmd"
)

//main
func main() {
	app := cmd.NewCli()
	app.Run()
}
