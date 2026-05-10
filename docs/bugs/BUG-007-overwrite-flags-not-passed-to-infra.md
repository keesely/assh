# BUG-007: -f/--skip 覆盖控制参数未传递到 infra 层

## Status
known

## Date
2026-05-11

## Description

`-f`（强制覆盖）和 `--skip`（跳过已存在）参数在 CLI handler 中解析并设置 `opts.Overwrite = "force"/"skip"`，但 `opts.Overwrite` 在 service 层和 infra 层**从未被读取**：

1. **CLI handler** (`cmd/app.go`): `-f`/`--skip` 正确设置 `opts.Overwrite`
2. **Service 层** (`asshc/service/transfer.go`): `TransferOptions.Overwrite` 字段存在，但在 `PushFile()`/`PushFileDirect()` 中从未读取
3. **Infra 层** (`asshc/infra/sftp/upload.go`): `UploadOptions` 没有 `Overwrite` 字段，始终使用 `O_CREATE|O_TRUNC` 无条件覆写

**后果**：
- F6（`-f` 强制覆盖）：✅ 表面通过，但成功是因为 infra 层**总是**覆写，而非 `-f` 生效
- F7（`--skip` 跳过已存在）：❌ 未实现，即使加 `--skip` 也覆写
- F8（默认询问）：❌ 未实现，始终静默覆写

**唯一保护**：`pushSingleFile()` 中有大小检查——当远程文件比本地大时（且无 header 标识），报错"remote file is larger than local file, resume not available"。但这个检查不以 `-f`/`--skip` 为条件。

## Solution

方案有两个方向：

### 短期修复（传递到 infra）
在 `Push()` 和 `Pull()` 方法 (`asshc/infra/sftp/transfer.go`) 中添加 `Overwrite` 字段到 `UploadOptions`/`DownloadOptions`，并在 `pushSingleFile()` 中根据 `Overwrite` 值做不同处理：

```go
type UploadOptions struct {
    Resume         bool
    Progress       bool
    VerifyChecksum bool
    Concurrency    int
    Overwrite      string  // "force", "skip", "ask", ""
}
```

### 长期方案（交互式确认）
在 service 层添加交互式覆盖确认逻辑（类似现有的目录创建确认模式）：

```go
if opts.Overwrite == "" && remoteExists {
    fmt.Printf("remote file %s exists, overwrite? ", remotePath)
    var answer string
    fmt.Scanln(&answer)
    if answer != "y" && answer != "Y" {
        return fmt.Errorf("aborted")
    }
}
```

## Affected Files

- `asshc/service/transfer.go` — `TransferOptions.Overwrite` 未使用
- `asshc/infra/sftp/transfer.go` — `Push()` 方法不传递 Overwrite/Resume
- `asshc/infra/sftp/upload.go` — `UploadOptions` 无 Overwrite 字段
- `cmd/app.go` — handler 中 opts.Overwrite 设置正确但无效果

## Related

- Phase 5 验收测试 F6/F7/F8
- BUG-008: --resume 参数未传递到 infra 层
