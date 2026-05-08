package main

import (
	"assh/cmd"
	"os"
)

var Version = "v2.0.1-dev"
var Build string

func main() {
	app := cmd.NewApp(Version, Build)
	app.Run(os.Args)
}
