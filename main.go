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
	// 1. 解析配置目录（-F 参数或 ASSH_CONFIG_DIR 环境变量）
	cfgDir := resolveConfigDir()
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
	connector := sshinfra.NewConnector()
	session := sshinfra.NewSession()

	// 5. 创建 service 层（依赖注入）
	serverSvc := service.NewServerService(repo)
	connectSvc := service.NewConnectService(connector, session, repo)

	// 6. 创建 CLI 应用并运行
	app := cmd.NewApp(Version, Build, connectSvc, serverSvc)
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// resolveConfigDir 从命令行参数或环境变量中解析配置目录路径。
// 优先级：命令行 -F/--config 参数 > ASSH_CONFIG_DIR 环境变量。
// 如果 -F 指定的是文件路径而非目录，返回其所在目录。
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
