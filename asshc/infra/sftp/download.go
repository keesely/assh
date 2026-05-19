package sftp

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type DownloadOptions struct {
	Resume         bool
	Progress       bool
	VerifyChecksum bool
	Concurrency    int
}

func PullFile(ctx context.Context, sshClient *ssh.Client, remotePath, localPath string, opts DownloadOptions, progress TransferProgress) error {
	client, err := sftp.NewClient(sshClient,
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
		sftp.MaxConcurrentRequestsPerFile(64),
	)
	if err != nil {
		return fmt.Errorf("sftp client creation failed: %w", err)
	}
	defer client.Close()

	info, err := client.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("remote file stat failed: %w", err)
	}

	if info.IsDir() {
		return pullDirectory(ctx, client, sshClient, remotePath, localPath, opts, progress)
	}

	return pullSingleFile(ctx, client, sshClient, remotePath, localPath, opts, progress)
}

func pullSingleFile(ctx context.Context, client *sftp.Client, sshClient *ssh.Client, remotePath, localPath string, opts DownloadOptions, progress TransferProgress) error {
	remoteFile, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote file failed: %w", err)
	}
	defer remoteFile.Close()

	remoteInfo, err := remoteFile.Stat()
	if err != nil {
		return fmt.Errorf("stat remote file failed: %w", err)
	}
	remoteSize := remoteInfo.Size()

	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("create local directory failed: %w", err)
	}

	var localFile *os.File
	var resumeOffset int64

	localExists := false
	localInfo, err := os.Stat(localPath)
	if err == nil {
		localExists = true
		if localInfo.Size() > remoteSize+int64(HeaderSize) {
			return fmt.Errorf("local file is larger than remote file, resume not available")
		}
	}

	if opts.Resume && localExists {
		if headerData, err := readLocalHeader(localPath); err == nil {
			if header, parseErr := ParseHashHeader(headerData); parseErr == nil {
				if localInfo.Size() == header.OrigSize+int64(HeaderSize) {
					remoteHash, hashErr := computeRemoteHash(sshClient, remotePath)
					if hashErr == nil {
						if bytesEqual(header.SHA256[:], remoteHash) {
							resumeOffset = header.OrigSize
							if resumeOffset >= remoteSize {
								return nil
							}
							fmt.Printf("resuming from offset %d\n", resumeOffset)
						}
					}
				}
			}
		}

		if resumeOffset == 0 && localInfo.Size() >= remoteSize {
			localHash, _ := computeLocalHashAtFile(localPath, 0, localInfo.Size())
			remoteHash, _ := computeRemoteHash(sshClient, remotePath)
			if localHash == remoteHash {
				return nil
			}
		}
	}

	if resumeOffset > 0 {
		localFile, err = os.OpenFile(localPath, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		localFile, err = os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	}
	if err != nil {
		return fmt.Errorf("open local file failed: %w", err)
	}
	defer localFile.Close()

	if resumeOffset > 0 {
		if _, err := remoteFile.Seek(resumeOffset, io.SeekStart); err != nil {
			return fmt.Errorf("seek remote file failed: %w", err)
		}
	}

	startTime := time.Now()
	totalBytes := remoteSize - resumeOffset
	bytesWritten := int64(0)
	tracker := NewProgressTracker()

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := remoteFile.Read(buf)
		if n > 0 {
			written, writeErr := localFile.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("write to local file failed: %w", writeErr)
			}
			bytesWritten += int64(written)

			if progress != nil {
				rate, eta := tracker.Update(bytesWritten)
				progress(ProgressInfo{
					Progress:   float64(bytesWritten) / float64(totalBytes),
					Rate:       rate,
					ETA:        eta,
					Bytes:      bytesWritten,
					TotalBytes: totalBytes,
				})
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read remote file failed: %w", err)
		}
	}

	duration := time.Since(startTime).Milliseconds()

	if opts.VerifyChecksum {
		if hash, err := ComputeLocalSHA256(localPath); err == nil {
			fmt.Printf("\nverifying: %s\n  sha256: %s\n", filepath.Base(localPath), hash)
		}
	}

	_ = duration
	return nil
}

func pullDirectory(ctx context.Context, client *sftp.Client, sshClient *ssh.Client, remoteDir, localDir string, opts DownloadOptions, progress TransferProgress) error {
	files, err := collectRemoteFiles(client, remoteDir, remoteDir, 0)
	if err != nil {
		return err
	}

	for i, remotePath := range files {
		relPath, _ := filepath.Rel(remoteDir, remotePath)
		localPath := filepath.Join(localDir, relPath)

		remotePath = strings.ReplaceAll(remotePath, string(filepath.Separator), "/")

		progressFile := progress
		if progress != nil {
			idx := i + 1
			total := len(files)
			currentProgress := progressFile
			progressFile = func(info ProgressInfo) {
				info.Index = idx
				info.Total = total
				info.FileName = filepath.Base(remotePath)
				currentProgress(info)
			}
		}

		if err := pullSingleFile(ctx, client, sshClient, remotePath, localPath, opts, progressFile); err != nil {
			return fmt.Errorf("pull %s failed: %w", remotePath, err)
		}
	}

	return nil
}

func collectRemoteFiles(client *sftp.Client, baseDir, currentDir string, depth int) ([]string, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("max directory depth exceeded (%d)", maxDepth)
	}

	var files []string

	entries, err := client.ReadDir(currentDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		path := currentDir + "/" + entry.Name()
		if entry.IsDir() {
			subFiles, err := collectRemoteFiles(client, baseDir, path, depth+1)
			if err != nil {
				return nil, err
			}
			files = append(files, subFiles...)
		} else {
			files = append(files, path)
		}

		if len(files) > maxFiles {
			return nil, fmt.Errorf("max file count exceeded (%d)", maxFiles)
		}
	}

	return files, nil
}

func readLocalHeader(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, HeaderSize)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n < HeaderSize {
		return nil, fmt.Errorf("file too small for header")
	}
	return header, nil
}

func computeLocalHashAtFile(filePath string, offset, length int64) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}

	h := sha256.New()
	remaining := length
	buf := make([]byte, 32*1024)
	for remaining > 0 {
		toRead := int64(len(buf))
		if toRead > remaining {
			toRead = remaining
		}
		n, err := f.Read(buf[:toRead])
		if err != nil && err != io.EOF {
			return "", err
		}
		if n == 0 {
			break
		}
		h.Write(buf[:n])
		remaining -= int64(n)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func bytesEqual(a []byte, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func GlobRemote(client *sftp.Client, basePath, pattern string) ([]string, error) {
	return nil, fmt.Errorf("glob not implemented for remote")
}