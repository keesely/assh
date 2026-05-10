package port

import (
	"context"
	"io"

	"assh/asshc/domain"
)

// TransferProgress 定义文件传输进度回调函数类型。
// percent: 传输进度百分比 (0-100)
// bytes: 已传输的字节数
// total: 总字节数
type TransferProgress func(percent int, bytes, total int64)

// FileTransfer 定义文件传输接口。
// 支持 SFTP 协议的文件上传、下载、列表和删除操作。
type FileTransfer interface {
	// Push 上传本地文件或目录到远程服务器。
	// localPath: 本地文件或目录路径
	// remotePath: 远程目标路径
	// progress: 进度回调函数
	Push(ctx context.Context, server *domain.Server, localPath, remotePath string, progress TransferProgress) error

	// Pull 从远程服务器下载文件或目录到本地。
	// remotePath: 远程文件或目录路径
	// localPath: 本地目标路径
	// progress: 进度回调函数
	Pull(ctx context.Context, server *domain.Server, remotePath, localPath string, progress TransferProgress) error

	// List 列出远程目录内容。
	List(server *domain.Server, remotePath string) ([]FileInfo, error)

	// Remove 删除远程文件或目录。
	Remove(server *domain.Server, remotePath string) error

	// Mkdir 在远程服务器上创建目录。
	Mkdir(server *domain.Server, remotePath string) error
}

// FileInfo 定义远程文件信息。
type FileInfo struct {
	Name    string
	Size    int64
	Mode    string
	IsDir   bool
	ModTime string
}

// TransferResult 定义传输结果。
type TransferResult struct {
	Path       string
	Size       int64
	Success    bool
	Error      error
	DurationMs int64
}

// PushBatch 批量上传文件。
// 返回每个文件的传输结果。
func PushBatch(ctx context.Context, transfer FileTransfer, server *domain.Server, files []string, remoteDir string, progress TransferProgress) []TransferResult {
	results := make([]TransferResult, 0, len(files))
	for _, file := range files {
		remotePath := remoteDir + "/" + file
		err := transfer.Push(ctx, server, file, remotePath, progress)
		results = append(results, TransferResult{
			Path:    remotePath,
			Success: err == nil,
			Error:   err,
		})
	}
	return results
}

// PullBatch 批量下载文件。
// 返回每个文件的传输结果。
func PullBatch(ctx context.Context, transfer FileTransfer, server *domain.Server, remoteFiles []string, localDir string, progress TransferProgress) []TransferResult {
	results := make([]TransferResult, 0, len(remoteFiles))
	for _, file := range remoteFiles {
		localPath := localDir + "/" + file
		err := transfer.Pull(ctx, server, file, localPath, progress)
		results = append(results, TransferResult{
			Path:    localPath,
			Success: err == nil,
			Error:   err,
		})
	}
	return results
}

// ReaderAtAndWriterAt 是 io.ReadAt 和 io.WriterAt 接口的组合。
type ReaderAtAndWriterAt interface {
	io.ReaderAt
	io.WriterAt
}