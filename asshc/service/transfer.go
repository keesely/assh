package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"assh/asshc/domain"
	"assh/asshc/port"
	"assh/asshc/infra/sftp"
	"assh/internal/i18n"
	gsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type TransferOptions struct {
	Recursive      bool
	Resume         bool
	Overwrite      string
	Concurrency    int
	Progress       bool
	VerifyChecksum bool
}

type TransferService struct {
	transfer   *sftp.SFTPTransfer
	repo       port.ServerRepository
	connectSvc *ConnectService // optional, for -J jump chain support
}

func NewTransferService(transfer *sftp.SFTPTransfer, repo port.ServerRepository) *TransferService {
	return &TransferService{
		transfer: transfer,
		repo:     repo,
	}
}

// SetConnectService 设置 ConnectService 引用，用于 -J 跳板链支持。
func (s *TransferService) SetConnectService(cs *ConnectService) {
	s.connectSvc = cs
}

// connectWithChain 通过跳板链建立 SSH 连接。
// 如果 chainExpr 为空，返回 nil 表示直接连接。
func (s *TransferService) connectWithChain(name, chainExpr string) (*ssh.Client, error) {
	if chainExpr == "" {
		return nil, nil
	}
	if s.connectSvc == nil {
		return nil, fmt.Errorf("%s", i18n.T("service.transfer.error.jump_chain_not_available"))
	}
	client, _, err := s.connectSvc.ConnectChain(name, chainExpr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("service.transfer.error.jump_connection_failed"), err)
	}
	return client, nil
}

func (s *TransferService) PushFile(ctx context.Context, name, localPath, remotePath string, opts TransferOptions) error {
	server, err := s.getServer(name)
	if err != nil {
		return err
	}

	localPath, err = filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.invalid_local_path", map[string]interface{}{"Path": localPath}), err)
	}

	if !s.localExists(localPath) {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.file_not_exist", map[string]interface{}{"Path": localPath}))
	}

	isDir := s.localIsDir(localPath)
	if isDir && !opts.Recursive {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.directory_omitted"))
	}

	remotePath = s.ensureRemoteSlash(remotePath)
	if isDir {
		baseName := filepath.Base(localPath)
		if !strings.HasSuffix(remotePath, "/") {
			remotePath += "/"
		}
		remotePath += baseName
	} else if strings.HasSuffix(remotePath, "/") {
		// Push file to a directory (remote path ends with /)
		remotePath += filepath.Base(localPath)
	}

	if !s.transfer.IsDir(server, s.remoteDir(remotePath)) {
		fmt.Printf("%s", i18n.T("service.transfer.error.remote_dir_not_exist", map[string]interface{}{"Path": s.remoteDir(remotePath)}))
		var answer string
		fmt.Scanln(&answer)
		if answer == "y" || answer == "Y" || answer == "yes" {
			if err := s.transfer.Mkdir(server, s.remoteDir(remotePath)); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.create_dir_failed"), err)
			}
		} else {
			return fmt.Errorf("%s", i18n.T("service.transfer.error.aborted"))
		}
	}

	var progress port.TransferProgress
	if opts.Progress {
		progress = func(info port.TransferInfo) {
			rate := info.Rate
			if rate == "" {
				rate = "0B/s"
			}
			eta := info.ETA
			if eta == "" {
				eta = "ETA ?"
			}
			percent := int(info.Progress * 100)
			fmt.Printf("\r[%s] %3d%% %s %s", info.FileName, percent, rate, eta)
			if info.Progress >= 1 {
				fmt.Println()
			}
		}
	}

	if err := s.transfer.Push(ctx, server, localPath, remotePath, s.toPortOpts(opts), progress); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.push_failed"), err)
	}

	if opts.VerifyChecksum {
		fmt.Printf("\n%s\n", i18n.T("service.transfer.verify.verifying", map[string]interface{}{"Path": filepath.Base(localPath)}))
		if err := s.transfer.VerifyUpload(server, localPath, remotePath, true); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.verification_failed"), err)
		}
		fmt.Println(i18n.T("service.transfer.verify.size_match"))
	}

	return nil
}

// PushFileDirect pushes a file using a pre-constructed server (for direct connections).
// This bypasses the server name lookup and uses the provided server config directly.
func (s *TransferService) PushFileDirect(ctx context.Context, server *domain.Server, localPath, remotePath string, opts TransferOptions) error {
	localPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.invalid_local_path", map[string]interface{}{"Path": localPath}), err)
	}

	if !s.localExists(localPath) {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.file_not_exist", map[string]interface{}{"Path": localPath}))
	}

	isDir := s.localIsDir(localPath)
	if isDir && !opts.Recursive {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.directory_omitted"))
	}

	remotePath = s.ensureRemoteSlash(remotePath)
	if isDir {
		baseName := filepath.Base(localPath)
		if !strings.HasSuffix(remotePath, "/") {
			remotePath += "/"
		}
		remotePath += baseName
	} else if strings.HasSuffix(remotePath, "/") {
		// Push file to a directory (remote path ends with /)
		remotePath += filepath.Base(localPath)
	}

	if !s.transfer.IsDir(server, s.remoteDir(remotePath)) {
		fmt.Printf("%s", i18n.T("service.transfer.error.remote_dir_not_exist", map[string]interface{}{"Path": s.remoteDir(remotePath)}))
		var answer string
		fmt.Scanln(&answer)
		if answer == "y" || answer == "Y" || answer == "yes" {
			if err := s.transfer.Mkdir(server, s.remoteDir(remotePath)); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.create_dir_failed"), err)
			}
		} else {
			return fmt.Errorf("%s", i18n.T("service.transfer.error.aborted"))
		}
	}

	var progress port.TransferProgress
	if opts.Progress {
		progress = func(info port.TransferInfo) {
			rate := info.Rate
			if rate == "" {
				rate = "0B/s"
			}
			eta := info.ETA
			if eta == "" {
				eta = "ETA ?"
			}
			percent := int(info.Progress * 100)
			fmt.Printf("\r[%s] %3d%% %s %s", info.FileName, percent, rate, eta)
			if info.Progress >= 1 {
				fmt.Println()
			}
		}
	}

	if err := s.transfer.Push(ctx, server, localPath, remotePath, s.toPortOpts(opts), progress); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.push_failed"), err)
	}

	if opts.VerifyChecksum {
		fmt.Printf("\n%s\n", i18n.T("service.transfer.verify.verifying", map[string]interface{}{"Path": filepath.Base(localPath)}))
		if err := s.transfer.VerifyUpload(server, localPath, remotePath, true); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.verification_failed"), err)
		}
		fmt.Println(i18n.T("service.transfer.verify.size_match"))
	}

	return nil
}

func (s *TransferService) PullFile(ctx context.Context, name, remotePath, localPath string, opts TransferOptions) error {
	server, err := s.getServer(name)
	if err != nil {
		return err
	}

	remotePath = s.normalizeRemotePath(remotePath)

	if !s.transfer.Exists(server, remotePath) {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.remote_path_not_exist", map[string]interface{}{"Path": remotePath}))
	}

	remoteIsDir := s.transfer.IsDir(server, remotePath)
	if remoteIsDir && !opts.Recursive {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.omit_download_dir"))
	}

	localPath, err = filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.invalid_local_path", map[string]interface{}{"Path": localPath}), err)
	}

	if remoteIsDir {
		baseName := s.remoteBaseName(remotePath)
		localPath = filepath.Join(localPath, baseName)
	} else if s.localExists(localPath) && s.localIsDir(localPath) {
		// Local target is an existing directory and remote is a file: append remote filename
		localPath = filepath.Join(localPath, filepath.Base(remotePath))
	}

	if !s.localExists(filepath.Dir(localPath)) {
		fmt.Printf("%s", i18n.T("service.transfer.error.local_dir_not_exist", map[string]interface{}{"Path": filepath.Dir(localPath)}))
		var answer string
		fmt.Scanln(&answer)
		if answer == "y" || answer == "Y" || answer == "yes" {
			if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.create_local_dir_failed"), err)
			}
		} else {
			return fmt.Errorf("%s", i18n.T("service.transfer.error.aborted"))
		}
	}

	if s.transfer.IsDir(server, remotePath) {
		baseName := s.remoteBaseName(remotePath)
		localPath = filepath.Join(localPath, baseName)
	}

	var progress port.TransferProgress
	if opts.Progress {
		progress = func(info port.TransferInfo) {
			rate := info.Rate
			if rate == "" {
				rate = "0B/s"
			}
			eta := info.ETA
			if eta == "" {
				eta = "ETA ?"
			}
			percent := int(info.Progress * 100)
			fmt.Printf("\r[%s] %3d%% %s %s", info.FileName, percent, rate, eta)
			if info.Progress >= 1 {
				fmt.Println()
			}
		}
	}

	if err := s.transfer.Pull(ctx, server, remotePath, localPath, s.toPortOpts(opts), progress); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.pull_failed"), err)
	}

	fmt.Println()
	return nil
}

// PullFileDirect pulls a file using a pre-constructed server (for direct connections).
func (s *TransferService) PullFileDirect(ctx context.Context, server *domain.Server, remotePath, localPath string, opts TransferOptions) error {
	remotePath = s.normalizeRemotePath(remotePath)

	if localPath == "" {
		localPath = "."
	}

	localPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.invalid_local_path", map[string]interface{}{"Path": localPath}), err)
	}

	remoteIsDir := s.transfer.IsDir(server, remotePath)
	if remoteIsDir && !opts.Recursive {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.omit_download_dir"))
	}

	if remoteIsDir {
		baseName := filepath.Base(remotePath)
		localPath = filepath.Join(localPath, baseName)
	} else if s.localExists(localPath) && s.localIsDir(localPath) {
		// Local target is an existing directory and remote is a file: append remote filename
		localPath = filepath.Join(localPath, filepath.Base(remotePath))
	}

	localDir := filepath.Dir(localPath)
	if !s.localExists(localDir) {
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.create_local_dir_failed"), err)
		}
	}

	var progress port.TransferProgress
	if opts.Progress {
		progress = func(info port.TransferInfo) {
			rate := info.Rate
			if rate == "" {
				rate = "0B/s"
			}
			eta := info.ETA
			if eta == "" {
				eta = "ETA ?"
			}
			percent := int(info.Progress * 100)
			fmt.Printf("\r[%s] %3d%% %s %s", info.FileName, percent, rate, eta)
			if info.Progress >= 1 {
				fmt.Println()
			}
		}
	}

	if err := s.transfer.Pull(ctx, server, remotePath, localPath, s.toPortOpts(opts), progress); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.pull_failed"), err)
	}

	fmt.Println()
	return nil
}

// makeProgressCallback constructs a generic progress callback function.
func (s *TransferService) makeProgressCallback(opts TransferOptions) port.TransferProgress {
	if !opts.Progress {
		return nil
	}
	return func(info port.TransferInfo) {
		rate := info.Rate
		if rate == "" {
			rate = "0B/s"
		}
		eta := info.ETA
		if eta == "" {
			eta = "ETA ?"
		}
		percent := int(info.Progress * 100)
		fmt.Printf("\r[%s] %3d%% %s %s", info.FileName, percent, rate, eta)
		if info.Progress >= 1 {
			fmt.Println()
		}
	}
}

// PushFileWithChain uploads a file through a jump chain.
// chainExpr is the -J argument value, format same as assh -J: comma-separated jump list.
func (s *TransferService) PushFileWithChain(ctx context.Context, name, chainExpr, localPath, remotePath string, opts TransferOptions) error {
	// 1. Validate local path
	localPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.invalid_local_path", map[string]interface{}{"Path": localPath}), err)
	}
	if !s.localExists(localPath) {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.file_not_exist", map[string]interface{}{"Path": localPath}))
	}
	isDir := s.localIsDir(localPath)
	if isDir && !opts.Recursive {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.directory_omitted"))
	}

	// 2. Establish jump connection
	sshClient, err := s.connectWithChain(name, chainExpr)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	// 3. Path normalization
	remotePath = s.ensureRemoteSlash(remotePath)
	if isDir {
		baseName := filepath.Base(localPath)
		if !strings.HasSuffix(remotePath, "/") {
			remotePath += "/"
		}
		remotePath += baseName
	} else if strings.HasSuffix(remotePath, "/") {
		remotePath += filepath.Base(localPath)
	}

	// 4. Check remote directory
	gsftpClient, err := gsftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.sftp_client_creation_failed"), err)
	}

	if !isDirExist(gsftpClient, s.remoteDir(remotePath)) {
		fmt.Printf("%s", i18n.T("service.transfer.error.remote_dir_not_exist", map[string]interface{}{"Path": s.remoteDir(remotePath)}))
		var answer string
		fmt.Scanln(&answer)
		if answer == "y" || answer == "Y" || answer == "yes" {
			if err := gsftpClient.MkdirAll(s.remoteDir(remotePath)); err != nil {
				gsftpClient.Close()
				return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.create_dir_failed"), err)
			}
		} else {
			gsftpClient.Close()
			return fmt.Errorf("%s", i18n.T("service.transfer.error.aborted"))
		}
	}
	gsftpClient.Close()

	// 5. Transfer
	progress := s.makeProgressCallback(opts)
	if err := s.transfer.PushWithSSHClient(ctx, sshClient, localPath, remotePath, s.toPortOpts(opts), progress); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.push_failed"), err)
	}

	if opts.VerifyChecksum {
		fmt.Printf("\n%s\n", i18n.T("service.transfer.verify.verifying", map[string]interface{}{"Path": filepath.Base(localPath)}))
		if err := s.transfer.VerifyUploadWithSSHClient(sshClient, localPath, remotePath, true); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.verification_failed"), err)
		}
		fmt.Println(i18n.T("service.transfer.verify.size_match"))
	}

	return nil
}

// PullFileWithChain downloads a file through a jump chain.
// chainExpr is the -J argument value, format same as assh -J.
func (s *TransferService) PullFileWithChain(ctx context.Context, name, chainExpr, remotePath, localPath string, opts TransferOptions) error {
	// 1. Establish jump connection
	sshClient, err := s.connectWithChain(name, chainExpr)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	// 2. Create SFTP client for checks
	gsftpClient, err := gsftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.sftp_client_creation_failed"), err)
	}

	// 3. Validate remote path
	remotePath = s.normalizeRemotePath(remotePath)
	if !exists(gsftpClient, remotePath) {
		gsftpClient.Close()
		return fmt.Errorf("%s", i18n.T("service.transfer.error.remote_path_not_exist", map[string]interface{}{"Path": remotePath}))
	}
	remoteIsDir := isDir(gsftpClient, remotePath)
	if remoteIsDir && !opts.Recursive {
		gsftpClient.Close()
		return fmt.Errorf("%s", i18n.T("service.transfer.error.omit_download_dir"))
	}

	// 4. Handle local path
	localPath, err = filepath.Abs(localPath)
	if err != nil {
		gsftpClient.Close()
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.invalid_local_path", map[string]interface{}{"Path": localPath}), err)
	}
	if remoteIsDir {
		baseName := s.remoteBaseName(remotePath)
		localPath = filepath.Join(localPath, baseName)
	} else if s.localExists(localPath) && s.localIsDir(localPath) {
		localPath = filepath.Join(localPath, filepath.Base(remotePath))
	}
	if !s.localExists(filepath.Dir(localPath)) {
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			gsftpClient.Close()
			return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.create_local_dir_failed"), err)
		}
	}
	gsftpClient.Close()

	// 5. Transfer
	progress := s.makeProgressCallback(opts)
	if err := s.transfer.PullWithSSHClient(ctx, sshClient, remotePath, localPath, s.toPortOpts(opts), progress); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.pull_failed"), err)
	}

	fmt.Println()
	return nil
}

// isDirExist 检查远程路径是否存在且为目录（使用已有 SFTP 客户端）。
func isDirExist(client *gsftp.Client, path string) bool {
	info, err := client.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// exists 检查远程路径是否存在（使用已有 SFTP 客户端）。
func exists(client *gsftp.Client, path string) bool {
	_, err := client.Stat(path)
	return err == nil
}

// isDir 检查远程路径是否为目录（使用已有 SFTP 客户端）。
func isDir(client *gsftp.Client, path string) bool {
	info, err := client.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (s *TransferService) ListRemote(name, remotePath string) ([]port.FileInfo, error) {
	server, err := s.getServer(name)
	if err != nil {
		return nil, err
	}

	if remotePath == "" {
		remotePath = "."
	}

	files, err := s.transfer.List(server, remotePath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("service.transfer.error.list_failed"), err)
	}

	return files, nil
}

// ListRemoteDirect lists a remote directory using a pre-constructed server config (for direct connections).
func (s *TransferService) ListRemoteDirect(server *domain.Server, remotePath string) ([]port.FileInfo, error) {
	if remotePath == "" {
		remotePath = "."
	}

	files, err := s.transfer.List(server, remotePath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("service.transfer.error.list_failed"), err)
	}

	return files, nil
}

func (s *TransferService) RemoveRemote(name, remotePath string) error {
	server, err := s.getServer(name)
	if err != nil {
		return err
	}

	if err := s.transfer.Remove(server, remotePath); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.remove_failed"), err)
	}

	return nil
}

func (s *TransferService) MkdirRemote(name, remotePath string) error {
	server, err := s.getServer(name)
	if err != nil {
		return err
	}

	if err := s.transfer.Mkdir(server, remotePath); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.mkdir_failed"), err)
	}

	return nil
}

func (s *TransferService) getServer(name string) (*domain.Server, error) {
	if strings.Contains(name, "@") || strings.Contains(name, "-H") {
		return s.parseDirectServer(name)
	}

	server, err := s.repo.Get(name)
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.T("service.transfer.error.server_not_found", map[string]interface{}{"Name": name}))
	}
	return server, nil
}

func (s *TransferService) parseDirectServer(name string) (*domain.Server, error) {
	host := ""
	user := "root"
	var port int = 22
	password := ""
	keyFile := ""

	parts := strings.Split(name, "@")
	if len(parts) == 2 {
		user = parts[0]
		host = parts[1]
	}

	if host == "" {
		return nil, fmt.Errorf("%s", i18n.T("service.transfer.error.host_required"))
	}

	return &domain.Server{
		Name: name,
		Host: host,
		Port: port,
		User: user,
		Auth: &domain.Auth{
			Password: password,
			KeyFile:  keyFile,
		},
	}, nil
}

func (s *TransferService) normalizeRemotePath(path string) string {
	normalized := strings.ReplaceAll(path, "\\", "/")
	normalized = strings.ReplaceAll(normalized, "//", "/")
	return normalized
}

func (s *TransferService) remoteDir(path string) string {
	normalized := s.normalizeRemotePath(path)
	if normalized == "/" {
		return "/"
	}
	parts := strings.Split(strings.TrimSuffix(normalized, "/"), "/")
	if len(parts) <= 1 {
		return "/"
	}
	result := strings.Join(parts[:len(parts)-1], "/")
	if !strings.HasPrefix(result, "/") {
		result = "/" + result
	}
	return result
}

func (s *TransferService) remoteBaseName(path string) string {
	normalized := s.normalizeRemotePath(path)
	parts := strings.Split(normalized, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

func (s *TransferService) ensureRemoteSlash(path string) string {
	if !strings.HasSuffix(path, "/") && !strings.Contains(filepath.Ext(path), ".") {
		return path + "/"
	}
	return path
}

// toPortOpts converts service TransferOptions to port-level TransferOptions.
func (s *TransferService) toPortOpts(opts TransferOptions) port.TransferOptions {
	return port.TransferOptions{
		Resume:         opts.Resume,
		Overwrite:      opts.Overwrite,
		VerifyChecksum: opts.VerifyChecksum,
		Concurrency:    opts.Concurrency,
	}
}

func (s *TransferService) localExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (s *TransferService) localIsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (s *TransferService) GlobLocal(pattern string) ([]string, error) {
	return s.transfer.GlobLocal(pattern)
}

func (s *TransferService) PushGlob(ctx context.Context, name, pattern, remotePath string, opts TransferOptions) error {
	files, err := s.GlobLocal(pattern)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("service.transfer.error.glob_failed"), err)
	}

	if len(files) == 0 {
		return fmt.Errorf("%s", i18n.T("service.transfer.error.no_matching_files", map[string]interface{}{"Pattern": pattern}))
	}

	for i, file := range files {
		fmt.Printf("%s\n", i18n.T("service.transfer.progress.file_counter", map[string]interface{}{"Current": i + 1, "Total": len(files), "Name": filepath.Base(file)}))
		if err := s.PushFile(ctx, name, file, remotePath, opts); err != nil {
			fmt.Printf("%s\n", i18n.T("service.transfer.progress.failed", map[string]interface{}{"Error": err}))
		}
	}

	return nil
}

func (s *TransferService) StatRemote(name, remotePath string) (*port.FileInfo, error) {
	return s.transfer.StatRemote(nil, remotePath)
}

func (s *TransferService) Exists(name, remotePath string) bool {
	server, err := s.getServer(name)
	if err != nil {
		return false
	}
	return s.transfer.Exists(server, remotePath)
}

func (s *TransferService) IsDir(name, remotePath string) bool {
	server, err := s.getServer(name)
	if err != nil {
		return false
	}
	return s.transfer.IsDir(server, remotePath)
}