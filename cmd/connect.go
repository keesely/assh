package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"assh/asshc/domain"

	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh"
)

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

func (a *App) loginAction(c *cli.Context) error {
	target := c.Args().Get(0)
	host := c.String("host")
	port := c.Int("port")
	user := firstNonEmpty(c.String("user"), c.String("login"))
	password := c.String("password")
	keyFile := firstNonEmpty(c.String("identity-file"), c.String("key"))

	if target == "" && host == "" {
		return fmt.Errorf("no target specified: use <name>, <user@host>, or -H <host>")
	}
	if target != "" && host != "" {
		return fmt.Errorf("cannot specify both target and -H/--host")
	}

	var client *ssh.Client
	var err error

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
		client, err = a.connectSvc.ConnectDirect(host, port, user, password, keyFile)

	case host != "":
		if user == "" {
			user = "root"
		}
		if port <= 0 {
			port = 22
		}
		client, err = a.connectSvc.ConnectDirect(host, port, user, password, keyFile)

	default:
		client, err = a.connectSvc.ConnectByName(target)
	}

	if err != nil {
		return err
	}
	defer a.connectSvc.Close(client)

	return a.connectSvc.Shell(client)
}

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

type bcServerResult struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	User    string `json:"user"`
	Stdout  string `json:"stdout"`
	Stderr  string `json:"stderr"`
	Error   string `json:"error"`
}

type bcSummary struct {
	Command   string            `json:"command"`
	Timestamp string            `json:"timestamp"`
	Total     int               `json:"total"`
	Success   int               `json:"success"`
	Failed    int               `json:"failed"`
	Servers   []bcServerResult  `json:"servers"`
}

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

func writeLogFile(filePath, content string) {
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(content + "\n")
}
