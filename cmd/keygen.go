package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"assh/asshc/domain"
	"assh/asshc/service"
	"assh/config"
	"assh/internal/i18n"
	"github.com/urfave/cli"
)

// registerKeygenCommands 注册 keygen 命令到 CLI 应用。
func (a *App) registerKeygenCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "keygen",
		Usage:     "Generate SSH key pair",
		ArgsUsage: "[<server>]",
		Description: i18n.T("cmd.keygen.description"),
		Action: a.keygenAction,
		Flags: []cli.Flag{
			cli.StringFlag{Name: "type, t", Value: "rsa", Usage: i18n.T("cmd.keygen.flag.type")},
			cli.IntFlag{Name: "bits, b", Value: 4096, Usage: i18n.T("cmd.keygen.flag.bits")},
			cli.StringFlag{Name: "output, O, f", Usage: i18n.T("cmd.keygen.flag.output")},
			cli.StringFlag{Name: "comment, C", Value: "", Usage: i18n.T("cmd.keygen.flag.comment")},
			cli.StringFlag{Name: "new-passphrase, N", Value: "", Usage: i18n.T("cmd.keygen.flag.passphrase")},
			// Direct connection parameters
			cli.StringFlag{Name: "H, host", Usage: i18n.T("cmd.keygen.flag.host")},
			cli.StringFlag{Name: "u, user", Usage: i18n.T("cmd.keygen.flag.user")},
			cli.StringFlag{Name: "l, login", Usage: i18n.T("cmd.keygen.flag.login")},
			cli.IntFlag{Name: "p, port", Value: 22, Usage: i18n.T("cmd.keygen.flag.port")},
			cli.StringFlag{Name: "P, password", Usage: i18n.T("cmd.keygen.flag.password")},
		},
	})
}

// keygenAction 实现 keygen 命令的处理逻辑.
//
// 三种运行模式（优先级从高到低）：
//
//  1. 独立生成模式（-f/--output 指定输出路径）：
//     assh keygen -f ~/.ssh/mykey
//     流程: GenerateToPath → 私钥写入指定路径 + 公钥写入 {path}.pub
//
//  2. 已命名服务器模式（通过服务器名查找配置）：
//     assh keygen myserver
//     流程：GenerateAndDeploy → 生成密钥 → 部署 → 更新服务器配置
//
//  3. 直连模式（通过 -H/--host 参数直连，无需服务器配置）：
//     assh keygen -H 192.168.1.1 -u root -P password
//     流程：生成密钥 → 尝试部署公钥 → 记录到 known_servers
func (a *App) keygenAction(c *cli.Context) error {
	keyType := c.String("type")
	bits := c.Int("bits")
	comment := c.String("comment")
	outputPath := c.String("output")

	// 解析 passphrase（优先级：-N > account password > nil）
	passphrase := resolvePassphrase(c.String("new-passphrase"), a.keymgr.GetAccountPassphrase())

	// --- 独立生成模式（优先级最高）---
	if outputPath != "" {
		return a.keygenOutput(keyType, bits, passphrase, comment, outputPath)
	}

	host := c.String("host")

	// --- 直连模式 ---
	if host != "" {
		user := firstNonEmpty(c.String("user"), c.String("login"))
		port := c.Int("port")
		password := c.String("password")
		return a.keygenDirect(host, port, user, password, keyType, bits, passphrase, comment)
	}

	// --- 已命名服务器模式 ---
	if c.NArg() < 1 {
		// 无参时触发交互式向导
		return a.keygenInteractive(keyType, bits, passphrase)
	}
	name := c.Args()[0]

	// 调用 KeyService 生成密钥并部署
	if err := a.keySvc.GenerateAndDeploy(name, keyType, bits, comment); err != nil {
		return fmt.Errorf("keygen failed: %w", err)
	}

	// 获取更新后的服务器配置以显示密钥路径
	server, err := a.serverSvc.GetServer(name)
	if err != nil {
		return nil // keygen 成功，只是无法获取详情
	}

	keyPath := ""
	if server.Auth != nil {
		keyPath = server.Auth.KeyFile
	}

	fmt.Print("✓ " + i18n.T("cmd.keygen.success") + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.server", map[string]interface{}{"Server": name}) + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.type", map[string]interface{}{"Type": keyType, "Bits": bits}) + "\n")
	if comment != "" {
		fmt.Print(i18n.T("cmd.keygen.info.comment", map[string]interface{}{"Comment": comment}) + "\n")
	}
	fmt.Print(i18n.T("cmd.keygen.info.key_file", map[string]interface{}{"Path": keyPath}) + "\n")
	if server.Auth != nil && server.Auth.Password != "" {
		fmt.Print(i18n.T("cmd.keygen.info.auth_password_key") + "\n")
	} else {
		fmt.Print(i18n.T("cmd.keygen.info.auth_key") + "\n")
	}

	return nil
}

// resolvePassphrase resolves passphrase priority.
// CLI-passphrase takes precedence over account-passphrase.
func resolvePassphrase(cliPassphrase string, accountPassphrase []byte) []byte {
	if cliPassphrase != "" {
		return []byte(cliPassphrase)
	}
	return accountPassphrase
}

// keygenOutput handles standalone generation mode (-f).
// Generates SSH key pair to specified path, no server involved.
func (a *App) keygenOutput(keyType string, bits int, passphrase []byte, comment string, outputPath string) error {
	pubKey, fingerprint, err := a.keymgr.GenerateToPath(keyType, bits, passphrase, comment, outputPath)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	// Resolve actual write path (same logic as GenerateToPath internal path resolution)
	privPath, pubPath := resolveOutputFiles(outputPath, keyType)

	fmt.Print("✓ " + i18n.T("cmd.keygen.success") + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.type", map[string]interface{}{"Type": keyType, "Bits": bits}) + "\n")
	if comment != "" {
		fmt.Print(i18n.T("cmd.keygen.info.comment", map[string]interface{}{"Comment": comment}) + "\n")
	}
	fmt.Print(i18n.T("cmd.keygen.info.private_key", map[string]interface{}{"Path": privPath}) + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.public_key", map[string]interface{}{"Path": pubPath}) + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.fingerprint", map[string]interface{}{"Fingerprint": fingerprint}) + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.public_key_contents") + "\n")
	fmt.Printf("  %s\n", string(pubKey))

	return nil
}

// resolveOutputFiles resolves private/public key paths from outputPath and keyType.
// Uses config.ResolveKeyOutputPath and config.DefaultKeyName for consistency.
func resolveOutputFiles(outputPath, keyType string) (privPath, pubPath string) {
	expanded, err := config.ResolveKeyOutputPath(outputPath, keyType)
	if err != nil {
		return outputPath, outputPath + ".pub"
	}

	pubPath = expanded + ".pub"
	return expanded, pubPath
}

// keygenDirect handles direct connection mode key generation.
// No server config needed; key saved to data/keys/, tries deploy, records known_servers.
func (a *App) keygenDirect(host string, port int, user, password, keyType string, bits int, passphrase []byte, comment string) error {
	// 1. Generate key pair
	privPath, pubKey, fingerprint, err := a.keymgr.Generate(keyType, bits, passphrase, comment)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	// 2. Build temp server object for deployment
	server := &domain.Server{
		Host: host,
		Port: port,
		User: user,
		Auth: &domain.Auth{
			Password: password,
		},
	}

	// 3. Try deploy (P6.2-b: DeployService)
	deploySvc := service.NewDeployService(a.connectSvc.Connector())
	deployed, deployErr := deploySvc.DeployToServer(server, pubKey)
	if deployErr != nil {
		fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cmd.keygen.warning.deploy_failed", map[string]interface{}{"Error": deployErr}))
	} else if deployed {
		fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cmd.keygen.info.deployed", map[string]interface{}{"User": user, "Host": host, "Port": port}))
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cmd.keygen.info.already_exists", map[string]interface{}{"User": user, "Host": host, "Port": port}))
	}

	// 4. Record to known_servers
	authFingerprint := domain.ComputeAuthFingerprint(password, "")
	id := domain.ComputeKnownServerID(user, host, port, authFingerprint)
	ks := &domain.KnownServer{
		ID:              id,
		Host:            host,
		Port:            port,
		User:            user,
		AuthFingerprint: authFingerprint,
		KeyBackupPath:   privPath,
	}

	if a.knownRecorder != nil {
		if err := a.knownRecorder.RecordDirectConnect(ks); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cmd.keygen.warning.record_failed", map[string]interface{}{"Error": err}))
		}
		if err := a.knownRecorder.UpdateKeyBackupPath(id, privPath); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cmd.keygen.warning.backup_failed", map[string]interface{}{"Error": err}))
		}
	}

	fmt.Print("✓ " + i18n.T("cmd.keygen.success_direct") + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.host", map[string]interface{}{"Host": host, "Port": port}) + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.user", map[string]interface{}{"User": user}) + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.type", map[string]interface{}{"Type": keyType, "Bits": bits}) + "\n")
	if comment != "" {
		fmt.Print(i18n.T("cmd.keygen.info.comment", map[string]interface{}{"Comment": comment}) + "\n")
	}
	fmt.Print(i18n.T("cmd.keygen.info.key_file", map[string]interface{}{"Path": privPath}) + "\n")
	fmt.Print(i18n.T("cmd.keygen.info.fingerprint", map[string]interface{}{"Fingerprint": fingerprint}) + "\n")

	return nil
}

// keygenInteractive implements the no-argument interactive key generation wizard,
// mimicking ssh-keygen's interactive experience.
func (a *App) keygenInteractive(keyType string, bits int, accountPassphrase []byte) error {
	fmt.Println()
	fmt.Println(i18n.T("cmd.keygen.interactive.generating"))

	reader := bufio.NewReader(os.Stdin)

	// 1. Ask key type
	fmt.Print(i18n.T("cmd.keygen.interactive.type_prompt", map[string]interface{}{"Default": keyType}))
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read key type: %w", err)
	}
	input = strings.TrimSpace(input)
	if input != "" {
		keyType = input
	}

	// 2. Ask bits (for RSA/ECDSA)
	interactiveBits := bits
	if keyType == "rsa" || keyType == "ecdsa" {
		fmt.Print(i18n.T("cmd.keygen.interactive.bits_prompt", map[string]interface{}{"Default": bits}))
		input, err = reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read key bits: %w", err)
		}
		input = strings.TrimSpace(input)
		if input != "" {
			var newBits int
			if _, parseErr := fmt.Sscanf(input, "%d", &newBits); parseErr == nil {
				interactiveBits = newBits
			}
		}
	}

	// 3. Ask save path
	defaultPath := getDefaultKeyPath(keyType)
	fmt.Print(i18n.T("cmd.keygen.interactive.file_prompt", map[string]interface{}{"Default": defaultPath}))
	input, err = reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read key path: %w", err)
	}
	outputPath := strings.TrimSpace(input)
	if outputPath == "" {
		outputPath = defaultPath
	}

	// 4. Ask passphrase
	fmt.Print(i18n.T("cmd.keygen.interactive.passphrase_prompt"))
	input, err = reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read passphrase: %w", err)
	}
	passStr := strings.TrimSpace(input)
	var passphraseArg []byte
	if passStr != "" {
		// 5. Confirm passphrase
		fmt.Print(i18n.T("cmd.keygen.interactive.passphrase_confirm"))
		input, err = reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read passphrase confirm: %w", err)
		}
		passConfirm := strings.TrimSpace(input)
		if passStr != passConfirm {
			return fmt.Errorf("%s", i18n.T("cmd.keygen.error.passphrase_mismatch"))
		}
		passphraseArg = []byte(passStr)
	} else {
		passphraseArg = accountPassphrase
	}

	// 6. Generate key
	if err := a.keygenOutput(keyType, interactiveBits, passphraseArg, "", outputPath); err != nil {
		return fmt.Errorf("generate key: %w", err)
	}
	return nil
}

// getDefaultKeyPath returns the default key save path for a given key type.
func getDefaultKeyPath(keyType string) string {
	home, _ := os.UserHomeDir()
	return home + "/.ssh/" + config.DefaultKeyName(keyType)
}
