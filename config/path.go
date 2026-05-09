package config

import (
	"os"
	"path/filepath"
	"strings"
)

var (
	ConfigPath     = "~/.assh/v2"
	DataPath       = "~/.assh/v2/data"
	PrivateKeyPath = "~/.assh/v2/.rsa"
	PublicKeyPath  = "~/.assh/v2/.rsa.pub"
	PasswordFile   = "~/.assh/v2/.account"
	ConfigFile     = "~/.assh/v2/assh.yml"
	DbFile         = "~/.assh/v2/asshv2.db"
	LogPath        = "/tmp/assh.log"
)

func ExpandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home := os.Getenv("HOME")
		if home == "" {
			return "", os.ErrInvalid
		}
		return filepath.Join(home, path[2:]), nil
	}

	if filepath.IsAbs(path) {
		return path, nil
	}

	return filepath.Abs(path)
}

func EnsureDir(path string) error {
	expanded, err := ExpandPath(path)
	if err != nil {
		return err
	}

	exists, err := pathExists(expanded)
	if err != nil {
		return err
	}

	if exists {
		info, err := os.Stat(expanded)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return os.ErrExist
		}
		return nil
	}

	return os.MkdirAll(expanded, 0755)
}

func FileExists(path string) bool {
	expanded, err := ExpandPath(path)
	if err != nil {
		return false
	}

	exists, _ := pathExists(expanded)
	return exists
}

func IsDir(path string) bool {
	expanded, err := ExpandPath(path)
	if err != nil {
		return false
	}

	info, err := os.Stat(expanded)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}