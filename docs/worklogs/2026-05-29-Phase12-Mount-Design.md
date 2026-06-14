# Phase 12: 远程服务器挂载 — 设计文档

> 创建日期: 2026-05-29
> 状态: 设计完成，待实现
> 版本: v2.1.0

---

## 一、概述

Phase 12 实现远程服务器文件系统挂载到本地，支持 sshfs 和 WebDAV 两种方案。挂载命令归属 `assh-fs` 二进制，与现有 SFTP 功能（push/pull/交互）统一管理。

### 设计目标

- 基于现有 SSH 基础设施，无需额外服务端配置（sshfs）
- WebDAV 通过 Go 内置服务端 + SSH 隧道实现，客户端原生挂载
- 挂载信息持久化到 SQLite，支持查询和未来自动恢复
- 支持多服务器并发挂载
- 自动检测依赖并提示安装命令

---

## 二、决策记录

| 决策项 | 结论 | 理由 |
|--------|------|------|
| 挂载方案 | sshfs + WebDAV | sshfs 基于现有 SSH；WebDAV 作为补充方案 |
| SMB/CIFS | 延后 | 需要 Samba 服务端配置，使用场景相对较少 |
| 挂载持久化 | SQLite `mounts` 表 | 支持 mount list 查询和未来自动挂载 |
| 依赖检测 | 自动检测 + 提示 | 降低用户使用门槛 |
| WebDAV 实现 | Go 内置服务端 + SSH 隧道 | 无需远程安装额外服务 |
| 自动重连 | sshfs 默认开启 reconnect | 网络波动时自动恢复 |
| 挂载点路径 | 必须手动指定 | 避免路径歧义 |
| 并发挂载 | 支持多服务器 | 实际使用场景需求 |
| WebDAV 端口 | 自动选择可用端口 | 多服务器挂载时避免冲突 |
| 进程生命周期 | 前台 + --daemon 可选 | 灵活适配不同场景 |
| 命令归属 | `assh-fs` | 文件系统操作统一入口 |

---

## 三、命令设计

### 3.1 命令结构

```
assh-fs
├── push           # 文件上传 (已有)
├── pull           # 文件下载 (已有)
├── sftp           # 交互式 SFTP (已有)
├── mount          # 挂载命令组 (新增)
│   ├── <server> <remote> <local>   # 挂载（默认子命令）
│   ├── list                        # 列出挂载
│   └── check                       # 检查依赖
└── umount         # 卸载命令 (新增)
```

### 3.2 挂载命令

```bash
# 基本挂载（默认 sshfs）
assh-fs mount <server> <remote-path> <local-path>

# 显式指定挂载类型
assh-fs mount <server> <remote-path> <local-path> --type sshfs
assh-fs mount <server> <remote-path> <local-path> --type webdav

# 挂载选项
assh-fs mount <server> /data ~/mnt -o readonly          # 只读
assh-fs mount <server> /data ~/mnt -o allow_other       # 允许其他用户
assh-fs mount <server> /data ~/mnt -o volname="MyData"  # 卷名称（Finder 显示）
assh-fs mount <server> /data ~/mnt -o "port=2222"       # 额外参数

# 进程模式
assh-fs mount <server> /data ~/mnt                      # 前台运行（Ctrl+C 自动卸载）
assh-fs mount <server> /data ~/mnt --daemon             # 后台模式

# 直连模式
assh-fs mount -H 192.168.1.1 -u root -P pass /data ~/mnt

# 跳板机支持
assh-fs mount -J gateway myserver /data ~/mnt
```

### 3.3 卸载命令

```bash
# 按挂载点卸载
assh-fs umount <local-path>

# 按服务器名卸载（卸载该服务器所有挂载）
assh-fs umount --server <server>

# 卸载所有
assh-fs umount --all
```

### 3.4 查询命令

```bash
# 列出所有挂载
assh-fs mount list

# 按服务器过滤
assh-fs mount list --server <server>

# 检查依赖
assh-fs mount check
assh-fs mount check --type sshfs
assh-fs mount check --type webdav
```

### 3.5 输出格式

#### mount list 输出

```
MOUNTS:
  SERVER        REMOTE          LOCAL                TYPE     OPTIONS
  myserver      /var/data       ~/mnt/data           sshfs    ro,reconnect
  prod.db       /backup         ~/mnt/backup         webdav   ro
  dev.api       /app/logs       ~/mnt/logs           sshfs    reconnect
```

#### mount check 输出

```
DEPENDENCY CHECK:
  sshfs:
    ✓ sshfs command found (/usr/local/bin/sshfs)
    ✓ macFUSE installed (4.6.0)
  
  webdav:
    ✓ mount_webdav available (macOS native)
  
  Notes:
    - WebDAV uses Go built-in server, no remote dependencies
```

或依赖缺失时：

```
DEPENDENCY CHECK:
  sshfs:
    ✗ sshfs command not found
      Install: brew install sshfs
    ✗ macFUSE not installed
      Install: brew install --cask macfuse
      Note: Requires reboot after installation
  
  webdav:
    ✓ mount_webdav available (macOS native)
```

---

## 四、技术架构

### 4.1 分层结构

```
cmd/fs/main.go              # assh-fs 入口 + DI 组合根
cmd/sftp_cmd.go             # mount/umount 命令定义
│
├── fsMountAction()         # mount 命令处理
├── fsUmountAction()        # umount 命令处理
├── fsMountListAction()     # mount list 处理
└── fsMountCheckAction()    # mount check 处理

asshc/port/mounter.go       # FileMounter/MountManager 接口
│
asshc/infra/mount/
├── sshfs.go                # SSHFS 实现
├── webdav.go               # WebDAV 实现（Go 服务端 + 隧道）
├── detect.go               # 平台检测 + 依赖检查
└── port.go                 # 端口管理（自动选择可用端口）
│
asshc/infra/store/mount.go  # mounts 表 CRUD
│
asshc/service/mount.go      # MountService 编排层
```

### 4.2 接口设计

```go
// port/mounter.go

// MountType 挂载类型枚举
type MountType string
const (
    MountTypeSSHFS  MountType = "sshfs"
    MountTypeWebDAV MountType = "webdav"
)

// MountOptions 挂载选项
type MountOptions struct {
    MountType    MountType         // 挂载类型
    MountPoint   string            // 本地挂载点（必填）
    ReadOnly     bool              // 只读模式
    AllowOther   bool              // 允许其他用户访问
    VolumeName   string            // 卷名称（Finder 显示）
    Daemon       bool              // 后台模式
    ExtraOpts    map[string]string // 额外挂载参数
}

// MountInfo 挂载信息
type MountInfo struct {
    ID          int64         // 数据库 ID
    ServerName  string        // 服务器名称
    RemotePath  string        // 远程路径
    MountPoint  string        // 本地挂载点
    MountType   MountType     // 挂载类型
    LocalPort   int           // WebDAV 本地端口（仅 WebDAV）
    Daemon      bool          // 是否后台模式
    PID         int           // 后台进程 PID（仅 daemon）
    MountedAt   time.Time     // 挂载时间
}

// FileMounter 单类型挂载器接口
type FileMounter interface {
    // Mount 挂载远程文件系统到本地
    Mount(server *domain.Server, remotePath string, opts MountOptions) error
    
    // Unmount 卸载已挂载的文件系统
    Unmount(mountPoint string) error
    
    // IsAvailable 检查依赖是否满足
    IsAvailable() bool
    
    // Name 返回挂载器名称
    Name() MountType
}

// MountManager 挂载管理器接口
type MountManager interface {
    // Mount 挂载远程文件系统
    Mount(server *domain.Server, remotePath string, opts MountOptions) error
    
    // Unmount 卸载指定挂载点
    Unmount(mountPoint string) error
    
    // UnmountByServer 卸载指定服务器的所有挂载
    UnmountByServer(serverName string) error
    
    // UnmountAll 卸载所有挂载
    UnmountAll() error
    
    // List 列出所有挂载
    List() ([]MountInfo, error)
    
    // GetMounter 获取指定类型的挂载器
    GetMounter(mountType MountType) (FileMounter, error)
    
    // CheckDependencies 检查依赖状态
    CheckDependencies() map[MountType]DependencyStatus
}

// DependencyStatus 依赖状态
type DependencyStatus struct {
    Available bool
    Details   []DependencyItem
}

// DependencyItem 单个依赖项
type DependencyItem struct {
    Name     string // 依赖名称
    Found    bool   // 是否找到
    Path     string // 路径（如果找到）
    Version  string // 版本（如果找到）
    Install  string // 安装命令（如果未找到）
}
```

### 4.3 数据库设计

```sql
-- mounts 表：挂载信息持久化
CREATE TABLE IF NOT EXISTS mounts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name   TEXT NOT NULL,
    remote_path   TEXT NOT NULL,
    mount_point   TEXT NOT NULL UNIQUE,
    mount_type    TEXT NOT NULL CHECK(mount_type IN ('sshfs', 'webdav')),
    local_port    INTEGER DEFAULT 0,        -- WebDAV 本地端口
    daemon        INTEGER DEFAULT 0,        -- 是否后台模式
    pid           INTEGER DEFAULT 0,        -- 后台进程 PID
    options       TEXT DEFAULT '{}',        -- JSON 格式挂载选项
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_mounts_server ON mounts(server_name);
CREATE INDEX IF NOT EXISTS idx_mounts_point ON mounts(mount_point);
```

### 4.4 DI 注入变更

```go
// cmd/fs/main.go — assh-fs 组合根
func main() {
    // ... 现有初始化 ...
    
    // 创建挂载管理器
    mountMgr := mount.NewManager()
    
    // 注入到 NewFSApp
    app := cmd.NewFSApp(version, build, transferSvc, serverSvc, 
        knownRecorder, connectSvc, jumpSvc, mountMgr)
}
```

```go
// cmd/app.go — NewFSApp 签名变更
func NewFSApp(
    version, build string,
    transferSvc *service.TransferService,
    serverSvc *service.ServerService,
    knownRecorder transferport.KnownServerRecorder,
    connectSvc *service.ConnectService,
    jumpSvc *service.JumpService,
    mountMgr transferport.MountManager,  // 新增
) *App {
    // ...
    a.registerFSCommands(transferSvc, serverSvc, mountMgr)
}
```

---

## 五、sshfs 实现细节

### 5.1 平台差异

| 项目 | MacOS | Linux |
|------|-------|-------|
| 前置依赖 | macFUSE + sshfs | FUSE + sshfs |
| 安装命令 | `brew install --cask macfuse && brew install sshfs` | `sudo apt install sshfs` |
| 卸载命令 | `umount <path>` 或 `diskutil unmount <path>` | `fusermount -u <path>` |
| FUSE 配置 | 无需额外配置 | `/etc/fuse.conf` 需 `user_allow_other` |

### 5.2 sshfs 命令构建

```bash
# 基本命令
sshfs <user>@<host>:<remote-path> <local-path> \
    -p <port> \
    -o IdentityFile=<key-file> \
    -o reconnect \
    -o ServerAliveInterval=15 \
    -o StrictHostKeyChecking=accept-new

# 只读模式
-o ro

# 允许其他用户
-o allow_other

# 卷名称（MacOS Finder 显示）
-o volname=<name>

# 密码认证（通过 SSH_ASKPASS 或 expect）
-o password_stdin
```

### 5.3 密码认证处理

sshfs 不直接支持密码参数，需要通过以下方式：

```go
// 方案1: SSH_ASKPASS 环境变量
os.Setenv("SSH_ASKPASS", "/path/to/askpass-helper")
os.Setenv("DISPLAY", ":0")  // SSH_ASKPASS 需要 DISPLAY

// 方案2: expect 风格交互
// 使用 Go expect 库或 syscall 控制 stdin

// 方案3: 优先使用密钥认证
// 密码认证时提示用户手动输入
```

### 5.4 自动重连

```bash
# 默认启用的重连参数
-o reconnect
-o ServerAliveInterval=15    # 15 秒心跳
-o ServerAliveCountMax=3     # 3 次失败后断开
-o ConnectTimeout=10         # 连接超时 10 秒
```

---

## 六、WebDAV 实现细节

### 6.1 架构设计

```
┌─────────────────────────────────────────────────────────┐
│  本地客户端                                              │
│                                                         │
│  ┌──────────────┐    ┌──────────────┐    ┌────────────┐│
│  │ mount_webdav │───▶│ localhost:    │───▶│ Go WebDAV  ││
│  │ / davfs2     │    │ <auto-port>  │    │ Server     ││
│  └──────────────┘    └──────────────┘    └─────┬──────┘│
│                                                │       │
└────────────────────────────────────────────────┼───────┘
                                                 │ SSH 隧道
┌────────────────────────────────────────────────┼───────┐
│  远程服务器                                      │       │
│                                                ▼       │
│  ┌──────────────────────────────────────────────────┐ │
│  │ SSH Server (sshd)                                │ │
│  │   └── SFTP Subsystem                             │ │
│  │       └── 文件系统访问                            │ │
│  └──────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────┘
```

### 6.2 实现流程

```
1. 建立 SSH 连接
2. 创建 SSH 隧道：远程 SFTP ↔ 本地随机端口
3. 启动 Go WebDAV 服务端，后端对接 SFTP
4. 调用系统 mount_webdav/davfs2 挂载本地 WebDAV 端口
5. 前台等待或后台守护
```

### 6.3 Go WebDAV 服务端

```go
// 使用 golang.org/x/net/webdav 实现
type WebDAVServer struct {
    sftpClient *sftp.Client
    listener   net.Listener
    server     *webdav.Handler
}

func NewWebDAVServer(sftpClient *sftp.Client, port int) (*WebDAVServer, error) {
    fs := &SFTPFileSystem{client: sftpClient}
    
    handler := &webdav.Handler{
        FileSystem: fs,
        LockSystem: webdav.NewMemLS(),
    }
    
    listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
    if err != nil {
        return nil, err
    }
    
    return &WebDAVServer{
        sftpClient: sftpClient,
        listener:   listener,
        server:     handler,
    }, nil
}

// SFTPFileSystem 将 SFTP 适配为 webdav.FileSystem
type SFTPFileSystem struct {
    client *sftp.Client
}

func (fs *SFTPFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
    // 实现 SFTP -> WebDAV 文件操作映射
}
```

### 6.4 端口管理

```go
// 自动选择可用端口
func FindAvailablePort() (int, error) {
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return 0, err
    }
    port := listener.Addr().(*net.TCPAddr).Port
    listener.Close()
    return port, nil
}

// 端口范围：18000-18999（避免与常用端口冲突）
const WebDAVPortMin = 18000
const WebDAVPortMax = 18999
```

### 6.5 平台差异

| 项目 | MacOS | Linux |
|------|-------|-------|
| 挂载命令 | `mount_webdav -S http://127.0.0.1:<port> <path>` | `mount -t davfs http://127.0.0.1:<port> <path>` |
| 卸载命令 | `umount <path>` | `umount <path>` |
| SSL 处理 | `-S` 跳过 SSL 验证 | 配置 `/etc/davfs2/davfs2.conf` |
| 凭证 | 交互式输入或 `-i` 参数 | `/etc/davfs2/secrets` 文件 |

---

## 七、依赖检测

### 7.1 sshfs 依赖检测

```go
type SSHFSDetector struct{}

func (d *SSHFSDetector) Detect() DependencyStatus {
    status := DependencyStatus{}
    
    // 1. 检测 sshfs 命令
    sshfsPath, err := exec.LookPath("sshfs")
    status.Details = append(status.Details, DependencyItem{
        Name:    "sshfs",
        Found:   err == nil,
        Path:    sshfsPath,
        Install: getSSHFSInstallCmd(),
    })
    
    // 2. 检测 macFUSE（仅 MacOS）
    if runtime.GOOS == "darwin" {
        macfuseInstalled := checkMacFUSE()
        status.Details = append(status.Details, DependencyItem{
            Name:    "macFUSE",
            Found:   macfuseInstalled,
            Install: "brew install --cask macfuse",
        })
    }
    
    status.Available = allFound(status.Details)
    return status
}

func getSSHFSInstallCmd() string {
    switch runtime.GOOS {
    case "darwin":
        return "brew install sshfs"
    case "linux":
        return "sudo apt install sshfs  # or: sudo yum install sshfs"
    default:
        return "install sshfs for your platform"
    }
}

func checkMacFUSE() bool {
    // 检查 /Library/Filesystems/macfuse.fs 是否存在
    _, err := os.Stat("/Library/Filesystems/macfuse.fs")
    return err == nil
}
```

### 7.2 WebDAV 依赖检测

```go
type WebDAVDetector struct{}

func (d *WebDAVDetector) Detect() DependencyStatus {
    status := DependencyStatus{}
    
    switch runtime.GOOS {
    case "darwin":
        // MacOS 原生支持 mount_webdav
        _, err := exec.LookPath("mount_webdav")
        status.Details = append(status.Details, DependencyItem{
            Name:  "mount_webdav",
            Found: err == nil,
        })
    case "linux":
        // Linux 需要 davfs2
        _, err := exec.LookPath("mount.davfs")
        status.Details = append(status.Details, DependencyItem{
            Name:    "davfs2",
            Found:   err == nil,
            Install: "sudo apt install davfs2",
        })
    }
    
    status.Available = allFound(status.Details)
    return status
}
```

---

## 八、服务层设计

### 8.1 MountService

```go
// service/mount.go
type MountService struct {
    manager  port.MountManager
    repo     port.ServerRepository
    store    port.MountStore
}

func NewMountService(manager port.MountManager, repo port.ServerRepository, store port.MountStore) *MountService {
    return &MountService{
        manager: manager,
        repo:    repo,
        store:   store,
    }
}

func (s *MountService) Mount(serverName, remotePath, mountType string, opts port.MountOptions) error {
    // 1. 获取服务器配置
    server, err := s.repo.Get(serverName)
    if err != nil {
        return fmt.Errorf("server %q not found: %w", serverName, err)
    }
    
    // 2. 设置挂载类型
    opts.MountType = port.MountType(mountType)
    
    // 3. 执行挂载
    if err := s.manager.Mount(server, remotePath, opts); err != nil {
        return err
    }
    
    // 4. 持久化到数据库
    info := port.MountInfo{
        ServerName: serverName,
        RemotePath: remotePath,
        MountPoint: opts.MountPoint,
        MountType:  opts.MountType,
        Daemon:     opts.Daemon,
        MountedAt:  time.Now(),
    }
    return s.store.Save(info)
}

func (s *MountService) Unmount(mountPoint string) error {
    // 1. 执行卸载
    if err := s.manager.Unmount(mountPoint); err != nil {
        return err
    }
    
    // 2. 从数据库删除
    return s.store.DeleteByMountPoint(mountPoint)
}

func (s *MountService) List() ([]port.MountInfo, error) {
    return s.store.List()
}

func (s *MountService) CheckDependencies() map[port.MountType]port.DependencyStatus {
    return s.manager.CheckDependencies()
}
```

---

## 九、验收场景

| # | 场景 | 操作 | 预期 |
|---|------|------|------|
| F1 | sshfs 挂载 | `assh-fs mount myserver /data ~/mnt` | 挂载成功，文件可见 |
| F2 | sshfs 卸载 | `assh-fs umount ~/mnt` | 卸载成功 |
| F3 | WebDAV 挂载 | `assh-fs mount myserver /data ~/mnt --type webdav` | 挂载成功 |
| F4 | 挂载列表 | `assh-fs mount list` | 显示当前挂载 |
| F5 | 只读挂载 | `assh-fs mount myserver /data ~/mnt -o readonly` | 写入失败 |
| F6 | 直连挂载 | `assh-fs mount -H host -u root /data ~/mnt` | 直连成功 |
| F7 | 跳板挂载 | `assh-fs mount -J gw myserver /data ~/mnt` | 通过跳板挂载 |
| F8 | 依赖检查 | `assh-fs mount check` | 显示依赖状态 |
| F9 | 服务器不存在 | `assh-fs mount bad /data ~/mnt` | 报错提示 |
| F10 | 重复挂载 | 同一挂载点再次挂载 | 报错已挂载 |
| F11 | 按服务器卸载 | `assh-fs umount --server myserver` | 卸载成功 |
| F12 | 全部卸载 | `assh-fs umount --all` | 全部卸载 |
| F13 | daemon 模式 | `assh-fs mount myserver /data ~/mnt --daemon` | 后台运行 |
| F14 | 自动重连 | 网络断开后恢复 | sshfs 自动重连 |
| F15 | 多服务器挂载 | 挂载 2 个不同服务器 | 互不影响 |

---

## 十、任务清单

| 编号 | 任务 | 文件 | 说明 | 优先级 |
|------|------|------|------|--------|
| P12.1 | 挂载接口定义 | `asshc/port/mounter.go` | FileMounter/MountManager/MountOptions/MountInfo | P0 |
| P12.2 | 数据库迁移 | `asshc/infra/store/db.go` | mounts 表建表 | P0 |
| P12.3 | 挂载存储 | `asshc/infra/store/mount.go` | CRUD 持久化 | P0 |
| P12.4 | sshfs 实现 | `asshc/infra/mount/sshfs.go` | SSHFS 挂载/卸载/重连 | P0 |
| P12.5 | WebDAV 实现 | `asshc/infra/mount/webdav.go` | Go WebDAV 服务 + SSH 隧道 + 客户端挂载 | P1 |
| P12.6 | 依赖检测 | `asshc/infra/mount/detect.go` | 平台检测 + 依赖检查 | P0 |
| P12.7 | 挂载服务 | `asshc/service/mount.go` | MountService 编排层 | P0 |
| P12.8 | CLI 命令 | `cmd/sftp_cmd.go` | mount/umount 命令定义 | P0 |
| P12.9 | DI 注入 | `cmd/fs/main.go` + `cmd/app.go` | MountManager 注入 | P0 |
| P12.10 | WebDAV 端口管理 | `asshc/infra/mount/port.go` | 自动端口选择 + 冲突避免 | P1 |

---

## 十一、依赖顺序

```
P12.1 (接口)
  └── P12.2 (数据库迁移)
        └── P12.3 (挂载存储)
  └── P12.6 (依赖检测)
        └── P12.4 (sshfs 实现)
        └── P12.5 (WebDAV 实现) + P12.10 (端口管理)
              └── P12.7 (挂载服务)
                    └── P12.8 (CLI 命令)
                          └── P12.9 (DI 注入)
```

---

## 十二、验证命令

```bash
# 编译
cd asshv2 && go build ./...

# 静态检查
cd asshv2 && go vet ./...

# 测试
cd asshv2 && go test ./... -v

# 功能验证
assh-fs mount check
assh-fs mount list
assh-fs mount myserver /tmp /mnt/test
assh-fs umount /mnt/test
```

---

## 十三、未来扩展

- **SMB/CIFS 支持**: 延后到后续迭代
- **自动挂载**: 基于 mounts 表记录，启动时自动恢复挂载
- **挂载配置导出/导入**: 结合云同步功能
- **性能优化**: sshfs 缓存参数调优
