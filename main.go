package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"assh/cmd"
	sshinfra "assh/asshc/infra/ssh"
	"assh/asshc/infra/store"
	"assh/asshc/service"
	"assh/config"
)

var Version = "v2.0.0"
var Build string

func main() {
	cfgDir := resolveConfigDir()
	if cfgDir != "" {
		config.ConfigPath = cfgDir
		config.SetDbPath(cfgDir + "/asshv2.db")
	}

	dbPath, err := config.ExpandPath(config.GetDbPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to resolve db path: %v\n", err)
		os.Exit(1)
	}
	config.EnsureDir(filepath.Dir(dbPath))

	repo, err := store.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize store: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	connector := sshinfra.NewConnector()
	session := sshinfra.NewSession()

	serverSvc := service.NewServerService(repo)
	connectSvc := service.NewConnectService(connector, session, repo)

	app := cmd.NewApp(Version, Build, connectSvc, serverSvc)
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func resolveConfigDir() string {
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-F" || arg == "--config" {
			if i+1 < len(os.Args) {
				p := os.Args[i+1]
				if info, err := os.Stat(p); err == nil && !info.IsDir() {
					return filepath.Dir(p)
				}
				return p
			}
		}
		if !strings.HasPrefix(arg, "-") {
			break
		}
	}
	return os.Getenv("ASSH_CONFIG_DIR")
}
