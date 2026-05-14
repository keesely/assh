// 命令入口：ASSH - An SSH Client
//
// ASSH 是一个使用 Go 语言开发的 SSH 客户端，适用于服务器管理、维护和快速使用。
// 本文件是程序的入口点（main 函数），负责：
//  1. 解析配置目录路径
//  2. 初始化 SQLite 数据库存储
//  3. 创建 SSH 连接器和会话实现
//  4. 组装 service 层（依赖注入的组合根）
//  5. 启动 CLI 命令处理
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"assh/cmd"
	"assh/asshc/infra/keymgr"
	"assh/asshc/infra/proxy"
	sshinfra "assh/asshc/infra/ssh"
	"assh/asshc/infra/store"
	"assh/asshc/service"
	"assh/config"
)

// Version 是 ASSH 的当前版本号，通过编译时注入（-ldflags）更新。
var Version = "v2.0.0"

// Build 是构建信息（构建时间等），通过编译时注入（-ldflags）更新。
var Build string

func main() {
	// RQ-001: Expand combined short flags (e.g., -qnt -> -q -n -t)
	args := expandCombinedFlags(os.Args)

	// 1. 解析配置目录（-F 参数或 ASSH_CONFIG_DIR 环境变量）
	cfgDir := resolveConfigDir(args)
	if cfgDir != "" {
		config.ConfigPath = cfgDir
		config.SetDbPath(cfgDir + "/asshv2.db")
	}

	// 2. 初始化数据库路径并确保目录存在
	dbPath, err := config.ExpandPath(config.GetDbPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to resolve db path: %v\n", err)
		os.Exit(1)
	}
	config.EnsureDir(filepath.Dir(dbPath))

	// 3. 创建存储层（SQLite + AES 加密）
	repo, err := store.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize store: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	// 4. 创建基础设施实现（SSH 连接器和会话）
	// passphrase 从 KeyManager 获取，用于解密 data/keys/ 中加密的备份密钥
	km, err := keymgr.New(config.KeysDir, nil) // passphrase = nil (account password pending Phase 8)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize key manager: %v\n", err)
		os.Exit(1)
	}
	connector := sshinfra.NewConnectorWithPassphrase(km.GetAccountPassphrase())
	session := sshinfra.NewSession()

	// 5. 创建 service 层（依赖注入）
	serverSvc := service.NewServerService(repo)
	connectSvc := service.NewConnectService(connector, session, repo)

	// 6. 创建密钥管理器服务
	keySvc := service.NewKeyService(km, repo, connector)

	// 7. 创建隧道管理器和代理服务（Phase 7）
	tunnelMgr := proxy.NewTunnelManager()
	proxySvc := service.NewProxyService(repo, connector, connectSvc, tunnelMgr)

	// 8. 创建 CLI 应用并运行
	app := cmd.NewApp(Version, Build, connectSvc, serverSvc, repo, keySvc, km, proxySvc)
	if err := app.Run(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// expandCombinedFlags 将组合短参数展开为独立参数。
// 例如：-qnt -> -q -n -t
// 只对已知的 bool 类型短参数进行展开。
func expandCombinedFlags(args []string) []string {
	if len(args) < 2 {
		return args
	}

	// Known bool flags that can be combined
	knownBoolFlags := map[string]bool{
		"v": true, // verbose
		"q": true, // quiet
		"V": true, // version
		// Note: -e (resume) is also a bool, but we need to handle it carefully
	}

	var result []string
	result = append(result, args[0]) // program name

	for i := 1; i < len(args); i++ {
		arg := args[i]

		// Skip non-flag arguments (like server names, commands, etc.)
		if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
			result = append(result, arg)
			continue
		}

		// Skip single dash "-" or already single-char flags
		if len(arg) == 2 {
			result = append(result, arg)
			continue
		}

		// Try to expand combined flags like -qnt
		flagPart := arg[1:] // Remove leading "-"
		allBool := true
		for _, ch := range flagPart {
			if !knownBoolFlags[string(ch)] {
				allBool = false
				break
			}
		}

		if allBool {
			// Expand: -qnt -> -q -n -t
			for _, ch := range flagPart {
				result = append(result, "-"+string(ch))
			}
		} else {
			// Keep original (will cause error by urfave/cli)
			result = append(result, arg)
		}
	}

	return result
}

// resolveConfigDir 从命令行参数或环境变量中解析配置目录路径。
// 优先级：命令行 -F/--config 参数 > ASSH_CONFIG_DIR 环境变量。
// 如果 -F 指定的是文件路径而非目录，返回其所在目录。
func resolveConfigDir(args []string) string {
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "-F" || arg == "--config" {
			if i+1 < len(args) {
				p := args[i+1]
				if info, err := os.Stat(p); err == nil && !info.IsDir() {
					return filepath.Dir(p)
				}
				return p
			}
		}
		// Stop at first non-flag argument (after all flags have been processed)
		// This handles cases like: -q -v servername or -qv servername
		if !strings.HasPrefix(arg, "-") {
			break
		}
		// Skip values that follow flags (for -F/--config with values)
		// These are already handled above, but we need to skip them to avoid breaking
		if arg == "-F" || arg == "--config" {
			i++ // Skip the next arg (the value)
			continue
		}
	}
	return os.Getenv("ASSH_CONFIG_DIR")
}
