package sftp

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type VerifyResult struct {
	Path        string
	SizeMatch   bool
	HashMatch   bool
	SHA256Local  string
	SHA256Remote string
	Error       error
}

func VerifyLocalSize(filePath string, expectedSize int64) (bool, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}
	return info.Size() == expectedSize, nil
}

func VerifyRemoteSize(client *sftp.Client, remotePath string, expectedSize int64) (bool, error) {
	info, err := client.Stat(remotePath)
	if err != nil {
		return false, err
	}
	return info.Size() == expectedSize, nil
}

func ComputeLocalSHA256(filePath string) (string, error) {
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

func VerifyTransfer(client *sftp.Client, localPath, remotePath string, verifyChecksum bool) *VerifyResult {
	result := &VerifyResult{
		Path: remotePath,
	}

	localInfo, err := os.Stat(localPath)
	if err != nil {
		result.Error = fmt.Errorf("local stat failed: %w", err)
		return result
	}

	remoteInfo, err := client.Stat(remotePath)
	if err != nil {
		result.Error = fmt.Errorf("remote stat failed: %w", err)
		return result
	}

	result.SizeMatch = localInfo.Size() == remoteInfo.Size()
	if !result.SizeMatch {
		return result
	}

	if verifyChecksum {
		localHash, err := ComputeLocalSHA256(localPath)
		if err != nil {
			result.Error = err
			return result
		}
		result.SHA256Local = localHash
	}

	return result
}

func VerifyUpload(client *sftp.Client, sshClient *ssh.Client, localPath, remotePath string, verifyChecksum bool) *VerifyResult {
	result := &VerifyResult{
		Path: remotePath,
	}

	localInfo, err := os.Stat(localPath)
	if err != nil {
		result.Error = fmt.Errorf("local stat failed: %w", err)
		return result
	}

	remoteInfo, err := client.Stat(remotePath)
	if err != nil {
		result.Error = fmt.Errorf("remote stat failed: %w", err)
		return result
	}

	result.SizeMatch = localInfo.Size() == remoteInfo.Size()

	if !result.SizeMatch {
		return result
	}

	if verifyChecksum {
		localHash, err := ComputeLocalSHA256(localPath)
		if err != nil {
			result.Error = err
			return result
		}
		result.SHA256Local = localHash

		if sshClient != nil {
			remoteHash, err := ComputeRemoteSHA256Exec(sshClient, remotePath)
			if err != nil {
				result.Error = err
				return result
			}
			result.SHA256Remote = remoteHash
			result.HashMatch = localHash == remoteHash
		}
	}

	return result
}

// ComputeRemoteSHA256Exec runs sha256sum on the remote server via SSH and returns the hash.
func ComputeRemoteSHA256Exec(sshClient *ssh.Client, remotePath string) (string, error) {
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

func VerifyDownload(client *sftp.Client, localPath, remotePath string, verifyChecksum bool) *VerifyResult {
	result := &VerifyResult{
		Path: localPath,
	}

	localInfo, err := os.Stat(localPath)
	if err != nil {
		result.Error = fmt.Errorf("local stat failed: %w", err)
		return result
	}

	remoteInfo, err := client.Stat(remotePath)
	if err != nil {
		result.Error = fmt.Errorf("remote stat failed: %w", err)
		return result
	}

	result.SizeMatch = localInfo.Size() == remoteInfo.Size()

	if !result.SizeMatch {
		return result
	}

	if verifyChecksum {
		localHash, err := ComputeLocalSHA256(localPath)
		if err != nil {
			result.Error = err
			return result
		}
		result.SHA256Local = localHash
	}

	return result
}

func PrintVerifyResult(result *VerifyResult) {
	if result.Error != nil {
		fmt.Printf("  ✗ verification error: %v\n", result.Error)
		return
	}

	if result.SizeMatch {
		fmt.Printf("  ✓ size match: %s\n", formatSize(result.Path))
	} else {
		fmt.Printf("  ✗ size mismatch\n")
	}

	if result.SHA256Local != "" {
		fmt.Printf("  ✓ sha256: %s", result.SHA256Local[:16])
		if result.SHA256Remote != "" {
			fmt.Printf("... (%s)\n", result.SHA256Remote[:16])
		} else {
			fmt.Println()
		}
		if result.HashMatch {
			fmt.Println("  ✓ hash match")
		} else if result.SHA256Remote != "" {
			fmt.Println("  ✗ hash mismatch")
		}
	}
}

func formatSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "unknown"
	}
	size := float64(info.Size())
	if size < 1024 {
		return fmt.Sprintf("%.0fB", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", size/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", size/1024/1024)
	}
	return fmt.Sprintf("%.1fGB", size/1024/1024/1024)
}

// ComputeRemoteSHA256 is kept for backward compatibility but deprecated.
func ComputeRemoteSHA256(client *sftp.Client, remotePath string) (string, error) {
	return "", fmt.Errorf("not implemented: use ComputeRemoteSHA256Exec with ssh.Client instead")
}