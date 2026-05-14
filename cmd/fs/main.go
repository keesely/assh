package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"assh/asshc/domain"
	sshinfra "assh/asshc/infra/ssh"
	"assh/asshc/infra/sftp"
	"assh/asshc/service"
	"assh/cmd"
	"assh/config"
	"golang.org/x/crypto/ssh"
)

var (
	Version = "v2.0.0"
	Build   string
)

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

	store, err := cmd.NewAppComponents(cfgDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize store: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	connector := sshinfra.NewConnector()

	sshConnectorFunc := func(server *domain.Server) (*ssh.Client, error) {
		return connector.Connect(server)
	}

	sftpTransfer := sftp.NewSFTPTransfer(sshConnectorFunc)
	serverSvc := service.NewServerService(store.Store)
	transferSvc := service.NewTransferService(sftpTransfer, store.Store)

	app := cmd.NewFSApp(Version, Build, transferSvc, serverSvc)
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