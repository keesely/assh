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
	transfer  *sftp.SFTPTransfer
	repo      port.ServerRepository
}

func NewTransferService(transfer *sftp.SFTPTransfer, repo port.ServerRepository) *TransferService {
	return &TransferService{
		transfer: transfer,
		repo:     repo,
	}
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

	if err := s.transfer.Push(ctx, server, localPath, remotePath, progress); err != nil {
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

	if err := s.transfer.Push(ctx, server, localPath, remotePath, progress); err != nil {
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

	if s.transfer.IsDir(server, remotePath) && !opts.Recursive {
		return fmt.Errorf("omitting directory, use -r to download directory")
	}

	localPath, err = filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("invalid local path: %w", err)
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

	if err := s.transfer.Pull(ctx, server, remotePath, localPath, progress); err != nil {
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

	isDir := s.transfer.IsDir(server, remotePath)
	if isDir && !opts.Recursive {
		return fmt.Errorf("omitting directory, use -r to download directory")
	}

	if isDir {
		baseName := filepath.Base(remotePath)
		localPath = filepath.Join(localPath, baseName)
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

	if err := s.transfer.Pull(ctx, server, remotePath, localPath, progress); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	fmt.Println()
	return nil
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