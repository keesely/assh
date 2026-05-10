package sftp

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"assh/asshc/domain"
	"assh/asshc/port"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	ErrSizeMismatch = errors.New("size mismatch")
)

type SFTPTransfer struct {
	sshConnector func(server *domain.Server) (*ssh.Client, error)
}

func NewSFTPTransfer(sshConnector func(server *domain.Server) (*ssh.Client, error)) *SFTPTransfer {
	return &SFTPTransfer{
		sshConnector: sshConnector,
	}
}

func (t *SFTPTransfer) Push(ctx context.Context, server *domain.Server, localPath, remotePath string, progress port.TransferProgress) error {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	opts := UploadOptions{
		Progress: progress != nil,
	}

	progressAdapter := func(info ProgressInfo) {
		if progress != nil {
			progress(port.TransferInfo{
				Index:      info.Index,
				Total:      info.Total,
				FileName:   info.FileName,
				Progress:   info.Progress,
				Rate:       info.Rate,
				ETA:        info.ETA,
				Bytes:      info.Bytes,
				TotalBytes: info.TotalBytes,
			})
		}
	}

	return PushFile(ctx, sshClient, localPath, remotePath, opts, progressAdapter)
}

func (t *SFTPTransfer) Pull(ctx context.Context, server *domain.Server, remotePath, localPath string, progress port.TransferProgress) error {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	opts := DownloadOptions{
		Progress: progress != nil,
	}

	progressAdapter := func(info ProgressInfo) {
		if progress != nil {
			progress(port.TransferInfo{
				Index:      info.Index,
				Total:      info.Total,
				FileName:   info.FileName,
				Progress:   info.Progress,
				Rate:       info.Rate,
				ETA:        info.ETA,
				Bytes:      info.Bytes,
				TotalBytes: info.TotalBytes,
			})
		}
	}

	return PullFile(ctx, sshClient, remotePath, localPath, opts, progressAdapter)
}

func (t *SFTPTransfer) List(server *domain.Server, remotePath string) ([]port.FileInfo, error) {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	files, err := List(client, remotePath)
	if err != nil {
		return nil, err
	}

	result := make([]port.FileInfo, len(files))
	for i, f := range files {
		result[i] = port.FileInfo{
			Name:    f.Name,
			Size:    f.Size,
			Mode:    f.Mode,
			IsDir:   f.IsDir,
			ModTime: f.ModTime,
		}
	}
	return result, nil
}

func (t *SFTPTransfer) Remove(server *domain.Server, remotePath string) error {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()

	return Remove(client, remotePath)
}

func (t *SFTPTransfer) Mkdir(server *domain.Server, remotePath string) error {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()

	return Mkdir(client, remotePath)
}

func (t *SFTPTransfer) StatRemote(server *domain.Server, remotePath string) (*port.FileInfo, error) {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	info, err := Stat(client, remotePath)
	if err != nil {
		return nil, err
	}

	return &port.FileInfo{
		Name:    info.Name,
		Size:    info.Size,
		Mode:    info.Mode,
		IsDir:   info.IsDir,
		ModTime: info.ModTime,
	}, nil
}

func (t *SFTPTransfer) Exists(server *domain.Server, remotePath string) bool {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return false
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return false
	}
	defer client.Close()

	return Exists(client, remotePath)
}

func (t *SFTPTransfer) IsDir(server *domain.Server, remotePath string) bool {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return false
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return false
	}
	defer client.Close()

	return IsDir(client, remotePath)
}

func (t *SFTPTransfer) Rename(server *domain.Server, oldPath, newPath string) error {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()

	return Rename(client, oldPath, newPath)
}

func (t *SFTPTransfer) Rmdir(server *domain.Server, remotePath string) error {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()

	return Rmdir(client, remotePath)
}

func (t *SFTPTransfer) Chmod(server *domain.Server, remotePath string, mode uint32) error {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()

	return Chmod(client, remotePath, mode)
}

func (t *SFTPTransfer) VerifyUpload(server *domain.Server, localPath, remotePath string, verifyChecksum bool) error {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()

	result := VerifyUpload(client, localPath, remotePath, verifyChecksum)
	if result.Error != nil {
		return result.Error
	}
	if !result.SizeMatch {
		return ErrSizeMismatch
	}
	if result.HashMatch == false && result.SHA256Remote != "" {
		return port.ErrHashMismatch
	}
	return nil
}

func (t *SFTPTransfer) ComputeLocalHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (t *SFTPTransfer) GlobLocal(pattern string) ([]string, error) {
	return GlobLocal(pattern)
}

func (t *SFTPTransfer) EnsureLocalDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func (t *SFTPTransfer) LocalExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (t *SFTPTransfer) LocalIsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (t *SFTPTransfer) CollectLocalFiles(dir string) ([]string, error) {
	return collectLocalFiles(dir, dir)
}

func (t *SFTPTransfer) CollectRemoteFiles(server *domain.Server, dir string) ([]string, error) {
	sshClient, err := t.sshConnector(server)
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()

	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	files, err := collectRemoteFiles(client, dir, dir)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (t *SFTPTransfer) NormalizeRemotePath(path string) string {
	return filepath.ToSlash(path)
}

func (t *SFTPTransfer) RemoteBaseName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

func (t *SFTPTransfer) RemoteDirName(path string) string {
	normalized := t.NormalizeRemotePath(path)
	parts := strings.Split(normalized, "/")
	if len(parts) <= 1 {
		return "/"
	}
	return strings.Join(parts[:len(parts)-1], "/")
}

func (t *SFTPTransfer) RemoteJoin(parts ...string) string {
	var normalized []string
	for _, p := range parts {
		normalized = append(normalized, filepath.ToSlash(p))
	}
	result := strings.Join(normalized, "/")
	result = strings.ReplaceAll(result, "//", "/")
	return result
}