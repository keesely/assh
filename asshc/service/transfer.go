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
		return nil, fmt.Errorf("jump chain support not available")
	}
	client, _, err := s.connectSvc.ConnectChain(name, chainExpr)
	if err != nil {
		return nil, fmt.Errorf("jump connection failed: %w", err)
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
		return fmt.Errorf("invalid local path: %w", err)
	}

	if !s.localExists(localPath) {
		return fmt.Errorf("local file does not exist: %s", localPath)
	}

	isDir := s.localIsDir(localPath)
	if isDir && !opts.Recursive {
		return fmt.Errorf("omitting directory, use -r to upload directory")
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
		fmt.Printf("remote directory %s does not exist, create? ", s.remoteDir(remotePath))
		var answer string
		fmt.Scanln(&answer)
		if answer == "y" || answer == "Y" || answer == "yes" {
			if err := s.transfer.Mkdir(server, s.remoteDir(remotePath)); err != nil {
				return fmt.Errorf("create remote directory failed: %w", err)
			}
		} else {
			return fmt.Errorf("aborted")
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
		return fmt.Errorf("push failed: %w", err)
	}

	if opts.VerifyChecksum {
		fmt.Printf("\nverifying: %s\n", filepath.Base(localPath))
		if err := s.transfer.VerifyUpload(server, localPath, remotePath, true); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}
		fmt.Printf("  size match\n")
	}

	return nil
}

// PushFileDirect pushes a file using a pre-constructed server (for direct connections).
// This bypasses the server name lookup and uses the provided server config directly.
func (s *TransferService) PushFileDirect(ctx context.Context, server *domain.Server, localPath, remotePath string, opts TransferOptions) error {
	localPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}

	if !s.localExists(localPath) {
		return fmt.Errorf("local file does not exist: %s", localPath)
	}

	isDir := s.localIsDir(localPath)
	if isDir && !opts.Recursive {
		return fmt.Errorf("omitting directory, use -r to upload directory")
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
		fmt.Printf("remote directory %s does not exist, create? ", s.remoteDir(remotePath))
		var answer string
		fmt.Scanln(&answer)
		if answer == "y" || answer == "Y" || answer == "yes" {
			if err := s.transfer.Mkdir(server, s.remoteDir(remotePath)); err != nil {
				return fmt.Errorf("create remote directory failed: %w", err)
			}
		} else {
			return fmt.Errorf("aborted")
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
		return fmt.Errorf("push failed: %w", err)
	}

	if opts.VerifyChecksum {
		fmt.Printf("\nverifying: %s\n", filepath.Base(localPath))
		if err := s.transfer.VerifyUpload(server, localPath, remotePath, true); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}
		fmt.Printf("  size match\n")
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
		return fmt.Errorf("remote path does not exist: %s", remotePath)
	}

	remoteIsDir := s.transfer.IsDir(server, remotePath)
	if remoteIsDir && !opts.Recursive {
		return fmt.Errorf("omitting directory, use -r to download directory")
	}

	localPath, err = filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}

	if remoteIsDir {
		baseName := s.remoteBaseName(remotePath)
		localPath = filepath.Join(localPath, baseName)
	} else if s.localExists(localPath) && s.localIsDir(localPath) {
		// Local target is an existing directory and remote is a file: append remote filename
		localPath = filepath.Join(localPath, filepath.Base(remotePath))
	}

	if !s.localExists(filepath.Dir(localPath)) {
		fmt.Printf("local directory %s does not exist, create? ", filepath.Dir(localPath))
		var answer string
		fmt.Scanln(&answer)
		if answer == "y" || answer == "Y" || answer == "yes" {
			if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
				return fmt.Errorf("create local directory failed: %w", err)
			}
		} else {
			return fmt.Errorf("aborted")
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
		return fmt.Errorf("pull failed: %w", err)
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
		return fmt.Errorf("invalid local path: %w", err)
	}

	remoteIsDir := s.transfer.IsDir(server, remotePath)
	if remoteIsDir && !opts.Recursive {
		return fmt.Errorf("omitting directory, use -r to download directory")
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
			return fmt.Errorf("create local directory failed: %w", err)
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
		return fmt.Errorf("pull failed: %w", err)
	}

	fmt.Println()
	return nil
}

// makeProgressCallback 构建通用的进度回调函数。
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

// PushFileWithChain 通过跳板链上传文件。
// chainExpr 为 -J 参数值，格式与 assh -J 一致：逗号分隔的跳板列表。
func (s *TransferService) PushFileWithChain(ctx context.Context, name, chainExpr, localPath, remotePath string, opts TransferOptions) error {
	// 1. 验证本地路径
	localPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}
	if !s.localExists(localPath) {
		return fmt.Errorf("local file does not exist: %s", localPath)
	}
	isDir := s.localIsDir(localPath)
	if isDir && !opts.Recursive {
		return fmt.Errorf("omitting directory, use -r to upload directory")
	}

	// 2. 建立跳板连接
	sshClient, err := s.connectWithChain(name, chainExpr)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	// 3. 路径标准化
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

	// 4. 检查远程目录
	// 对于链连接，使用 sshClient 直接创建 SFTP 客户端检查
	gsftpClient, err := gsftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("sftp client creation failed: %w", err)
	}

	if !isDirExist(gsftpClient, s.remoteDir(remotePath)) {
		fmt.Printf("remote directory %s does not exist, create? ", s.remoteDir(remotePath))
		var answer string
		fmt.Scanln(&answer)
		if answer == "y" || answer == "Y" || answer == "yes" {
			if err := gsftpClient.MkdirAll(s.remoteDir(remotePath)); err != nil {
				gsftpClient.Close()
				return fmt.Errorf("create remote directory failed: %w", err)
			}
		} else {
			gsftpClient.Close()
			return fmt.Errorf("aborted")
		}
	}
	gsftpClient.Close()

	// 5. 传输
	progress := s.makeProgressCallback(opts)
	if err := s.transfer.PushWithSSHClient(ctx, sshClient, localPath, remotePath, s.toPortOpts(opts), progress); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	if opts.VerifyChecksum {
		fmt.Printf("\nverifying: %s\n", filepath.Base(localPath))
		if err := s.transfer.VerifyUploadWithSSHClient(sshClient, localPath, remotePath, true); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}
		fmt.Printf("  size match\n")
	}

	return nil
}

// PullFileWithChain 通过跳板链下载文件。
// chainExpr 为 -J 参数值，格式与 assh -J 一致。
func (s *TransferService) PullFileWithChain(ctx context.Context, name, chainExpr, remotePath, localPath string, opts TransferOptions) error {
	// 1. 建立跳板连接（先连接，需要用于 Exists/IsDir 检查）
	sshClient, err := s.connectWithChain(name, chainExpr)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	// 2. 创建 SFTP 客户端用于检查
	gsftpClient, err := gsftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("sftp client creation failed: %w", err)
	}

	// 3. 验证远程路径
	remotePath = s.normalizeRemotePath(remotePath)
	if !exists(gsftpClient, remotePath) {
		gsftpClient.Close()
		return fmt.Errorf("remote path does not exist: %s", remotePath)
	}
	remoteIsDir := isDir(gsftpClient, remotePath)
	if remoteIsDir && !opts.Recursive {
		gsftpClient.Close()
		return fmt.Errorf("omitting directory, use -r to download directory")
	}

	// 4. 处理本地路径
	localPath, err = filepath.Abs(localPath)
	if err != nil {
		gsftpClient.Close()
		return fmt.Errorf("invalid local path: %w", err)
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
			return fmt.Errorf("create local directory failed: %w", err)
		}
	}
	gsftpClient.Close()

	// 5. 传输
	progress := s.makeProgressCallback(opts)
	if err := s.transfer.PullWithSSHClient(ctx, sshClient, remotePath, localPath, s.toPortOpts(opts), progress); err != nil {
		return fmt.Errorf("pull failed: %w", err)
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
		return nil, fmt.Errorf("list failed: %w", err)
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
		return nil, fmt.Errorf("list failed: %w", err)
	}

	return files, nil
}

func (s *TransferService) RemoveRemote(name, remotePath string) error {
	server, err := s.getServer(name)
	if err != nil {
		return err
	}

	if err := s.transfer.Remove(server, remotePath); err != nil {
		return fmt.Errorf("remove failed: %w", err)
	}

	return nil
}

func (s *TransferService) MkdirRemote(name, remotePath string) error {
	server, err := s.getServer(name)
	if err != nil {
		return err
	}

	if err := s.transfer.Mkdir(server, remotePath); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}

	return nil
}

func (s *TransferService) getServer(name string) (*domain.Server, error) {
	if strings.Contains(name, "@") || strings.Contains(name, "-H") {
		return s.parseDirectServer(name)
	}

	server, err := s.repo.Get(name)
	if err != nil {
		return nil, fmt.Errorf("server not found: %s", name)
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
		return nil, fmt.Errorf("host is required")
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
		return fmt.Errorf("glob failed: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files matched pattern: %s", pattern)
	}

	for i, file := range files {
		fmt.Printf("(%d/%d) %s\n", i+1, len(files), filepath.Base(file))
		if err := s.PushFile(ctx, name, file, remotePath, opts); err != nil {
			fmt.Printf("  failed: %v\n", err)
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