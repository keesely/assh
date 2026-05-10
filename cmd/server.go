package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"assh/asshc/domain"

	"github.com/urfave/cli"
	"golang.org/x/term"
)

// registerServerCommands 注册顶层 server 命令（add/set/ls/info/rm/mv/rollback）。
func (a *App) registerServerCommands() {
	a.cli.Commands = append(a.cli.Commands, []cli.Command{
		{
			Name:      "add",
			Usage:     "Add a server",
			ArgsUsage: "<name>",
			Action:    a.serverAddAction,
			Flags: []cli.Flag{
				cli.StringFlag{Name: "H, host", Usage: "host address (required)"},
				cli.IntFlag{Name: "p, port", Value: 22, Usage: "port"},
				cli.StringFlag{Name: "u, user", Usage: "username"},
				cli.StringFlag{Name: "l, login", Usage: "username (same as --user)"},
				cli.StringFlag{Name: "P, password", Usage: "password (omit for interactive prompt)"},
				cli.StringFlag{Name: "i, identity-file", Usage: "identity file path"},
				cli.StringFlag{Name: "k, key", Usage: "key file path (same as --identity-file)"},
				cli.StringSliceFlag{Name: "o, option", Usage: "option in key=value format"},
				cli.StringFlag{Name: "remark", Usage: "remark"},
				cli.StringFlag{Name: "group", Usage: "group"},
			},
		},
		{
			Name:      "set",
			Usage:     "Create or update server parameters (upsert)",
			ArgsUsage: "<name>",
			Action:    a.serverSetAction,
			Flags: []cli.Flag{
				cli.StringFlag{Name: "H, host", Usage: "host address (required for new)"},
				cli.IntFlag{Name: "p, port", Usage: "port"},
				cli.StringFlag{Name: "u, user", Usage: "username"},
				cli.StringFlag{Name: "l, login", Usage: "username (same as --user)"},
				cli.StringFlag{Name: "P, password", Usage: "password"},
				cli.StringFlag{Name: "i, identity-file", Usage: "identity file path"},
				cli.StringFlag{Name: "k, key", Usage: "key file path (same as --identity-file)"},
				cli.StringSliceFlag{Name: "o, option", Usage: "add/replace option in key=value format"},
				cli.StringFlag{Name: "remark", Usage: "remark"},
				cli.BoolFlag{Name: "clear-password", Usage: "clear the password"},
				cli.BoolFlag{Name: "clear-key", Usage: "clear the key file"},
			},
		},
		{
			Name:   "ls",
			Usage:  "List servers",
			Action: a.serverListAction,
			Flags: []cli.Flag{
				cli.StringFlag{Name: "group", Usage: "filter by group"},
				cli.StringFlag{Name: "search", Usage: "search by keyword"},
			},
		},
		{
			Name:      "info",
			Usage:     "Show server details",
			ArgsUsage: "<name>",
			Action:    a.serverInfoAction,
		},
		{
			Name:      "rm",
			Usage:     "Remove a server",
			ArgsUsage: "<name>",
			Action:    a.serverRemoveAction,
		},
		{
			Name:      "mv",
			Usage:     "Rename/move a server",
			ArgsUsage: "<from> <to>",
			Action:    a.serverMoveAction,
		},
		{
			Name:      "rollback",
			Usage:     "Rollback server configuration to a previous version",
			ArgsUsage: "<name> [version]",
			Action:    a.serverRollbackAction,
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "list", Usage: "show changelog versions"},
			},
		},
	}...)
}

// serverAddAction 处理 server add 命令。
// 如果未指定密码且未指定密钥文件，交互式提示用户输入密码。
// 支持 --group 参数，在添加时自动为名称添加分组前缀。
func (a *App) serverAddAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("server name is required")
	}

	host := c.String("host")
	if host == "" {
		return fmt.Errorf("--host/-H is required")
	}

	if group := c.String("group"); group != "" {
		name = group + "." + name
	}

	port := c.Int("port")
	user := firstNonEmpty(c.String("user"), c.String("login"))
	password := c.String("password")
	keyFile := firstNonEmpty(c.String("identity-file"), c.String("key"))

	if password == "" && keyFile == "" {
		fmt.Fprint(os.Stderr, "Enter password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err == nil && len(passwordBytes) > 0 {
			password = string(passwordBytes)
		}
	}

	auth := &domain.Auth{
		Password: password,
		KeyFile:  keyFile,
	}
	if auth.Password == "" && auth.KeyFile == "" {
		auth = nil
	}

	options := make(map[string]interface{})
	for _, opt := range c.StringSlice("option") {
		parts := strings.SplitN(opt, "=", 2)
		if len(parts) == 2 {
			options[parts[0]] = parts[1]
		}
	}

	server := &domain.Server{
		Host:    host,
		Port:    port,
		User:    user,
		Auth:    auth,
		Remark:  c.String("remark"),
		Options: options,
	}

	if err := a.serverSvc.AddServer(name, server); err != nil {
		return err
	}

	fmt.Printf("server %q added (v%d)\n", name, 1)
	return nil
}

// serverSetAction 处理 server set 命令（upsert 语义）。
// 如果服务器不存在则创建，存在则部分更新（仅修改通过标志指定的字段）。
// 支持 --clear-password 和 --clear-key 清除认证信息。
func (a *App) serverSetAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("server name is required")
	}

	existing, err := a.serverSvc.GetServer(name)
	isNew := err != nil

	// 检查是否有任何修改标志被设置
	modFlags := []string{"host", "port", "user", "login", "password",
		"identity-file", "key", "remark", "clear-password", "clear-key"}
	hasChanges := false
	for _, f := range modFlags {
		if c.IsSet(f) {
			hasChanges = true
			break
		}
	}
	if !hasChanges && len(c.StringSlice("option")) > 0 {
		hasChanges = true
	}

	if isNew {
		// --- 创建分支 ---
		if !hasChanges {
			return fmt.Errorf("--host/-H is required for a new server")
		}

		host := c.String("host")
		if host == "" {
			return fmt.Errorf("--host/-H is required for a new server")
		}

		port := 22
		if c.IsSet("port") {
			port = c.Int("port")
		}
		if port < 1 || port > 65535 {
			return domain.ErrInvalidPort
		}

		user := firstNonEmpty(c.String("user"), c.String("login"))
		if user == "" {
			user = "root"
		}

		password := c.String("password")
		keyFile := firstNonEmpty(c.String("identity-file"), c.String("key"))

		auth := &domain.Auth{
			Password: password,
			KeyFile:  keyFile,
		}
		if auth.Password == "" && auth.KeyFile == "" {
			auth = nil
		}

		options := make(map[string]interface{})
		for _, opt := range c.StringSlice("option") {
			parts := strings.SplitN(opt, "=", 2)
			if len(parts) == 2 {
				options[parts[0]] = parts[1]
			}
		}

		server := &domain.Server{
			Host:    host,
			Port:    port,
			User:    user,
			Auth:    auth,
			Remark:  c.String("remark"),
			Options: options,
		}

		if err := a.serverSvc.SetServer(name, server); err != nil {
			return err
		}

		fmt.Printf("server %q created (v%d)\n", name, 1)
	} else {
		// --- 更新分支 ---
		if !hasChanges {
			a.printServerDetail(existing)
			fmt.Println("\nhint: use flags to modify parameters, e.g. --host, -p, -P, --remark")
			return nil
		}

		// 深拷贝现有配置，仅修改显式设置的字段
		updated := *existing
		if existing.Options != nil {
			updated.Options = make(map[string]interface{})
			for k, v := range existing.Options {
				updated.Options[k] = v
			}
		} else {
			updated.Options = make(map[string]interface{})
		}
		if existing.Auth != nil {
			authCopy := *existing.Auth
			updated.Auth = &authCopy
		}

		if c.IsSet("host") {
			updated.Host = c.String("host")
		}
		if c.IsSet("port") {
			p := c.Int("port")
			if p < 1 || p > 65535 {
				return domain.ErrInvalidPort
			}
			updated.Port = p
		}
		if c.IsSet("user") || c.IsSet("login") {
			updated.User = firstNonEmpty(c.String("user"), c.String("login"))
		}
		if c.IsSet("password") {
			if updated.Auth == nil {
				updated.Auth = &domain.Auth{}
			}
			updated.Auth.Password = c.String("password")
		}
		if c.IsSet("identity-file") || c.IsSet("key") {
			keyFile := firstNonEmpty(c.String("identity-file"), c.String("key"))
			if updated.Auth == nil {
				updated.Auth = &domain.Auth{}
			}
			updated.Auth.KeyFile = keyFile
		}
		if opts := c.StringSlice("option"); len(opts) > 0 {
			for _, opt := range opts {
				parts := strings.SplitN(opt, "=", 2)
				if len(parts) == 2 {
					updated.Options[parts[0]] = parts[1]
				}
			}
		}
		if c.IsSet("remark") {
			updated.Remark = c.String("remark")
		}

		if c.Bool("clear-password") {
			if updated.Auth != nil {
				updated.Auth.Password = ""
			}
		}
		if c.Bool("clear-key") {
			if updated.Auth != nil {
				updated.Auth.KeyFile = ""
			}
		}

		if updated.Auth != nil && updated.Auth.Password == "" && updated.Auth.KeyFile == "" {
			updated.Auth = nil
		}

		if err := a.serverSvc.SetServer(name, &updated); err != nil {
			return err
		}

		// 更新后读取最新版本号并显示
		after, err := a.serverSvc.GetServer(name)
		if err == nil {
			fmt.Printf("server %q updated (v%d)\n", name, after.Version)
		} else {
			fmt.Printf("server %q updated\n", name)
		}
	}

	return nil
}

// serverRollbackAction 处理 server rollback 命令。
// 使用 --list 标志查看可回滚的版本列表，或指定回滚到目标版本。
func (a *App) serverRollbackAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("server name is required")
	}

	// --list 模式：显示变更历史
	if c.Bool("list") {
		entries, err := a.serverSvc.GetServerChangelog(name)
		if err != nil {
			return err
		}
		fmt.Printf("Changelog for %q:\n", name)
		fmt.Printf("%-8s %-12s %-19s\n", "Version", "Type", "Date")
		fmt.Println(strings.Repeat("-", 42))
		for _, e := range entries {
			date := e.CreatedAt
			if len(date) > 19 {
				date = date[:19]
			}
			fmt.Printf("%-8d %-12s %-19s\n", e.Version, e.ChangeType, date)
		}
		return nil
	}

	// 回滚到指定版本
	versionStr := c.Args().Get(1)
	if versionStr == "" {
		return fmt.Errorf("version number is required (use --list to see available versions)")
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return fmt.Errorf("invalid version number: %s", versionStr)
	}

	if err := a.serverSvc.RollbackServer(name, version); err != nil {
		return err
	}

	fmt.Printf("server %q rolled back to v%d\n", name, version)
	return nil
}

// printServerDetail 打印服务器配置的详细信息，用于 server info 命令。
func (a *App) printServerDetail(s *domain.Server) {
	fullName := s.Name
	if s.Group != "" {
		fullName = domain.JoinName(s.Group, s.Name)
	}

	fmt.Printf("Name:    %s\n", fullName)
	fmt.Printf("Host:    %s\n", s.Host)
	fmt.Printf("Port:    %d\n", s.Port)
	fmt.Printf("User:    %s\n", s.User)
	if s.Auth != nil {
		switch {
		case s.Auth.KeyFile != "":
			fmt.Printf("Auth:    key (%s)\n", s.Auth.KeyFile)
		case s.Auth.Password != "":
			fmt.Printf("Auth:    password\n")
		default:
			fmt.Printf("Auth:    none\n")
		}
	} else {
		fmt.Printf("Auth:    none\n")
	}
	if s.Remark != "" {
		fmt.Printf("Remark:  %s\n", s.Remark)
	}
	for k, v := range s.Options {
		fmt.Printf("Option:  %s=%v\n", k, v)
	}
	fmt.Printf("Version: %d\n", s.Version)
}

// serverListAction 处理 server ls 命令，支持按分组和关键字筛选。
func (a *App) serverListAction(c *cli.Context) error {
	group := c.String("group")
	search := c.String("search")

	var result map[string]map[string]*domain.Server
	var err error

	switch {
	case search != "":
		result, err = a.serverSvc.SearchServers(search)
	case group != "":
		var servers map[string]*domain.Server
		servers, err = a.serverSvc.GetGroup(group)
		if err == nil {
			result = map[string]map[string]*domain.Server{group: servers}
		}
	default:
		result, err = a.serverSvc.ListServers()
	}

	if err != nil {
		return err
	}

	if len(result) == 0 {
		fmt.Println("no servers found")
		return nil
	}

	var groupColors = []int{32, 34, 33, 35, 36}

	fmt.Printf("%-16s %-20s %-28s %-4s %-14s %s\n", "Group", "Name", "Host", "Ver", "Auth", "Remark")
	fmt.Println(strings.Repeat("-", 88))

	colorIdx := 0
	for g, servers := range result {
		baseColor := groupColors[colorIdx%len(groupColors)]
		colorIdx++
		row := 0
		for _, s := range servers {
			hostStr := fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)

			auth := "none"
			if s.Auth != nil {
				switch {
				case s.Auth.Password != "" && s.Auth.KeyFile != "":
					auth = "password+key"
				case s.Auth.Password != "":
					auth = "password"
				case s.Auth.KeyFile != "":
					auth = "key"
				}
			}

			var bold int
			if row%2 == 0 {
				bold = 1
			} else {
				bold = 0
			}

			fmt.Printf("\033[%d;%dm%-16s %-20s %-28s %-4d %-14s %s\033[0m\n",
				bold, baseColor, g, s.Name, hostStr, s.Version, auth, s.Remark)
			row++
		}
	}

	return nil
}

// serverInfoAction 处理 server info 命令，显示服务器的完整配置详情。
func (a *App) serverInfoAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("server name is required")
	}

	server, err := a.serverSvc.GetServer(name)
	if err != nil {
		return err
	}

	a.printServerDetail(server)
	return nil
}

// serverRemoveAction 处理 server rm 命令，删除指定服务器配置。
func (a *App) serverRemoveAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("server name is required")
	}

	if err := a.serverSvc.RemoveServer(name); err != nil {
		return err
	}

	fmt.Printf("server %q removed\n", name)
	return nil
}

// serverMoveAction 处理 server mv 命令，重命名或跨分组移动服务器。
func (a *App) serverMoveAction(c *cli.Context) error {
	from := c.Args().Get(0)
	to := c.Args().Get(1)

	if from == "" || to == "" {
		return fmt.Errorf("both <from> and <to> names are required")
	}

	if err := a.serverSvc.MoveServer(from, to); err != nil {
		return err
	}

	fmt.Printf("server %q moved to %q\n", from, to)
	return nil
}
