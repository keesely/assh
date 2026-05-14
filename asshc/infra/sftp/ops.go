package sftp

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/sftp"
)

type FileInfo struct {
	Name    string
	Size    int64
	Mode    string
	IsDir   bool
	ModTime string
}

func List(client *sftp.Client, remotePath string) ([]FileInfo, error) {
	entries, err := client.ReadDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("read dir failed: %w", err)
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Size:    entry.Size(),
			Mode:    entry.Mode().String(),
			IsDir:   entry.IsDir(),
			ModTime: entry.ModTime().Format(time.RFC3339),
		})
	}
	return files, nil
}

func Remove(client *sftp.Client, remotePath string) error {
	info, err := client.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("stat failed: %w", err)
	}

	if info.IsDir() {
		return client.RemoveDirectory(remotePath)
	}
	return client.Remove(remotePath)
}

func Mkdir(client *sftp.Client, remotePath string) error {
	return client.MkdirAll(remotePath)
}

func Rmdir(client *sftp.Client, remotePath string) error {
	return client.RemoveDirectory(remotePath)
}

func Rename(client *sftp.Client, oldPath, newPath string) error {
	return client.Rename(oldPath, newPath)
}

func Stat(client *sftp.Client, remotePath string) (*FileInfo, error) {
	info, err := client.Stat(remotePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path does not exist: %s", remotePath)
		}
		return nil, fmt.Errorf("stat failed: %w", err)
	}

	return &FileInfo{
		Name:    info.Name(),
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().Format(time.RFC3339),
	}, nil
}

func Exists(client *sftp.Client, remotePath string) bool {
	_, err := client.Stat(remotePath)
	return err == nil
}

func IsDir(client *sftp.Client, remotePath string) bool {
	info, err := client.Stat(remotePath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func Chmod(client *sftp.Client, remotePath string, mode uint32) error {
	return client.Chmod(remotePath, os.FileMode(mode))
}

func DiskUsage(client *sftp.Client, path string) (total, used, free int64, err error) {
	entries, err := client.ReadDir(path)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("disk usage read dir failed: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subTotal, subUsed, _, subErr := DiskUsage(client, path+"/"+entry.Name())
			if subErr == nil {
				total += subTotal
				used += subUsed
			}
		} else {
			total += entry.Size()
			used += entry.Size()
		}
	}

	return total, used, free, nil
}

func Umask(client *sftp.Client) uint32 {
	return 022
}