# BUG-008: --resume 参数未从 service 层传递到 infra 层

## Status
known

## Date
2026-05-11

## Description

`--resume`（断点续传）参数在 CLI handler 中解析并设置 `opts.Resume = true`，但该选项**未从 service 层传递到 infra 层**，导致断点续传功能完全不可用。

**数据流断裂点**：

1. **CLI handler** (`cmd/app.go`): `opts.Resume = c.Bool("e") || c.Bool("resume")` ✅
2. **Service 层** (`asshc/service/transfer.go`): `TransferOptions.Resume` 字段存在，但 `PushFile()` 调用 `s.transfer.Push()` 时**只传了 `progress`**，不传 `opts.Resume`
3. **Infra 层** (`asshc/infra/sftp/transfer.go`): `Push()` 方法构造 `UploadOptions{Progress: progress != nil}` — **硬编码，不含 Resume**
4. **上传函数** (`asshc/infra/sftp/upload.go`): `pushSingleFile()` 读取 `opts.Resume`，但因为始终为 `false`，续传逻辑永远不会触发

**后果**：
- F18（批量续传）：❌ 未实现，`--resume` 被忽略
- F5（断点续传）：⚠️ 表面通过，但实际行为等同于完整重传
- 任何 `--resume` push 都表现为完整文件传输，只是凑巧能完成

## Solution

需要修改数据流，将 `Resume`（以及未来 `Concurrency`、`Overwrite`）选项传递到 infra 层：

### 步骤 1：修改 `SFTPTransfer.Push()` 和 `Pull()` 方法签名
```go
// 当前（丢失选项）
func (t *SFTPTransfer) Push(ctx context.Context, server *domain.Server, localPath, remotePath string, progress port.TransferProgress) error

// 建议
type TransferOptions struct {
    Resume    bool
    Progress  bool
    Overwrite string
}

func (t *SFTPTransfer) Push(ctx context.Context, server *domain.Server, localPath, remotePath string, opts TransferOptions, progress port.TransferProgress) error
```

### 步骤 2：同步修改 port/transfer.go 接口
`FileTransfer` 接口中的 `Push`/`Pull` 签名需要同步更新。

### 步骤 3：service 层传递选项
```go
// 在 PushFile() 中
uploadOpts := TransferOptions{
    Resume:   opts.Resume,
    Progress: opts.Progress,
}
if err := s.transfer.Push(ctx, server, localPath, remotePath, uploadOpts, progress); err != nil {
```

## Affected Files

- `asshc/port/transfer.go` — `FileTransfer` 接口 Push/Pull 签名
- `asshc/infra/sftp/transfer.go` — `SFTPTransfer.Push()`/`Pull()` 方法和选项构造
- `asshc/infra/sftp/upload.go` — 底层 PushFile + UploadOptions
- `asshc/infra/sftp/download.go` — 底层 PullFile + DownloadOptions
- `asshc/service/transfer.go` — `PushFile()`/`PushFileDirect()` 选项传递

## Related

- Phase 5 验收测试 F5/F18
- BUG-007: -f/--skip 覆盖控制参数未传递到 infra 层
