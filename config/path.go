// Package config 提供应用配置管理和路径工具。
//
// 管理文件系统路径（数据库文件、密钥文件、配置文件等）的默认值、
// 环境变量展开、目录创建、以及全局配置项的读写。
// 配置值可通过全局变量在运行时动态修改。
package config

import (
	"os"
	"path/filepath"
	"strings"
)

// 默认文件路径常量，使用 ~ 表示用户 home 目录。
// 运行时可通过 ConfigPath 等全局变量动态调整。
var (
	ConfigPath     = "~/.assh/v2"             // 配置目录
	DataPath       = "~/.assh/v2/data"        // 数据目录
	PrivateKeyPath = "~/.assh/v2/.rsa"        // RSA 私钥文件路径
	PublicKeyPath  = "~/.assh/v2/.rsa.pub"    // RSA 公钥文件路径
	PasswordFile   = "~/.assh/v2/.account"    // 密码文件路径
	ConfigFile     = "~/.assh/v2/assh.yml"    // YAML 配置文件路径
	DbFile         = "~/.assh/v2/asshv2.db"   // SQLite 数据库文件路径
	LogPath        = "/tmp/assh.log"          // 日志文件路径
)

// ExpandPath 展开路径中的 ~/ 为用户的 home 目录。
// 支持绝对路径和相对路径的解析。
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

// EnsureDir 确保指定路径的目录存在，不存在时自动创建。
// 如果路径已存在但不是目录，返回错误。
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

// FileExists 检查路径所指的文件或目录是否存在。
func FileExists(path string) bool {
	expanded, err := ExpandPath(path)
	if err != nil {
		return false
	}

	exists, _ := pathExists(expanded)
	return exists
}

// IsDir 判断路径是否为目录。
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

// pathExists 检查文件系统中是否存在指定路径（内部函数，不展开 ~）。
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