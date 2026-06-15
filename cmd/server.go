package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"assh/asshc/domain"
	"assh/internal/i18n"

	"github.com/urfave/cli"
	"golang.org/x/term"
)

// registerServerCommands 注册顶层 server 命令（add/set/ls/info/rm/mv/rollback）。
func (a *App) registerServerCommands() {
	a.cli.Commands = append(a.cli.Commands, []cli.Command{
		{
			Name:      "add",
			Usage:     i18n.T("cmd.server.add.description"),
			ArgsUsage: "<name>",
			Action:    a.serverAddAction,
			Flags: []cli.Flag{
				cli.StringFlag{Name: "H, host", Usage: i18n.T("cmd.server.flag.host")},
				cli.IntFlag{Name: "p, port", Value: 22, Usage: i18n.T("cmd.server.flag.port")},
				cli.StringFlag{Name: "u, user", Usage: i18n.T("cmd.server.flag.user")},
				cli.StringFlag{Name: "l, login", Usage: i18n.T("cmd.server.flag.login")},
				cli.StringFlag{Name: "P, password", Usage: i18n.T("cmd.server.flag.password")},
				cli.StringFlag{Name: "i, identity-file", Usage: i18n.T("cmd.server.flag.identity_file")},
				cli.StringFlag{Name: "k, key", Usage: i18n.T("cmd.server.flag.key")},
				cli.StringSliceFlag{Name: "o, option", Usage: i18n.T("cmd.server.flag.option")},
				cli.StringFlag{Name: "remark", Usage: i18n.T("cmd.server.flag.remark")},
				cli.StringFlag{Name: "group", Usage: i18n.T("cmd.server.flag.group")},
			},
		},
		{
			Name:      "set",
			Usage:     i18n.T("cmd.server.set.description"),
			ArgsUsage: "<name>",
			Action:    a.serverSetAction,
			Flags: []cli.Flag{
				cli.StringFlag{Name: "H, host", Usage: i18n.T("cmd.server.flag.host_new")},
				cli.IntFlag{Name: "p, port", Usage: i18n.T("cmd.server.flag.port")},
				cli.StringFlag{Name: "u, user", Usage: i18n.T("cmd.server.flag.user")},
				cli.StringFlag{Name: "l, login", Usage: i18n.T("cmd.server.flag.login")},
				cli.StringFlag{Name: "P, password", Usage: i18n.T("cmd.server.flag.password")},
				cli.StringFlag{Name: "i, identity-file", Usage: i18n.T("cmd.server.flag.identity_file")},
				cli.StringFlag{Name: "k, key", Usage: i18n.T("cmd.server.flag.key")},
				cli.StringSliceFlag{Name: "o, option", Usage: i18n.T("cmd.server.flag.option_replace")},
				cli.StringFlag{Name: "remark", Usage: i18n.T("cmd.server.flag.remark")},
				cli.BoolFlag{Name: "clear-password", Usage: i18n.T("cmd.server.flag.clear_password")},
				cli.BoolFlag{Name: "clear-key", Usage: i18n.T("cmd.server.flag.clear_key")},
			},
		},
		{
			Name:   "ls",
			Usage:  i18n.T("cmd.server.ls.description"),
			Action: a.serverListAction,
			Flags: []cli.Flag{
				cli.StringFlag{Name: "group", Usage: i18n.T("cmd.server.flag.filter_group")},
				cli.StringFlag{Name: "search", Usage: i18n.T("cmd.server.flag.search")},
			},
		},
		{
			Name:      "info",
			Usage:     i18n.T("cmd.server.info.description"),
			ArgsUsage: "<name>",
			Action:    a.serverInfoAction,
		},
		{
			Name:      "rm",
			Usage:     i18n.T("cmd.server.rm.description"),
			ArgsUsage: "<name>",
			Action:    a.serverRemoveAction,
		},
		{
			Name:      "mv",
			Usage:     i18n.T("cmd.server.mv.description"),
			ArgsUsage: "<from> <to>",
			Action:    a.serverMoveAction,
		},
		{
			Name:      "rollback",
			Usage:     i18n.T("cmd.server.rollback.description"),
			ArgsUsage: "<name> [version]",
			Action:    a.serverRollbackAction,
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "list", Usage: i18n.T("cmd.server.flag.list")},
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
		return fmt.Errorf("%s", i18n.T("cmd.server.error.name_required"))
	}

	host := c.String("host")
	if host == "" {
		return fmt.Errorf("%s", i18n.T("cmd.server.error.host_required"))
	}

	if group := c.String("group"); group != "" {
		name = group + "." + name
	}

	port := c.Int("port")
	user := firstNonEmpty(c.String("user"), c.String("login"))
	password := c.String("password")
	keyFile := firstNonEmpty(c.String("identity-file"), c.String("key"))

	if password == "" && keyFile == "" {
		fmt.Fprint(os.Stderr, i18n.T("cmd.server.add.enter_password"))
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err == nil && len(passwordBytes) > 0 {
			password = string(passwordBytes)
		}
	}

	// 1) Handle -k flag before saving (P6.4-c enhancement)
	// 先用原始路径构建 auth，创建服务器后备份并更新 Auth.KeyFile
	keyFlag := c.String("key")
	identityFileFlag := c.String("identity-file")
	keyValue := firstNonEmpty(keyFlag, identityFileFlag)

	// 构建 auth 对象（使用原始路径，备份逻辑在创建后执行）
	auth := &domain.Auth{
		Password: password,
		KeyFile:  keyValue,
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

	// 2) Preview
	a.previewServer(server, name)

	// 3) Confirm
	if !a.askConfirmation(fmt.Sprintf(i18n.T("cmd.server.add.description")+" %q? ", name)) {
		fmt.Println(i18n.T("common.cancelled"))
		return nil
	}

	// 4) Execute (single save)
	if err := a.serverSvc.AddServer(name, server); err != nil {
		return err
	}

	// 5) Handle -k after server is created (备份/生成密钥)
	if keyValue != "" && a.keySvc != nil {
		updatedPath, err := a.keySvc.HandleKeyFlag(name, keyValue)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cmd.server.warning.key_failed", map[string]interface{}{"Error": err}))
		} else if updatedPath != "" {
			fmt.Println(i18n.T("cmd.server.key_file", map[string]interface{}{"Path": updatedPath}))
		}
	}

	fmt.Println(i18n.T("cmd.server.add.success", map[string]interface{}{"Name": name, "Version": 1}))
	return nil
}

// serverSetAction 处理 server set 命令（upsert 语义）。
// 如果服务器不存在则创建，存在则部分更新（仅修改通过标志指定的字段）。
// 支持 --clear-password 和 --clear-key 清除认证信息。
func (a *App) serverSetAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("%s", i18n.T("cmd.server.error.name_required"))
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

	// 构建待保存的服务器对象
	var serverToSave *domain.Server
	action := "update" // or "create"

	if isNew {
		// --- 创建分支 ---
		if !hasChanges {
			return fmt.Errorf("%s", i18n.T("cmd.server.error.host_required_new"))
		}

		host := c.String("host")
		if host == "" {
			return fmt.Errorf("%s", i18n.T("cmd.server.error.host_required_new"))
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
		keyValue := firstNonEmpty(c.String("identity-file"), c.String("key"))

		options := make(map[string]interface{})
		for _, opt := range c.StringSlice("option") {
			parts := strings.SplitN(opt, "=", 2)
			if len(parts) == 2 {
				options[parts[0]] = parts[1]
			}
		}

		auth := &domain.Auth{
			Password: password,
		}

		// P6.4-c enhancement: 处理 -k 参数（生成/导入/备份）
		if keyValue != "" && a.keySvc != nil {
			// 先创建服务器，再处理密钥
			serverToSave = &domain.Server{
				Host:    host,
				Port:    port,
				User:    user,
				Auth:    auth,
				Remark:  c.String("remark"),
				Options: options,
			}

			// 1) 创建服务器
			if err := a.serverSvc.AddServer(name, serverToSave); err != nil {
				return err
			}

			// 2) 处理密钥（生成/导入/备份）
			keyFilePath, err := a.keySvc.HandleKeyFlag(name, keyValue)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cmd.server.warning.key_failed", map[string]interface{}{"Error": err}))
			} else if keyFilePath != "" {
				fmt.Println(i18n.T("cmd.server.key_file", map[string]interface{}{"Path": keyFilePath}))
			}

			// 3) 重新获取服务器信息（包含更新的密钥路径）
			serverToSave, _ = a.serverSvc.GetServer(name)
			action = "create"
		} else {
			// 无 -k 参数，直接设置密钥路径
			auth.KeyFile = keyValue
			serverToSave = &domain.Server{
				Host:    host,
				Port:    port,
				User:    user,
				Auth:    auth,
				Remark:  c.String("remark"),
				Options: options,
			}
			action = "create"
		}
	} else {
		// --- 更新分支 ---
		if !hasChanges {
			a.printServerDetail(existing)
			fmt.Println("\n" + i18n.T("cmd.server.set.hint"))
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
			// P6.4-c enhancement: 使用 HandleKeyFlag 处理密钥（生成/导入/备份）
			keyFlag := c.String("key")
			identityFileFlag := c.String("identity-file")
			keyValue := firstNonEmpty(keyFlag, identityFileFlag)
			if keyValue != "" && a.keySvc != nil {
				keyFilePath, err := a.keySvc.HandleKeyFlag(name, keyValue)
				if err != nil {
					return fmt.Errorf("handle key: %w", err)
				}
				if updated.Auth == nil {
					updated.Auth = &domain.Auth{}
				}
				updated.Auth.KeyFile = keyFilePath
			} else {
				// 空值或不带 KeyService 时，直接设置路径
				keyFile := firstNonEmpty(c.String("identity-file"), c.String("key"))
				if updated.Auth == nil {
					updated.Auth = &domain.Auth{}
				}
				updated.Auth.KeyFile = keyFile
			}
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

		serverToSave = &updated
		action = "update"
	}

	// 1) Preview
	a.previewServer(serverToSave, name)

	// 2) Confirm
	confirmMsg := fmt.Sprintf("%s server %q? ", action, name)
	if !a.askConfirmation(confirmMsg) {
		fmt.Println(i18n.T("common.cancelled"))
		return nil
	}

	// 3) Execute
	if err := a.serverSvc.SetServer(name, serverToSave); err != nil {
		return err
	}

	if action == "create" {
		fmt.Println(i18n.T("cmd.server.set.created", map[string]interface{}{"Name": name, "Version": 1}))
	} else {
		after, err := a.serverSvc.GetServer(name)
		if err == nil {
			fmt.Println(i18n.T("cmd.server.set.success", map[string]interface{}{"Name": name, "Version": after.Version}))
		} else {
			fmt.Println(i18n.T("cmd.server.set.success_no_version", map[string]interface{}{"Name": name}))
		}
	}

	return nil
}

// serverRollbackAction 处理 server rollback 命令。
// 使用 --list 标志查看可回滚的版本列表，或指定回滚到目标版本。
func (a *App) serverRollbackAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("%s", i18n.T("cmd.server.error.name_required"))
	}

	// --list 模式：显示变更历史
	if c.Bool("list") {
		entries, err := a.serverSvc.GetServerChangelog(name)
		if err != nil {
			return err
		}
		fmt.Println(i18n.T("cmd.server.rollback.changelog.title", map[string]interface{}{"Name": name}))
		fmt.Printf("%-8s %-12s %-19s\n",
			i18n.T("cmd.server.rollback.changelog.header.version"),
			i18n.T("cmd.server.rollback.changelog.header.type"),
			i18n.T("cmd.server.rollback.changelog.header.date"))
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
		return fmt.Errorf("%s", i18n.T("cmd.server.rollback.error.version_required"))
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return fmt.Errorf("%s", i18n.T("cmd.server.rollback.error.invalid_version", map[string]interface{}{"Version": versionStr}))
	}

	if err := a.serverSvc.RollbackServer(name, version); err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.server.rollback.success", map[string]interface{}{"Name": name, "Version": version}))
	return nil
}

// printServerDetail 打印服务器配置的详细信息，用于 server info 命令。
// 输出格式：
//   --------------------------------
//   +   server-name (ver. N)    +
//   ================================
//        Server Name: value
//        Server Host: value
//                 ...
func (a *App) printServerDetail(s *domain.Server) {
	fullName := s.Name
	if s.Group != "" {
		fullName = domain.JoinName(s.Group, s.Name)
	}

	// ---- header (title centered, + aligned to separators) ----
	const headerWidth = 40
	title := fmt.Sprintf("%s (ver. %d)", fullName, s.Version)
	inner := headerWidth - 2 // exclude the two + signs
	leftPad := (inner - len(title)) / 2
	rightPad := inner - len(title) - leftPad

	fmt.Println(strings.Repeat("-", headerWidth))
	fmt.Printf("+%s%s%s+\n", strings.Repeat(" ", leftPad), title, strings.Repeat(" ", rightPad))
	fmt.Println(strings.Repeat("=", headerWidth))

	// ---- detail items (label right-aligned, no excess indent) ----
	const labelWidth = 11
	pl := func(label string, value interface{}) {
		fmt.Printf("%*s: %v\n", labelWidth, label, value)
	}

	pl(i18n.T("cmd.server.detail.name"), fullName)
	pl(i18n.T("cmd.server.detail.host"), s.Host)
	pl(i18n.T("cmd.server.detail.port"), s.Port)
	pl(i18n.T("cmd.server.detail.user"), s.User)

	if s.Auth != nil {
		hasPwd := s.Auth.Password != ""
		hasKey := s.Auth.KeyFile != ""
		switch {
		case hasPwd && hasKey:
			pl(i18n.T("cmd.server.detail.auth_type"), i18n.T("cmd.server.auth.password_key"))
		case hasPwd:
			pl(i18n.T("cmd.server.detail.auth_type"), i18n.T("cmd.server.auth.password"))
		case hasKey:
			pl(i18n.T("cmd.server.detail.auth_type"), i18n.T("cmd.server.auth.key"))
		default:
			pl(i18n.T("cmd.server.detail.auth_type"), i18n.T("cmd.server.auth.none"))
		}
		if hasPwd {
			pl(i18n.T("cmd.server.detail.password"), s.Auth.Password)
		}
		if hasKey {
			pl(i18n.T("cmd.server.detail.key_file"), s.Auth.KeyFile)
		}
	} else {
		pl(i18n.T("cmd.server.detail.auth_type"), i18n.T("cmd.server.auth.none"))
	}

	if s.Remark != "" {
		pl(i18n.T("cmd.server.detail.remark"), s.Remark)
	}
	for k, v := range s.Options {
		pl(i18n.T("cmd.server.detail.option")+": "+k, v)
	}
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
		fmt.Println(i18n.T("cmd.server.ls.empty"))
		return nil
	}

	// 1) collect all rows
	type row struct {
		group, name, host, auth, remark string
		version                         int
	}
	var rows []row
	for g, servers := range result {
		for _, s := range servers {
			hostStr := fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)

			auth := i18n.T("cmd.server.auth.none")
			if s.Auth != nil {
				switch {
				case s.Auth.Password != "" && s.Auth.KeyFile != "":
					auth = i18n.T("cmd.server.auth.password_key")
				case s.Auth.Password != "":
					auth = i18n.T("cmd.server.auth.password")
				case s.Auth.KeyFile != "":
					auth = i18n.T("cmd.server.auth.key")
				}
			}

			rows = append(rows, row{
				group:   g,
				name:    s.Name,
				host:    hostStr,
				version: s.Version,
				auth:    auth,
				remark:  s.Remark,
			})
		}
	}

	// 2) calculate adaptive column widths
	maxGroup, maxName, maxHost := len(i18n.T("cmd.server.ls.header.group")), len(i18n.T("cmd.server.ls.header.name")), len(i18n.T("cmd.server.ls.header.host"))
	for _, r := range rows {
		if l := len(r.group); l > maxGroup {
			maxGroup = l
		}
		if l := len(r.name); l > maxName {
			maxName = l
		}
		if l := len(r.host); l > maxHost {
			maxHost = l
		}
	}

	// 3) print header + separator
	authWidth := 14
	headerFmt := fmt.Sprintf("%%%ds %%-%ds %%-%ds %%-4s %%-%ds %%s",
		maxGroup, maxName, maxHost, authWidth)
	fmt.Printf(headerFmt+"\n",
		i18n.T("cmd.server.ls.header.group"),
		i18n.T("cmd.server.ls.header.name"),
		i18n.T("cmd.server.ls.header.host"),
		i18n.T("cmd.server.ls.header.ver"),
		i18n.T("cmd.server.ls.header.auth"),
		i18n.T("cmd.server.ls.header.remark"))

	// fixed-width area: Group + 1sp + Name + 1sp + Host + 1sp + Ver(4) + 1sp + Auth + 1sp
	sepLen := maxGroup + maxName + maxHost + authWidth + 10
	if sepLen < 60 {
		sepLen = 60
	}
	fmt.Println(strings.Repeat("-", sepLen))

	// 4) print data rows (with color per group, bold alternation within group)
	var groupColors = []int{32, 34, 33, 35, 36}
	groupOrder := make([]string, 0, len(result))
	groupSeen := make(map[string]bool)
	groupRow := make(map[string]int) // group → row counter for bold alternation
	for _, r := range rows {
		if !groupSeen[r.group] {
			groupSeen[r.group] = true
			groupOrder = append(groupOrder, r.group)
		}
	}

	for _, r := range rows {
		groupIdx := 0
		for i, g := range groupOrder {
			if g == r.group {
				groupIdx = i
				break
			}
		}
		baseColor := groupColors[groupIdx%len(groupColors)]

		rowN := groupRow[r.group]
		var bold int
		if rowN%2 == 0 {
			bold = 1
		} else {
			bold = 0
		}
		groupRow[r.group] = rowN + 1

		fmt.Printf("\033[%d;%dm%*s %-*s %-*s %-4d %-*s %s\033[0m\n",
			bold, baseColor,
			maxGroup, r.group,
			maxName, r.name,
			maxHost, r.host,
			r.version, authWidth, r.auth, r.remark)
	}

	return nil
}

// serverInfoAction 处理 server info 命令，显示服务器的完整配置详情。
// 如果服务器不存在，自动搜索最接近的名称并给出建议。
func (a *App) serverInfoAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("%s", i18n.T("cmd.server.error.name_required"))
	}

	server, err := a.serverSvc.GetServer(name)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			if suggestion, ok := a.serverSvc.SuggestServer(name); ok {
				return fmt.Errorf("%s", i18n.T("cmd.server.info.not_found", map[string]interface{}{"Name": name, "Suggestion": suggestion}))
			}
		}
		return err
	}

	a.printServerDetail(server)
	return nil
}

// serverRemoveAction 处理 server rm 命令，删除指定服务器配置。
func (a *App) serverRemoveAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("%s", i18n.T("cmd.server.error.name_required"))
	}

	if err := a.serverSvc.RemoveServer(name); err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.server.rm.success", map[string]interface{}{"Name": name}))
	return nil
}

// serverMoveAction 处理 server mv 命令，重命名或跨分组移动服务器。
func (a *App) serverMoveAction(c *cli.Context) error {
	from := c.Args().Get(0)
	to := c.Args().Get(1)

	if from == "" || to == "" {
		return fmt.Errorf("%s", i18n.T("cmd.server.mv.error.names_required"))
	}

	if err := a.serverSvc.MoveServer(from, to); err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.server.mv.success", map[string]interface{}{"Name": from, "NewName": to}))
	return nil
}

// previewServer 预览服务器配置信息，用于 add/set 确认前的预览。
// 显示格式简洁，便于用户确认。
func (a *App) previewServer(s *domain.Server, name string) {
	group, serverName := domain.ParseName(name)
	fullName := domain.JoinName(group, serverName)

	fmt.Println(i18n.T("cmd.server.preview.title"))
	fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.name"), fullName)
	fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.host"), s.Host)
	fmt.Printf("%-9s %d\n", i18n.T("cmd.server.preview.port"), s.Port)
	fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.user"), s.User)

	if s.Auth != nil {
		hasPwd := s.Auth.Password != ""
		hasKey := s.Auth.KeyFile != ""
		switch {
		case hasPwd && hasKey:
			fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.auth"), i18n.T("cmd.server.auth.password_key"))
		case hasPwd:
			fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.auth"), i18n.T("cmd.server.auth.password"))
		case hasKey:
			fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.auth"), i18n.T("cmd.server.auth.key"))
		default:
			fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.auth"), i18n.T("cmd.server.auth.none"))
		}
		if hasKey {
			fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.key"), s.Auth.KeyFile)
		}
	} else {
		fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.auth"), i18n.T("cmd.server.auth.none"))
	}

	if s.Remark != "" {
		fmt.Printf("%-9s %s\n", i18n.T("cmd.server.preview.remark"), s.Remark)
	}
	if len(s.Options) > 0 {
		fmt.Printf("%-9s %v\n", i18n.T("cmd.server.preview.options"), s.Options)
	}
}

// askConfirmation 询问用户确认，返回 true 表示用户确认（Y/y/yes）。
func (a *App) askConfirmation(prompt string) bool {
	fmt.Print(prompt)
	fmt.Print(" [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "" || input == "y" || input == "yes"
}
