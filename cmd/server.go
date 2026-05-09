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

func (a *App) registerServerCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:  "server",
		Usage: "Server management",
		Subcommands: []cli.Command{
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
		},
	})
}

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

// serverSetAction — upsert: create if not exists, update if exists.
// Validation ensures host, port range, etc.
func (a *App) serverSetAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("server name is required")
	}

	existing, err := a.serverSvc.GetServer(name)
	isNew := err != nil

	// Check if any modification flag was provided
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
		// --- Create path ---
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
		// --- Update path ---
		if !hasChanges {
			a.printServerDetail(existing)
			fmt.Println("\nhint: use flags to modify parameters, e.g. --host, -p, -P, --remark")
			return nil
		}

		// Deep copy existing
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

		// Apply changes only for explicitly-set flags
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

		// After SetServer, the store increments version; read it back
		after, err := a.serverSvc.GetServer(name)
		if err == nil {
			fmt.Printf("server %q updated (v%d)\n", name, after.Version)
		} else {
			fmt.Printf("server %q updated\n", name)
		}
	}

	return nil
}

// serverRollbackAction rolls back a server to a specific version.
func (a *App) serverRollbackAction(c *cli.Context) error {
	name := c.Args().Get(0)
	if name == "" {
		return fmt.Errorf("server name is required")
	}

	// --list mode: show changelog
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

	// Rollback to version
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

	fmt.Printf("%-12s %-14s %-16s %-6s %-8s %-8s %s\n", "Group", "Name", "Host", "Port", "User", "Ver", "Auth")
	fmt.Println(strings.Repeat("-", 80))

	for g, servers := range result {
		for _, s := range servers {
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
			fmt.Printf("%-12s %-14s %-16s %-6d %-8s %-8d %s\n", g, s.Name, s.Host, s.Port, s.User, s.Version, auth)
		}
	}

	return nil
}

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
