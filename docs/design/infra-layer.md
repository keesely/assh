# 基础设施层设计 (Infrastructure Layer)

## 1. 概述

基础设施层实现了端口层定义的接口，负责与外部系统交互（数据库、SSH、云服务等）。该层是应用与外部世界的边界。

### 1.1 设计原则

- **接口适配**：实现端口层定义的接口
- **技术隔离**：封装具体技术实现细节
- **可替换性**：同一接口可有多种实现

## 2. 实现分类

```
┌─────────────────────────────────────────────────────────────┐
│                   基础设施层结构                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  infra/                                                      │
│  ├── store/          # 数据持久化                            │
│  │   ├── db.go       #   SQLite 连接管理                    │
│  │   ├── server.go   #   服务器配置存储                      │
│  │   ├── account.go  #   云账户存储                          │
│  │   ├── jump.go     #   跳板历史存储                        │
│  │   └── mount.go    #   挂载配置存储                        │
│  │                                                           │
│  ├── ssh/            # SSH 协议实现                          │
│  │   ├── client.go   #   SSH 客户端                          │
│  │   ├── session.go  #   SSH 会话                            │
│  │   └── health.go   #   健康检查                            │
│  │                                                           │
│  ├── sftp/           # SFTP 协议实现                         │
│  │   ├── client.go   #   SFTP 客户端                        │
│  │   ├── upload.go   #   文件上传                            │
│  │   └── download.go #   文件下载                            │
│  │                                                           │
│  ├── crypto/         # 加解密实现                            │
│  │   ├── rsa.go      #   RSA 加解密                          │
│  │   └── aes.go      #   AES-GCM 加解密                     │
│  │                                                           │
│  ├── proxy/          # 代理服务实现                          │
│  │   ├── socks5.go   #   SOCKS5 代理                        │
│  │   ├── http.go     #   HTTP 代理                          │
│  │   └── tunnel.go   #   隧道管理                            │
│  │                                                           │
│  ├── sync/           # 云同步实现                            │
│  │   └── qiniu/      #   七牛云实现                          │
│  │                                                           │
│  ├── mount/          # 文件系统挂载实现                      │
│  │   ├── sshfs.go    #   SSHFS 挂载                          │
│  │   └── webdav.go   #   WebDAV 挂载                        │
│  │                                                           │
│  └── updater/        # 系统更新实现                          │
│      ├── github.go   #   GitHub 版本检查                    │
│      └── download.go #   文件下载                            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## 3. 存储实现 (Store)

### 3.1 SQLiteStore

**位置**: `asshc/infra/store/`

SQLite 存储实现，是项目最核心的基础设施组件。

**职责**:
- 管理 SQLite 数据库连接
- 实现 `ServerRepository` 接口
- 实现 `KnownServerRecorder` 接口
- 数据加密存储（AES-GCM）

**核心结构**:

```go
type SQLiteStore struct {
    db      *sql.DB
    cryptor port.Cryptor
}

func NewSQLiteStore(dbPath string, cryptor port.Cryptor) (*SQLiteStore, error) {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, err
    }
    
    store := &SQLiteStore{db: db, cryptor: cryptor}
    if err := store.migrate(); err != nil {
        return nil, err
    }
    
    return store, nil
}
```

**数据库表结构**:

```sql
-- 服务器配置表
CREATE TABLE servers (
    name TEXT PRIMARY KEY,
    group_name TEXT,
    host TEXT NOT NULL,
    port INTEGER DEFAULT 22,
    user TEXT DEFAULT 'root',
    auth_password TEXT,  -- AES-GCM 加密
    auth_keyfile TEXT,
    remark TEXT,
    options TEXT,        -- JSON
    version INTEGER DEFAULT 1
);

-- 变更日志表
CREATE TABLE server_changelog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    version INTEGER NOT NULL,
    change_type TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    snapshot TEXT,       -- JSON, AES-GCM 加密
    FOREIGN KEY (server_name) REFERENCES servers(name)
);

-- 已知服务器表
CREATE TABLE known_servers (
    id TEXT PRIMARY KEY,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    user TEXT NOT NULL,
    auth_fingerprint TEXT,
    key_backup_path TEXT,
    connect_count INTEGER DEFAULT 1,
    last_connected_at DATETIME
);

-- 云账户表
CREATE TABLE cloud_accounts (
    name TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    credentials TEXT,    -- AES-GCM 加密
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**加密策略**:
- 敏感字段（密码、凭据）使用 AES-GCM 加密
- 加密密钥通过 `Cryptor` 接口提供
- 存储时加密，读取时解密

### 3.2 AccountStore

**位置**: `asshc/infra/store/account.go`

云账户存储实现。

```go
type AccountStore struct {
    db      *sql.DB
    cryptor port.Cryptor
}
```

**职责**:
- 云账户凭据的 CRUD
- 凭据加密存储

### 3.3 MountStore

**位置**: `asshc/infra/store/mount.go`

挂载配置存储实现。

```go
type MountStore struct {
    db *sql.DB
}
```

**职责**:
- 挂载点配置的持久化
- 挂载状态管理

## 4. SSH 实现

### 4.1 SSHClient

**位置**: `asshc/infra/ssh/client.go`

SSH 连接实现。

```go
type SSHClient struct {
    hostKeyChecker port.HostKeyChecker
}

func NewSSHClient(hostKeyChecker port.HostKeyChecker) *SSHClient {
    return &SSHClient{hostKeyChecker: hostKeyChecker}
}
```

**连接流程**:

```
┌─────────────────────────────────────────────────────────────┐
│                    SSH 连接流程                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. 构建 ssh.ClientConfig                                   │
│     │                                                        │
│     ├── 认证方式                                            │
│     │   ├── 密钥认证 (KeyFile)                              │
│     │   └── 密码认证 (Password)                             │
│     │                                                        │
│     ├── HostKey 校验                                        │
│     │   └── known_hosts 文件检查                            │
│     │                                                        │
│     └── 超时设置                                            │
│                                                              │
│  2. 建立 TCP 连接                                           │
│                                                              │
│  3. SSH 握手                                                │
│                                                              │
│  4. 返回 ssh.Client                                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**认证回退**:
```go
func (c *SSHClient) Connect(server *domain.Server) (*ssh.Client, error) {
    // 1. 尝试密钥认证
    if server.Auth != nil && server.Auth.KeyFile != "" {
        client, err := c.connectWithKey(server)
        if err == nil {
            return client, nil
        }
    }
    
    // 2. 尝试密码认证
    if server.Auth != nil && server.Auth.Password != "" {
        client, err := c.connectWithPassword(server)
        if err == nil {
            return client, nil
        }
    }
    
    return nil, errors.New("all authentication methods failed")
}
```

### 4.2 SSHSession

**位置**: `asshc/infra/ssh/session.go`

SSH 会话实现。

```go
type SSHSessionImpl struct{}

func NewSSHSession() *SSHSessionImpl {
    return &SSHSessionImpl{}
}
```

**主要方法**:

```go
func (s *SSHSessionImpl) Shell(client *ssh.Client) error {
    session, err := client.NewSession()
    if err != nil {
        return err
    }
    defer session.Close()
    
    // 连接标准输入输出
    session.Stdin = os.Stdin
    session.Stdout = os.Stdout
    session.Stderr = os.Stderr
    
    // 请求 PTY
    modes := ssh.TerminalModes{
        ssh.ECHO: 1,
    }
    if err := session.RequestPty("xterm", 80, 24, modes); err != nil {
        return err
    }
    
    return session.Shell()
}

func (s *SSHSessionImpl) Run(client *ssh.Client, cmd string) ([]byte, []byte, error) {
    session, err := client.NewSession()
    if err != nil {
        return nil, nil, err
    }
    defer session.Close()
    
    var stdout, stderr bytes.Buffer
    session.Stdout = &stdout
    session.Stderr = &stderr
    
    err = session.Run(cmd)
    return stdout.Bytes(), stderr.Bytes(), err
}
```

### 4.3 HealthChecker

**位置**: `asshc/infra/ssh/health.go`

SSH 健康检查实现。

```go
type SSHHealthChecker struct{}

func NewSSHHealthChecker() *SSHHealthChecker {
    return &SSHHealthChecker{}
}
```

**检查项目**:
- 连接延迟
- 系统信息（hostname、uptime、load）
- 磁盘使用率
- 内存使用率

## 5. SFTP 实现

### 5.1 SFTPTransfer

**位置**: `asshc/infra/sftp/`

SFTP 文件传输实现。

```go
type SFTPTransfer struct{}

func NewSFTPTransfer() *SFTPTransfer {
    return &SFTPTransfer{}
}
```

**文件传输流程**:

```
┌─────────────────────────────────────────────────────────────┐
│                    文件上传流程                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. 创建 SFTP 客户端                                        │
│                                                              │
│  2. 打开远程文件                                            │
│                                                              │
│  3. 打开本地文件                                            │
│                                                              │
│  4. 复制文件内容                                            │
│     │                                                        │
│     └── 显示进度条                                          │
│                                                              │
│  5. 校验文件完整性                                          │
│                                                              │
│  6. 关闭连接                                                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**进度显示**:
```go
type ProgressWriter struct {
    total   int64
    current int64
    onProgress func(current, total int64)
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
    n := len(p)
    pw.current += int64(n)
    if pw.onProgress != nil {
        pw.onProgress(pw.current, pw.total)
    }
    return n, nil
}
```

## 6. 加密实现 (Crypto)

### 6.1 RSACryptor

**位置**: `asshc/infra/crypto/rsa.go`

RSA 加解密实现。

```go
type RSACryptor struct {
    privateKey *rsa.PrivateKey
    publicKey  *rsa.PublicKey
}

func NewRSACryptor(privateKeyPath string) (*RSACryptor, error) {
    // 加载私钥
    // 提取公钥
    return &RSACryptor{privateKey: privateKey, publicKey: publicKey}, nil
}
```

### 6.2 AESCryptor

**位置**: `asshc/infra/crypto/aes.go`

AES-GCM 加解密实现。

```go
type AESCryptor struct {
    key []byte
}

func NewAESCryptor(key []byte) *AESCryptor {
    return &AESCryptor{key: key}
}

func (c *AESCryptor) Encrypt(plaintext []byte) ([]byte, error) {
    block, err := aes.NewCipher(c.key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return nil, err
    }
    
    return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (c *AESCryptor) Decrypt(ciphertext []byte) ([]byte, error) {
    block, err := aes.NewCipher(c.key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, errors.New("ciphertext too short")
    }
    
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}
```

## 7. 代理实现 (Proxy)

### 7.1 SOCKS5Proxy

**位置**: `asshc/infra/proxy/socks5.go`

SOCKS5 代理实现，遵循 RFC 1928。

```go
type SOCKS5Proxy struct {
    listener net.Listener
    tunnel   port.TunnelManager
}

func NewSOCKS5Proxy(tunnel port.TunnelManager) *SOCKS5Proxy {
    return &SOCKS5Proxy{tunnel: tunnel}
}
```

**支持的认证方式**:
- NO AUTHENTICATION (0x00)
- USERNAME/PASSWORD (0x02) - RFC 1929

### 7.2 HTTPProxy

**位置**: `asshc/infra/proxy/http_connect.go`

HTTP CONNECT 代理实现，遵循 RFC 7231。

```go
type HTTPProxy struct {
    listener net.Listener
    tunnel   port.TunnelManager
}
```

### 7.3 TunnelManager

**位置**: `asshc/infra/proxy/tunnel_manager.go`

SSH 隧道管理器实现。

```go
type TunnelManagerImpl struct {
    tunnels map[string]*Tunnel
    mu      sync.RWMutex
}

func NewTunnelManager() *TunnelManagerImpl {
    return &TunnelManagerImpl{
        tunnels: make(map[string]*Tunnel),
    }
}
```

**隧道类型**:
- Local Forward (本地端口转发)
- Remote Forward (远程端口转发)
- Dynamic Forward (动态端口转发 - SOCKS5)

## 8. 云同步实现 (Sync)

### 8.1 QiniuSyncer

**位置**: `asshc/infra/sync/qiniu/qiniu.go`

七牛云同步实现。

```go
type QiniuSyncer struct {
    mac *qbox.Mac
    cfg *storage.Config
}

func NewQiniuSyncer(accessKey, secretKey string) *QiniuSyncer {
    mac := qbox.NewMac(accessKey, secretKey)
    cfg := storage.NewConfig()
    return &QiniuSyncer{mac: mac, cfg: cfg}
}
```

**主要方法**:
- `Login()` - 测试连接
- `Push(data)` - 上传数据
- `Pull()` - 下载数据
- `List()` - 列出文件
- `Delete(key)` - 删除文件

## 9. 挂载实现 (Mount)

### 9.1 SSHFSMounter

**位置**: `asshc/infra/mount/sshfs.go`

SSHFS 挂载实现。

```go
type SSHFSMounter struct{}

func NewSSHFSMounter() *SSHFSMounter {
    return &SSHFSMounter{}
}
```

**挂载流程**:
1. 检查 sshfs 依赖
2. 创建本地挂载点
3. 执行 sshfs 命令
4. 验证挂载成功

### 9.2 WebDAVMounter

**位置**: `asshc/infra/mount/webdav.go`

WebDAV 挂载实现。

```go
type WebDAVMounter struct {
    portManager *WebDAVPortManager
}

func NewWebDAVMounter() *WebDAVMounter {
    return &WebDAVMounter{
        portManager: NewWebDAVPortManager(),
    }
}
```

**挂载流程**:
1. 启动 WebDAV 服务器（通过 SSH 隧道）
2. 挂载 WebDAV 到本地
3. 管理端口分配

## 10. 依赖注入

### 10.1 组合根 (Composition Root)

**位置**: `main.go` 和 `cmd/fs/main.go`

```go
// main.go - assh 组合根
func main() {
    // 1. 创建加密器
    cryptor, _ := crypto.NewAESCryptor(getEncryptionKey())
    
    // 2. 创建存储
    store, _ := store.NewSQLiteStore(getDBPath(), cryptor)
    defer store.Close()
    
    // 3. 创建 SSH 组件
    hostKeyChecker := ssh.NewHostKeyChecker()
    sshConnector := ssh.NewSSHClient(hostKeyChecker)
    sshSession := ssh.NewSSHSession()
    
    // 4. 创建服务
    serverService := service.NewServerService(store)
    connectService := service.NewConnectService(sshConnector, sshSession)
    
    // 5. 创建 CLI 应用
    app := cmd.NewApp(serverService, connectService)
    app.Run(os.Args)
}
```

### 10.2 共享初始化

**位置**: `cmd/bootstrap.go`

```go
// AppComponents 包含共享的应用组件
type AppComponents struct {
    Store          *store.SQLiteStore
    SSHConnector   port.SSHConnector
    ServerService  *service.ServerService
    // ... 其他组件
}

// NewAppComponents 创建共享的应用组件
func NewAppComponents() *AppComponents {
    cryptor, _ := crypto.NewAESCryptor(getEncryptionKey())
    store, _ := store.NewSQLiteStore(getDBPath(), cryptor)
    sshConnector := ssh.NewSSHClient(ssh.NewHostKeyChecker())
    serverService := service.NewServerService(store)
    
    return &AppComponents{
        Store:         store,
        SSHConnector:  sshConnector,
        ServerService: serverService,
    }
}
```

## 11. 添加新实现

### 11.1 确定实现类型

1. **存储实现**：在 `infra/store/` 中添加
2. **协议实现**：在对应的 `infra/` 子目录中添加
3. **外部服务实现**：在 `infra/` 对应子目录中添加

### 11.2 实现步骤

1. 在 `infra/` 对应目录中创建新文件
2. 实现端口层定义的接口
3. 在组合根中注入实现
4. 编写集成测试

### 11.3 示例

```go
// infra/notify/email.go

package notify

import (
    "assh/asshc/domain"
    "assh/asshc/port"
    "net/smtp"
)

type EmailNotifier struct {
    smtpHost string
    smtpPort int
    username string
    password string
    from     string
}

func NewEmailNotifier(host string, port int, user, pass, from string) *EmailNotifier {
    return &EmailNotifier{
        smtpHost: host,
        smtpPort: port,
        username: user,
        password: pass,
        from:     from,
    }
}

func (n *EmailNotifier) Notify(event *domain.NotificationEvent) error {
    // 实现邮件发送
    msg := fmt.Sprintf("Subject: %s\n\n%s", event.Type, event.Message)
    addr := fmt.Sprintf("%s:%d", n.smtpHost, n.smtpPort)
    auth := smtp.PlainAuth("", n.username, n.password, n.smtpHost)
    
    return smtp.SendMail(addr, auth, n.from, []string{event.To}, []byte(msg))
}
```

## 12. 相关文档

- [DDD 架构概览](./overview.md)
- [领域层设计](./domain-layer.md)
- [端口层设计](./port-layer.md)
- [服务层设计](./service-layer.md)
