package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"assh/asshc/domain"

	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh"
)

// registerConnectCommands 注册连接相关命令（login/run/bc）。
func (a *App) registerConnectCommands() {
	a.cli.Commands = append(a.cli.Commands, []cli.Command{
		{
			Name:      "login",
			Usage:     "SSH login (auto-detect: name / user@host / -H host)",
			ArgsUsage: "[target]",
			Action:    a.loginAction,
			Flags: []cli.Flag{
				cli.StringFlag{Name: "H, host", Usage: "host address"},
				cli.IntFlag{Name: "p, port", Value: 0, Usage: "port"},
				cli.StringFlag{Name: "u, user", Usage: "username"},
				cli.StringFlag{Name: "l, login", Usage: "username (same as --user)"},
				cli.StringFlag{Name: "P, password", Usage: "password"},
				cli.StringFlag{Name: "i, identity-file", Usage: "identity file path"},
				cli.StringFlag{Name: "k, key", Usage: "key file path (same as --identity-file)"},
			},
		},
		{
			Name:      "run",
			Usage:     "Run a command on a remote server",
			ArgsUsage: "<name> <command>",
			Action:    a.runAction,
			Flags: []cli.Flag{
				cli.IntFlag{Name: "p, port", Value: 0, Usage: "port"},
				cli.StringFlag{Name: "u, user", Usage: "username"},
				cli.StringFlag{Name: "l, login", Usage: "username (same as --user)"},
				cli.StringFlag{Name: "P, password", Usage: "password"},
				cli.StringFlag{Name: "i, identity-file", Usage: "identity file path"},
				cli.StringFlag{Name: "k, key", Usage: "key file path (same as --identity-file)"},
			},
		},
		{
			Name:      "bc",
			Usage:     "Batch execute command on multiple servers",
			ArgsUsage: "<command>",
			Action:    a.bcAction,
			Flags: []cli.Flag{
				cli.StringFlag{Name: "servers", Usage: "comma-separated server list"},
				cli.StringFlag{Name: "group", Usage: "server group"},
				cli.IntFlag{Name: "p, port", Value: 0, Usage: "port override"},
				cli.StringFlag{Name: "u, user", Usage: "username override"},
				cli.StringFlag{Name: "l, login", Usage: "username (same as --user)"},
				cli.StringFlag{Name: "i, identity-file", Usage: "identity file path"},
				cli.StringFlag{Name: "k, key", Usage: "key file path (same as --identity-file)"},
				cli.StringFlag{Name: "P, password", Usage: "password override"},
				cli.StringFlag{Name: "log", Usage: "output log path (default: ./bc-result-{timestamp}.json)"},
			},
		},
	}...)
}

// loginCore 执行登录逻辑，返回已连接的 SSH 客户端。
// 三种目标识别方式：
//   - 已保存的服务器名称（从数据库中读取配置）
//   - user@host 格式（直接指定）
//   - -H host 参数（直接指定）
//
// 注意：当通过默认 Action（assh <target> -p 9999）触发时，urfave/cli
// 不会解析位置参数后的全局 flags，需要手动回落查找短参数（v1 兼容）。
func (a *App) loginCore(c *cli.Context) (*ssh.Client, error) {
	target := c.Args().Get(0)
	host := resolveFlag(c, "host", "H")
	port := resolveIntFlag(c, "port", "p")
	user := firstNonEmpty(resolveFlag(c, "user", "u"), resolveFlag(c, "login", "l"))
	password := resolveFlag(c, "password", "P")
	keyFile := firstNonEmpty(resolveFlag(c, "identity-file", "i"), resolveFlag(c, "key", "k"))

	if target == "" && host == "" {
		return nil, fmt.Errorf("no target specified: use <name>, <user@host>, or -H <host>")
	}
	if target != "" && host != "" {
		return nil, fmt.Errorf("cannot specify both target and -H/--host")
	}

	switch {
	case strings.Contains(target, "@"):
		parts := strings.SplitN(target, "@", 2)
		user, host = parts[0], parts[1]
		if user == "" {
			user = "root"
		}
		if port <= 0 {
			port = 22
		}
		return a.connectSvc.ConnectDirect(host, port, user, password, keyFile)

	case host != "":
		if user == "" {
			user = "root"
		}
		if port <= 0 {
			port = 22
		}
		return a.connectSvc.ConnectDirect(host, port, user, password, keyFile)

	default:
		return a.connectSvc.ConnectByName(target)
	}
}

// loginAction 处理 login 命令（显式登录）。
func (a *App) loginAction(c *cli.Context) error {
	client, err := a.loginCore(c)
	if err != nil {
		return err
	}
	defer a.connectSvc.Close(client)

	return a.connectSvc.Shell(client)
}

// runAction 处理 run 命令，在指定服务器上执行单条命令。
func (a *App) runAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("server name is required")
	}

	cmd := strings.Join(c.Args()[1:], " ")
	if cmd == "" {
		return fmt.Errorf("command is required")
	}

	client, err := a.connectSvc.ConnectByName(name)
	if err != nil {
		return err
	}
	defer a.connectSvc.Close(client)

	return a.connectSvc.Run(client, cmd)
}

// bcServerResult 记录批处理命令在单台服务器上的执行结果。
type bcServerResult struct {
	Name   string `json:"name"`   // 服务器名称
	Host   string `json:"host"`   // 主机地址
	Port   int    `json:"port"`   // 端口号
	User   string `json:"user"`   // 用户名
	Stdout string `json:"stdout"` // 标准输出内容
	Stderr string `json:"stderr"` // 错误输出内容
	Error  string `json:"error"`  // 错误信息（执行失败时）
}

// bcSummary 批处理命令的整体执行摘要。
type bcSummary struct {
	Command   string           `json:"command"`   // 执行的命令
	Timestamp string           `json:"timestamp"` // 执行时间戳
	Total     int              `json:"total"`     // 总服务器数
	Success   int              `json:"success"`   // 成功数
	Failed    int              `json:"failed"`    // 失败数
	Servers   []bcServerResult `json:"servers"`   // 各服务器结果明细
}

// bcAction 处理 bc 命令，在多台服务器上并发执行同一命令。
// 支持通过 --servers（逗号分隔）和 --group（服务器分组）指定目标服务器列表。
// 结果输出到 JSON 文件，同时实时打印到标准输出。
func (a *App) bcAction(c *cli.Context) error {
	cmd := c.Args().Get(0)
	if cmd == "" {
		return fmt.Errorf("command is required")
	}

	serversFlag := c.String("servers")
	groupFlag := c.String("group")

	if serversFlag == "" && groupFlag == "" {
		return fmt.Errorf("either --servers or --group is required")
	}

	var names []string
	if serversFlag != "" {
		for _, s := range strings.Split(serversFlag, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				names = append(names, s)
			}
		}
	}
	if groupFlag != "" {
		groupServers, err := a.serverSvc.GetGroup(groupFlag)
		if err != nil {
			return fmt.Errorf("failed to get group %q: %w", groupFlag, err)
		}
		for name, server := range groupServers {
			names = append(names, domain.JoinName(server.Group, name))
		}
	}

	if len(names) == 0 {
		return fmt.Errorf("no servers to execute on")
	}

	logDir := "/tmp/assh/bclogs"
	os.MkdirAll(logDir, 0755)
	timestamp := time.Now()

	var (
		wg         sync.WaitGroup
		stdoutMu   sync.Mutex
		resultsMu  sync.Mutex
		allResults []bcServerResult
	)

	// 并发对每台服务器执行命令
	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()

			result := bcServerResult{Name: n}

			client, err := a.connectSvc.ConnectByName(n)
			if err != nil {
				result.Error = err.Error()
				writeLogFile(logDir+"/"+n+".log", "error: "+err.Error())
				stdoutMu.Lock()
				fmt.Printf("[%s] error: %v\n", n, err)
				stdoutMu.Unlock()

				resultsMu.Lock()
				allResults = append(allResults, result)
				resultsMu.Unlock()
				return
			}
			defer a.connectSvc.Close(client)

			if srv, err := a.serverSvc.GetServer(n); err == nil && srv != nil {
				result.Host = srv.Host
				result.Port = srv.Port
				result.User = srv.User
			}

			output, runErr := a.connectSvc.RunWithOutput(client, cmd)
			if runErr != nil {
				result.Error = runErr.Error()
				result.Stdout = output
				writeLogFile(logDir+"/"+n+".log", output)
				writeLogFile(logDir+"/"+n+".log", "error: "+runErr.Error())

				stdoutMu.Lock()
				fmt.Printf("[%s] error: %v\n", n, runErr)
				stdoutMu.Unlock()
			} else {
				result.Stdout = output
				writeLogFile(logDir+"/"+n+".log", output)

				stdoutMu.Lock()
				for _, line := range strings.Split(output, "\n") {
					if line != "" {
						fmt.Printf("[%s] %s\n", n, line)
					}
				}
				stdoutMu.Unlock()
			}

			resultsMu.Lock()
			allResults = append(allResults, result)
			resultsMu.Unlock()
		}(name)
	}

	wg.Wait()

	summary := bcSummary{
		Command:   cmd,
		Timestamp: timestamp.Format(time.RFC3339),
		Total:     len(allResults),
		Servers:   allResults,
	}
	for _, r := range allResults {
		if r.Error != "" {
			summary.Failed++
		} else {
			summary.Success++
		}
	}

	jsonData, _ := json.MarshalIndent(summary, "", "  ")

	logPath := c.String("log")
	if logPath == "" {
		logPath = fmt.Sprintf("./bc-result-%s.json", timestamp.Format("20060102T150405"))
	}

	if err := os.WriteFile(logPath, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write result: %v\n", err)
	} else {
		fmt.Printf("result written to %s\n", logPath)
	}

	return nil
}

// writeLogFile 将内容追加写入日志文件，文件不存在时自动创建。
func writeLogFile(filePath, content string) {
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(content + "\n")
}

// resolveFlag 从 cli 上下文中获取字符串参数值。
// 优先使用 cli 正常解析的值（c.IsSet 为 true），
// 如果未解析则从 c.Args() 中手动查找短参数（v1 兼容）。
func resolveFlag(c *cli.Context, longName, shortName string) string {
	if c.IsSet(longName) {
		return c.String(longName)
	}
	if v, ok := lookupShortFlag(c, shortName); ok {
		return v
	}
	return ""
}

// resolveIntFlag 从 cli 上下文中获取整数参数值。
// 优先使用 cli 正常解析的值，否则手动查找短参数。
func resolveIntFlag(c *cli.Context, longName, shortName string) int {
	if c.IsSet(longName) {
		return c.Int(longName)
	}
	if v, ok := lookupShortFlag(c, shortName); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

// lookupShortFlag 从 c.Args() 中手动查找短参数（v1 兼容）。
// urfave/cli 在默认 Action 中不会解析位置参数后的全局 flags，
// 需要手动从剩余参数中查找 -p、-P 等短参数。
// 支持 -p 9999（空格分隔）和 -p9999（紧凑）两种格式。
func lookupShortFlag(c *cli.Context, shortName string) (string, bool) {
	prefix := "-" + shortName
	args := c.Args()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// 紧凑格式：-p9999
		if strings.HasPrefix(arg, prefix) && len(arg) > len(prefix) {
			return arg[len(prefix):], true
		}
		// 空格分隔：-p 9999
		if arg == prefix && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}
