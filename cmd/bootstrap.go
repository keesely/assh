package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"assh/asshc/infra/store"
	"assh/config"
)

type AppComponents struct {
	Store      *store.Store
}

func NewAppComponents(cfgDir string) (*AppComponents, error) {
	if cfgDir != "" {
		config.ConfigPath = cfgDir
		config.SetDbPath(cfgDir + "/asshv2.db")
	}

	dbPath, err := config.ExpandPath(config.GetDbPath())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve db path: %v", err)
	}
	config.EnsureDir(filepath.Dir(dbPath))

	store, err := store.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %v", err)
	}

	return &AppComponents{
		Store: store,
	}, nil
}

func (c *AppComponents) Close() error {
	if c.Store != nil {
		return c.Store.Close()
	}
	return nil
}

func EnsureConfigDir(cfgDir string) error {
	if cfgDir == "" {
		absPath, err := config.ExpandPath(config.DataPath)
		if err != nil {
			return err
		}
		cfgDir = absPath
	}

	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return nil
}