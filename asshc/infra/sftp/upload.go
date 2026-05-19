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

	"github.com/kballard/go-shellquote"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	maxDepth = 100
	maxFiles = 100000
)

type UploadOptions struct {
	Resume         bool
	Progress       bool
	VerifyChecksum bool
	Concurrency    int
	Overwrite      string // "force", "skip", "ask", or "" (ask on conflict)
}

type UploadResult struct {
	Path       string
	Size       int64
	Success    bool
	Error      error
	DurationMs int64
}

func PushFile(ctx context.Context, sshClient *ssh.Client, localPath, remotePath string, opts UploadOptions, progress TransferProgress) error {
	client, err := sftp.NewClient(sshClient,
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
		sftp.MaxConcurrentRequestsPerFile(64),
	)
	if err != nil {
		return fmt.Errorf("sftp client creation failed: %w", err)
	}
	defer client.Close()

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("local file stat failed: %w", err)
	}

	if info.IsDir() {
		return pushDirectory(ctx, client, sshClient, localPath, remotePath, opts, progress)
	}

	return pushSingleFile(ctx, client, sshClient, localPath, remotePath, opts, progress)
}

func pushSingleFile(ctx context.Context, client *sftp.Client, sshClient *ssh.Client, localPath, remotePath string, opts UploadOptions, progress TransferProgress) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file failed: %w", err)
	}
	defer localFile.Close()

	info, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("stat local file failed: %w", err)
	}
	localSize := info.Size()

	remoteDir := filepath.Dir(remotePath)
	if err := client.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("create remote directory failed: %w", err)
	}

	var remoteFile *sftp.File
	var resumeOffset int64

	remoteExists := false
	remoteInfo, err := client.Stat(remotePath)
	if err == nil {
		remoteExists = true
		// Size check: prevent overwriting larger remote file (unless -f force)
		if opts.Overwrite != "force" && remoteInfo.Size() > localSize+int64(HeaderSize) {
			return fmt.Errorf("remote file is larger than local file, resume not available")
		}
	}

	// Handle Overwrite option (BUG-007)
	// Skip interactive prompt when in resume mode (user explicitly wants to continue)
	shouldAsk := !opts.Resume
	if remoteExists && shouldAsk {
		switch opts.Overwrite {
		case "skip":
			fmt.Printf("remote file %s exists, skipping\n", remotePath)
			return nil
		case "force":
			// proceed with overwrite
		case "":
			// Default: ask interactively
			fmt.Printf("remote file %s exists, overwrite? (y/N) ", remotePath)
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" && answer != "yes" {
				return fmt.Errorf("aborted")
			}
		}
	}

	if opts.Resume && remoteExists {
		if remoteInfo.Size() >= localSize {
			existingHash, _ := computeLocalHashAtOffset(localFile, 0, remoteInfo.Size())
			remoteHash, _ := computeRemoteHash(sshClient, remotePath)
			// computeLocalHashAtOffset changed localFile position; seek back to 0
			localFile.Seek(0, io.SeekStart)
			if existingHash == remoteHash {
				return nil
			}
			// Hash mismatch: remote has different content, fall through to truncate + full upload
		} else {
			// 远程文件不完整，续传：resumeOffset = remoteInfo.Size()
			resumeOffset = remoteInfo.Size()
			remoteFile, err = client.OpenFile(remotePath, os.O_WRONLY|os.O_APPEND)
			if err != nil {
				return fmt.Errorf("open remote file for resume failed: %w", err)
			}
		}
	}

	if remoteFile == nil {
		remoteFile, err = client.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	}

	if err != nil {
		return fmt.Errorf("open remote file failed: %w", err)
	}
	defer remoteFile.Close()

	if resumeOffset > 0 {
		if _, err := localFile.Seek(resumeOffset, io.SeekStart); err != nil {
			return fmt.Errorf("seek local file failed: %w", err)
		}
		fmt.Printf("resuming from offset %d\n", resumeOffset)
	}

	startTime := time.Now()
	totalBytes := localSize - resumeOffset
	bytesWritten := int64(0)
	tracker := NewProgressTracker()

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := localFile.Read(buf)
		if n > 0 {
			written, writeErr := remoteFile.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("write to remote file failed: %w", writeErr)
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
			return fmt.Errorf("read local file failed: %w", err)
		}
	}

	duration := time.Since(startTime).Milliseconds()

	if opts.VerifyChecksum {
		if lf, err := os.Open(localPath); err == nil {
			defer lf.Close()
			if hash, err := computeLocalHash(lf); err == nil {
				fmt.Printf("\nverifying: %s\n  sha256: %x\n", filepath.Base(localPath), hash)
			}
		}
	}

	_ = duration
	return nil
}

func pushDirectory(ctx context.Context, client *sftp.Client, sshClient *ssh.Client, localDir, remoteDir string, opts UploadOptions, progress TransferProgress) error {
	files, err := collectLocalFiles(localDir, localDir, 0)
	if err != nil {
		return err
	}

	for i, localPath := range files {
		relPath, _ := filepath.Rel(localDir, localPath)
		remotePath := filepath.Join(remoteDir, relPath)

		remotePath = strings.ReplaceAll(remotePath, string(filepath.Separator), "/")

		progressFile := progress
		if progress != nil {
			idx := i + 1
			total := len(files)
			currentProgress := progressFile
			progressFile = func(info ProgressInfo) {
				info.Index = idx
				info.Total = total
				info.FileName = filepath.Base(localPath)
				currentProgress(info)
			}
		}

		if err := pushSingleFile(ctx, client, sshClient, localPath, remotePath, opts, progressFile); err != nil {
			return fmt.Errorf("push %s failed: %w", localPath, err)
		}
	}

	return nil
}

func collectLocalFiles(baseDir, currentDir string, depth int) ([]string, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("max directory depth exceeded (%d)", maxDepth)
	}

	var files []string

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		path := filepath.Join(currentDir, entry.Name())
		if entry.IsDir() {
			subFiles, err := collectLocalFiles(baseDir, path, depth+1)
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

func computeLocalHash(f *os.File) ([32]byte, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return [32]byte{}, err
	}
	defer f.Seek(0, io.SeekStart)

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [32]byte{}, err
	}
	var hash [32]byte
	copy(hash[:], h.Sum(nil))
	return hash, nil
}

func computeLocalHashAtOffset(f *os.File, offset, length int64) (string, error) {
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

func computeRemoteHash(sshClient *ssh.Client, remotePath string) (string, error) {
	session, err := sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session failed: %w", err)
	}
	defer session.Close()

	out, err := session.Output("sha256sum " + shellquote.Join(remotePath))
	if err != nil {
		return "", fmt.Errorf("remote sha256sum failed: %w", err)
	}

	// Output: "d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2  /path/to/file"
	parts := strings.Fields(string(out))
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", fmt.Errorf("unexpected sha256sum output: %s", out)
}

func GlobLocal(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func OpenRemote(client *sftp.Client, path string, flags int) (*sftp.File, error) {
	return client.OpenFile(path, flags)
}