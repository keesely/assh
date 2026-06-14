# BUG-009: ComputeRemoteSHA256 是 stub，-S 校验总是失败

## Status
known

## Date
2026-05-11

## Description

`ComputeRemoteSHA256()` 在 `asshc/infra/sftp/verify.go:130-132` 中是一个 stub：

```go
func ComputeRemoteSHA256(client *sftp.Client, remotePath string) (string, error) {
    return "", fmt.Errorf("not implemented: use ssh exec to run sha256sum")
}
```

这导致以下功能链完全断裂：

1. 用户执行 `assh-fs push -S t ./file /tmp/` 推文件
2. 文件传输成功 (100%)
3. Service 层调用 `s.transfer.VerifyUpload(server, localPath, remotePath, true)` 进入验证
4. Infra 层的 `VerifyUpload()` (transfer.go) 设置 `verifyChecksum=true`
5. `VerifyUpload()` (verify.go) 计算本地 SHA256 成功，然后调用 `ComputeRemoteSHA256()` 
6. 远程计算返回 error → `result.Error` 被设置
7. 回到 transfer.go → `result.Error != nil` → return error
8. 最终错误：`"verification failed: not implemented: use ssh exec to run sha256sum"`

**影响**：
- F16（SHA256 校验）：❌ 实际上不可用，但 F16 原验收标注为 ✅（仅测试了 without `-S`，验证步骤没走）
- F17（验证失败检测）：❌ 未实现，核心原因是 `ComputeRemoteSHA256` 未实现

**注意**：`VerifyUpload` 中的 `result.SizeMatch` 检查是正常的（对比本地和远程文件大小），但 `verifyChecksum=true` 时必走到 `ComputeRemoteSHA256` 的 error path。

## Solution

需要实现远程 SHA256 计算。有两种方案：

### 方案 A：SSH exec 执行 sha256sum（推荐）
通过 SSH session 在远程服务器上执行 `sha256sum <file>` 命令来计算：

```go
func ComputeRemoteSHA256(client *sftp.Client, remotePath string) (string, error) {
    // 通过 sshClient (需要从外部传入) 执行 sha256sum
    // sshClient 可以通过 client 获取底层连接
    session, err := sshClient.NewSession()
    if err != nil {
        return "", err
    }
    defer session.Close()
    
    out, err := session.Output("sha256sum " + remotePath)
    if err != nil {
        return "", err
    }
    
    parts := strings.Fields(string(out))
    if len(parts) > 0 {
        return parts[0], nil
    }
    return "", fmt.Errorf("unexpected sha256sum output: %s", out)
}
```

### 方案 B：SFTP 读取+本地计算
通过 SFTP 读取远程文件内容，在本地计算 SHA256（对大数据量效率低）。

### 方案 A 需要修改
- computeRemoteHash 也需要类似修复（在 download.go 中也有使用）
- `VerifyUpload` 需要 sshClient 访问权限（可能需要从方法参数传入）

## Affected Files

- `asshc/infra/sftp/verify.go:130-132` — `ComputeRemoteSHA256` stub
- `asshc/infra/sftp/download.go` — `computeRemoteHash()` 依赖同 stub
- `asshc/infra/sftp/transfer.go` — `SFTPTransfer.VerifyUpload()` 只传了 sftp client 没有 ssh client

## Related

- Phase 5 验收测试 F16/F17
- ADR-017: SFTP 文件传输架构设计
