# SFTP 模块

**依赖库**: `github.com/pkg/sftp` v1.13.7+
**架构决策**: ADR-017 + ADR-019 (Hash Header 安全验证)
**设计文档**: TODO.md Phase 5

## 接口定义 — `asshc/port/transfer.go`（已有，需更新）

```go
// TransferOptions 定义文件传输选项。
type TransferOptions struct {
    Resume         bool
    Overwrite      string // "force", "skip", "ask", or "" (ask on conflict)
    VerifyChecksum bool
    Concurrency    int
}

// TransferInfo 定义进度回调信息。
type TransferInfo struct {
    Index      int     // 当前文件序号（1-based）
    Total      int     // 总文件数
    FileName   string  // 当前文件名
    Progress   float64 // 0.0 ~ 1.0
    Rate       string  // 已格式化速率，如 "2.1MB/s"
    ETA        string  // 已格式化剩余时间，如 "4s"
    Bytes      int64   // 已传输字节数
    TotalBytes int64   // 总字节数
}
type TransferProgress func(info TransferInfo)

type FileTransfer interface {
    Push(ctx context.Context, server *domain.Server, localPath, remotePath string, opts TransferOptions, progress TransferProgress) error
    Pull(ctx context.Context, server *domain.Server, remotePath, localPath string, opts TransferOptions, progress TransferProgress) error
    List(server *domain.Server, remotePath string) ([]FileInfo, error)
    Remove(server *domain.Server, remotePath string) error
    Mkdir(server *domain.Server, remotePath string) error
}
```

## 文件结构

```
asshc/infra/sftp/
├── client.go      # SFTPSession: SFTP 连接管理（创建/关闭/重连 + 并发配置）
├── header.go      # HashHeader: 256B 固定头部构建/解析/剥离（ADR-019）NEW
├── upload.go      # Push 实现: 单文件/目录递归(-r)/断点续传(--resume)/Glob匹配
├── download.go    # Pull 实现: 单文件/目录递归(-r)/断点续传(--resume)/Glob匹配
├── ops.go         # List/Remove/Mkdir 实现
├── progress.go    # 进度显示: 进度条/速率/ETA
└── verify.go      # 传输后验证: 大小比对 + SHA256 校验
```

## CLI 命令 — `assh-fs`（独立二进制）

`assh-fs` 是与 `assh` 平行独立的可执行命令，共享同一套核心模块（config/asshc/...），专注于文件系统操作。

### push 命令
```bash
# 基本用法
assh-fs push <server> <local_path> <remote_path>

# 递归目录
assh-fs push -r <server> <local_dir> <remote_dir>

# 断点续传
assh-fs push --resume <server> <file> <remote_path>
assh-fs push -e <server> <file> <remote_path>       # 短参

# 传输后 SHA256 校验
assh-fs push --checksum <server> <file> <remote_path>

# 覆盖策略
assh-fs push -f <server> <local> <remote>    # 强制覆盖
assh-fs push --skip <server> <local> <remote> # 跳过已存在

# Glob 匹配
assh-fs push <server> "*.log" <remote_dir>

# 直连
assh-fs push -H <host> -u <user> -P <pass> <local> <remote>
```

### pull 命令
```bash
assh-fs pull <server> <remote_path> [local_path]
assh-fs pull -r <server> <remote_dir> <local_dir>
```

### 交互式 FS 会话
```bash
assh-fs <server>    # 交互式 SFTP 会话
```

**支持命令集**（与 OpenSSH sftp 对齐）：

| 命令 | 用途 | 示例 |
|------|------|------|
| `ls [path]` | 列出远程目录 | `ls` / `ls /var/log` |
| `cd [path]` | 切换远程目录 | `cd /etc` |
| `pwd` | 显示远程当前路径 | `pwd` |
| `get <remote> [local]` | 下载文件 | `get file.txt ./local/` |
| `put <local> [remote]` | 上传文件 | `put ./file.txt /remote/` |
| `rm <path>` | 删除远程文件 | `rm old.log` |
| `mkdir <path>` | 创建远程目录 | `mkdir newdir` |
| `rmdir <path>` | 删除远程空目录 | `rmdir emptydir` |
| `chmod <mode> <path>` | 修改远程权限 | `chmod 644 file.txt` |
| `rename <old> <new>` | 重命名远程文件/目录 | `rename a.txt b.txt` |
| `ln [-s] <target> <link>` | 创建链接 | `ln -s /path/file link` |
| `df [-hi] [path]` | 显示远程磁盘空间 | `df -h /` |
| `umask [mask]` | 显示/设置 umask | `umask 022` |
| `lcd [path]` | 切换本地目录 | `lcd ~/Downloads` |
| `lpwd` | 显示本地当前路径 | `lpwd` |
| `lls [path]` | 列出本地目录 | `lls ./` |
| `!<cmd>` | 执行本地命令 | `!ls -la` |
| `help` | 显示帮助 | `help` |
| `quit` / `exit` | 退出交互模式 | `quit` |

## 架构设计

### 数据流
```
cmd/fs/main.go (assh-fs 入口：DI 组合根)
  └── cmd/sftp_cmd.go (push/pull/FS 命令定义)
        ↓ 参数解析 + 服务器查找
      service/transfer.go (TransferService)
        ↓ 传输用例编排（覆盖策略、进度管理）
      infra/sftp/ (FileTransfer 实现)
        ↓ pkg/sftp API
      ssh.Client → sftp.Client → 远程文件系统
```

### 关键设计

1. **SFTP 连接复用**：通过已建立的 `ssh.Client` 创建 `sftp.Client`（`sftp.NewClient(sshClient)`），无需额外认证
2. **递归传输**：遍历目录树，自动创建远程目录结构（`-r` 递归保持完整路径；Glob 匹配平铺到目标目录）
3. **Hash Header 安全验证（ADR-019）**：目标文件头部写入 256B hash header，用于续传前验证源文件完整性。
   - **格式**：`[Magic:4B][Version:2B][HashType:2B][OrigSize:8B][SHA256:64B][Reserved:176B]`
   - **写入时机**：首次传输时写入目标文件头部
   - **续传验证**：读取目标文件 header → 提取 hash → 与当前源文件 hash 比对
     | hash 比对结果 | 行为 |
     |---------------|------|
     | 一致 | 文件未修改，从 offset=256 续传 |
     | 不一致 | 文件被修改，询问用户或强制覆盖 |
     | header 不存在 | 回退到纯大小比对 |
   - **清理**：传输完成后重写文件去除 header（恢复原始格式）
   - **性能**：256B 开销对任意文件可忽略，重写时间 ≈ 本地读写时间
4. **断点续传（偏移量分块方案）**：
   - 上传：`sftp.Stat()` 获取远程大小 → 读取 header(256B) → 验证 hash → `localFile.Seek(offset)` → 续传
   - 下载：`os.Stat()` 获取本地大小 → 读取 header(256B) → 验证 hash → `remoteFile.Seek(offset)` → 续传
   - SFTP 客户端配置：`UseConcurrentReads(true)` + `UseConcurrentWrites(true)` + `MaxConcurrentRequestsPerFile(64)`
   - 边界处理（hash header 不存在时回退到纯大小比对）：
     | 场景 | 行为 |
     |------|------|
     | 远程 > 本地 | 报错"远程文件比本地大，续传不可用"，提示用 `-f` 覆盖 |
     | 远程 == 本地 | 跳过（信任大小比对，或 hash 验证通过） |
     | hash 不匹配 | 文件内容被修改，询问用户是否覆盖 |
4. **并发模型**：两层级并发
   - 单文件内部：由 pkg/sftp 内部控制，`MaxConcurrentRequestsPerFile(64)`
   - 多文件间：goroutine 池，大小由 `Concurrency` 控制（默认 3）
5. **进度显示格式**：实时更新，自动对齐长度
   ```
   ( 1/5) [project.tar.gz] .............. 58%  2.1MB/s  ETA 4s
   ( 2/5) [config.json] ................. 100%  5.3MB/s  ETA 0s
   ```
   - `(序号/总数)` — 定宽，右侧数字右对齐
   - `[文件名]` — 按最长文件名自动 padding，超长截断
   - `......` — 用点填充剩余空间，百分比例右对齐
   - `百分比 / 速率 / ETA` — 右侧对齐
6. **覆盖策略**：默认询问 → 用户输入 y/n/ya/na；`-f/--force` 强制覆盖；`--skip` 跳过
   - ya/na 仅本次会话有效
   - 单文件传输失败不影响后续文件继续传输
7. **远程路径检查**：push/pull 前检测目标路径是否存在，不存在时询问是否创建（push：远程目录；pull：本地目录）
8. **传输后验证**：
   - 大小比对（始终）：`sftp.Stat()` vs `os.Stat()` 比对文件大小
   - SHA256 校验（可选 `--checksum`/`-S`）：通过远程 SSH exec 执行 `sha256sum` 或本地计算比对
   - 验证失败：输出详细错误，保留文件不自动删除

### 覆盖策略
| 策略 | 参数 | 行为 | 作用域 |
|------|------|------|--------|
| 询问 | (默认) | 交互式确认 y/N/ya/na/a | 每次文件 |
| 强制覆盖 | `-f` / `--force` | 静默覆盖 | 本次命令 |
| 跳过 | `--skip` | 静默跳过 | 本次命令 |
| 全部是 | ya | 本次会话全部覆盖 | 会话内 |
| 全部否 | na | 本次会话全部跳过 | 会话内 |

## 依赖变更

`go.mod` 新增：
```
require github.com/pkg/sftp v1.13.7+
```

## DI 注入

SFTP 功能在独立的 `assh-fs` 二进制中，其 DI 组合根在 `cmd/fs/main.go`：

```go
// cmd/fs/main.go (assh-fs 入口)
sftpClient := sftpinfra.NewSFTPSession(sshClient)
transferSvc := service.NewTransferService(sftpClient, repo)
app := cmd.NewFSApp(Version, Build, transferSvc, serverSvc)
```

共享的初始化逻辑（DB/SSH/Service 创建）封装在 `cmd/bootstrap.go` 中，供 `main.go` 和 `cmd/fs/main.go` 共同调用。

## 测试策略

| 层级 | 方法 | 覆盖范围 |
|------|------|----------|
| Service | mock FileTransfer | Push/Pull/List/Remove + 覆盖策略 |
| Infra | local SFTP test server | 文件上传下载/目录递归/断点续传 |
| 集成 | 真实 SSH + SFTP | 全链路端到端测试 |