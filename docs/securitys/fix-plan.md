# ASSH v2.0.0 安全修复方案

**文档日期**: 2026-05-19  
**关联报告**: [综合安全审查报告](security-audit-2026-05-19.md)  
**目标**: 为所有安全问题提供详细的修复步骤和验证方法

---

## 目录

1. [P0 立即修复](#1-p0-立即修复)
2. [P1 短期修复](#2-p1-短期修复)
3. [P2 中期修复](#3-p2-中期修复)
4. [P3 长期改进](#4-p3-长期改进)
5. [修复验证清单](#5-修复验证清单)

---

## 1. P0 立即修复

### 1.1 CVE-2025-47914: 升级 golang.org/x/crypto

**问题**: SSH Agent 越界读取漏洞  
**文件**: `go.mod`  
**预计工作量**: 0.5h

#### 修复步骤

1. **升级依赖**:
```bash
cd asshv2
go get golang.org/x/crypto@v0.35.0
go mod tidy
```

2. **验证编译**:
```bash
go build ./...
```

3. **运行测试**:
```bash
go test ./...
```

#### 验证方法

- 检查 `go.mod` 中 `golang.org/x/crypto` 版本 ≥ v0.35.0
- 运行 `go vet ./...` 无错误
- 运行 `go test ./...` 全部通过

---

### 1.2 Host Key 验证缺失

**问题**: SSH 连接默认不验证 Host Key，易受 MitM 攻击  
**文件**: `asshc/infra/ssh/client.go:362-369`  
**预计工作量**: 4h

#### 修复步骤

1. **修复 `getHostKeyCallback()` 逻辑**:
```go
// asshc/infra/ssh/client.go

func (c *Connector) getHostKeyCallback(server *domain.Server) ssh.HostKeyCallback {
    // 检查是否跳过 host key 验证
    if skip, ok := server.Options["insecure_skip_hostkey"]; ok {
        if v, ok := skip.(bool); ok && v {
            log.Warnf("Host key verification disabled for %s:%d", server.Host, server.Port)
            return ssh.InsecureIgnoreHostKey()
        }
    }
    
    // 默认：使用 known_hosts 文件验证
    return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
        return c.verifyHostKey(hostname, remote, key, server)
    }
}
```

2. **实现 `verifyHostKey()` 方法**:
```go
func (c *Connector) verifyHostKey(hostname string, remote net.Addr, key ssh.PublicKey, server *domain.Server) error {
    // 获取 known_hosts 文件路径
    knownHostsPath := c.getKnownHostsPath()
    
    // 读取 known_hosts 文件
    hosts, err := knownhosts.New(knownHostsPath)
    if err != nil {
        // 如果文件不存在，创建并提示用户
        if os.IsNotExist(err) {
            return c.handleUnknownHost(hostname, key, knownHostsPath)
        }
        return fmt.Errorf("failed to read known_hosts: %w", err)
    }
    
    // 验证 host key
    err = hosts(hostname, remote, key)
    if err != nil {
        // 如果是未知主机，提示用户
        var keyErr *knownhosts.KeyError
        if errors.As(err, &keyErr) {
            return c.handleUnknownHost(hostname, key, knownHostsPath)
        }
        return err
    }
    
    return nil
}
```

3. **实现 `handleUnknownHost()` 方法**:
```go
func (c *Connector) handleUnknownHost(hostname string, key ssh.PublicKey, knownHostsPath string) error {
    // 获取 key 指纹
    fingerprint := ssh.FingerprintSHA256(key)
    
    // 交互式提示（如果在终端中运行）
    if term.IsTerminal(int(os.Stdin.Fd())) {
        fmt.Printf("The authenticity of host '%s' can't be established.\n", hostname)
        fmt.Printf("%s key fingerprint is %s\n", key.Type(), fingerprint)
        fmt.Printf("Are you sure you want to continue connecting (yes/no)? ")
        
        var response string
        fmt.Scanln(&response)
        
        if strings.ToLower(response) != "yes" {
            return fmt.Errorf("host key verification failed: user rejected")
        }
        
        // 添加到 known_hosts
        return c.addToKnownHosts(hostname, key, knownHostsPath)
    }
    
    // 非交互式环境：拒绝连接
    return fmt.Errorf("host key verification failed: unknown host %s (fingerprint: %s)", hostname, fingerprint)
}
```

4. **实现 `addToKnownHosts()` 方法**:
```go
func (c *Connector) addToKnownHosts(hostname string, key ssh.PublicKey, knownHostsPath string) error {
    // 确保目录存在
    dir := filepath.Dir(knownHostsPath)
    if err := os.MkdirAll(dir, 0700); err != nil {
        return fmt.Errorf("failed to create known_hosts directory: %w", err)
    }
    
    // 打开文件（追加模式）
    f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
    if err != nil {
        return fmt.Errorf("failed to open known_hosts: %w", err)
    }
    defer f.Close()
    
    // 写入 host key
    line := knownhosts.Line([]string{hostname}, key)
    if _, err := f.WriteString(line + "\n"); err != nil {
        return fmt.Errorf("failed to write known_hosts: %w", err)
    }
    
    return nil
}
```

5. **添加依赖**:
```bash
go get golang.org/x/crypto/ssh/knownhosts
```

#### 验证方法

- 测试默认连接时验证 host key
- 测试 `insecure_skip_hostkey: true` 时跳过验证
- 测试首次连接时交互式提示
- 测试 known_hosts 文件创建和读取
- 运行 `go test ./asshc/infra/ssh/...`

---

### 1.3 远程命令注入修复 (sha256sum)

**问题**: `remotePath` 直接拼接到 shell 命令中  
**文件**: `asshc/infra/sftp/upload.go:305`, `asshc/infra/sftp/verify.go:140`  
**预计工作量**: 2h

#### 修复方案 A: 使用 shellquote（推荐）

1. **添加依赖**:
```bash
go get github.com/kballard/go-shellquote
```

2. **修复 `upload.go`**:
```go
// asshc/infra/sftp/upload.go:305

import "github.com/kballard/go-shellquote"

// 替换前
// out, err := session.Output("sha256sum " + remotePath)

// 替换后
safePath := shellquote.Join(remotePath)
out, err := session.Output("sha256sum " + safePath)
```

3. **修复 `verify.go`**:
```go
// asshc/infra/sftp/verify.go:140

import "github.com/kballard/go-shellquote"

// 替换前
// out, err := session.Output("sha256sum " + remotePath)

// 替换后
safePath := shellquote.Join(remotePath)
out, err := session.Output("sha256sum " + safePath)
```

#### 修复方案 B: 本地计算 hash（更安全）

1. **创建本地 hash 计算函数**:
```go
// asshc/infra/sftp/hash.go

package sftp

import (
    "crypto/sha256"
    "fmt"
    "io"
    "os"
)

func calculateLocalHash(filePath string) (string, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return "", fmt.Errorf("failed to open file: %w", err)
    }
    defer f.Close()
    
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", fmt.Errorf("failed to calculate hash: %w", err)
    }
    
    return fmt.Sprintf("%x", h.Sum(nil)), nil
}
```

2. **修改验证逻辑**:
```go
// 在 upload.go 和 verify.go 中，改用本地计算 hash
// 通过 SFTP 读取远程文件内容，本地计算 hash
```

#### 验证方法

- 测试包含特殊字符的路径（如 `; rm -rf /`）
- 测试包含空格的路径
- 测试正常路径
- 运行 `go test ./asshc/infra/sftp/...`

---

### 1.4 公钥部署命令注入修复

**问题**: 公钥内容直接拼接到 shell 命令中  
**文件**: `asshc/service/deploy.go:121-132`  
**预计工作量**: 1h

#### 修复方案: 使用 stdin 传入公钥

1. **修改 `appendToAuthorizedKeys()` 方法**:
```go
// asshc/service/deploy.go

func (s *DeployService) appendToAuthorizedKeys(session *ssh.Session, pubKey []byte) error {
    // 使用 stdin 传入公钥，避免 shell 转义
    session.Stdin = bytes.NewReader(append(pubKey, '\n'))
    
    cmd := `mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`
    
    return session.Run(cmd)
}
```

2. **修改调用方**:
```go
// deploy.go:121-132

// 替换前
pubKeyStr := strings.ReplaceAll(string(pubKey), "'", "'\"'\"'")
cmd := fmt.Sprintf(
    `mkdir -p ~/.ssh && chmod 700 ~/.ssh && printf '%%s\n' '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`,
    pubKeyStr,
)
if err := session.Run(cmd); err != nil {
    return fmt.Errorf("failed to append public key: %w", err)
}

// 替换后
if err := s.appendToAuthorizedKeys(session, pubKey); err != nil {
    return fmt.Errorf("failed to append public key: %w", err)
}
```

#### 验证方法

- 测试正常公钥部署
- 测试包含特殊字符的公钥（理论上不应出现，但应防御）
- 测试远程服务器上 authorized_keys 文件权限

---

### 1.5 HTTP CONNECT 握手无限制修复

**问题**: HTTP 头部读取无限制，易受 Slowloris 攻击  
**文件**: `asshc/infra/proxy/http_connect.go:151-205`  
**预计工作量**: 2h

#### 修复步骤

1. **添加读取超时和限制**:
```go
// asshc/infra/proxy/http_connect.go

func (p *HTTPConnectProxy) httpHandshake(conn net.Conn) (string, error) {
    // 设置读取超时（30秒）
    conn.SetReadDeadline(time.Now().Add(30 * time.Second))
    defer conn.SetReadDeadline(time.Time{})
    
    // 使用固定大小的缓冲区
    br := bufio.NewReaderSize(conn, 4096)
    
    // 读取请求行
    reqLine, err := br.ReadString('\n')
    if err != nil {
        return "", fmt.Errorf("failed to read request line: %w", err)
    }
    
    // 解析请求行
    parts := strings.SplitN(strings.TrimSpace(reqLine), " ", 3)
    if len(parts) < 3 {
        return "", fmt.Errorf("invalid request line: %s", reqLine)
    }
    
    method, target, _ := parts[0], parts[1], parts[2]
    
    // 验证方法
    if !strings.EqualFold(method, "CONNECT") {
        return "", fmt.Errorf("unsupported method: %s", method)
    }
    
    // 读取头部（限制数量）
    maxHeaders := 100
    headerCount := 0
    maxHeaderSize := 8192 // 单个头部最大 8KB
    
    for {
        line, err := br.ReadString('\n')
        if err != nil {
            return "", fmt.Errorf("failed to read header: %w", err)
        }
        
        // 移除行尾的 \r\n
        line = strings.TrimRight(line, "\r\n")
        
        // 空行表示头部结束
        if line == "" {
            break
        }
        
        // 检查头部数量
        headerCount++
        if headerCount > maxHeaders {
            return "", fmt.Errorf("too many headers (max %d)", maxHeaders)
        }
        
        // 检查头部大小
        if len(line) > maxHeaderSize {
            return "", fmt.Errorf("header too large (max %d bytes)", maxHeaderSize)
        }
    }
    
    return target, nil
}
```

2. **添加连接速率限制（可选）**:
```go
// 在 HTTPConnectProxy 结构体中添加
type HTTPConnectProxy struct {
    // ... 现有字段
    rateLimiter *rate.Limiter
}

// 在 NewHTTPConnectProxy() 中初始化
rateLimiter := rate.NewLimiter(rate.Limit(100), 200) // 每秒 100 个请求，突发 200
```

#### 验证方法

- 测试正常 HTTP CONNECT 请求
- 测试超时（发送慢速请求）
- 测试头部数量限制
- 测试头部大小限制
- 运行 `go test ./asshc/infra/proxy/...`

---

### 1.6 测试构建阻断修复

**问题**: mock 接口返回类型不匹配  
**文件**: `asshc/service/connect_test.go:63`  
**预计工作量**: 0.5h

#### 修复步骤

1. **修改 mock 实现**:
```go
// asshc/service/connect_test.go:63

// 替换前
type mockConnector struct {
    connectFunc func(server *domain.Server) (*ssh.Client, error)
}

func (m *mockConnector) Connect(server *domain.Server) (*ssh.Client, error) {
    if m.connectFunc != nil {
        return m.connectFunc(server)
    }
    return &ssh.Client{}, nil
}

func (m *mockConnector) ConnectChain(server *domain.Server, jumpServers []*domain.Server) (interface{}, error) {
    // ...
}

// 替换后
type mockConnector struct {
    connectFunc func(server *domain.Server) (*ssh.Client, error)
}

func (m *mockConnector) Connect(server *domain.Server) (*ssh.Client, error) {
    if m.connectFunc != nil {
        return m.connectFunc(server)
    }
    return &ssh.Client{}, nil
}

func (m *mockConnector) ConnectChain(server *domain.Server, jumpServers []*domain.Server) (*ssh.Client, error) {
    if m.connectFunc != nil {
        return m.connectFunc(server)
    }
    return &ssh.Client{}, nil
}
```

#### 验证方法

- 运行 `go vet ./...` 无错误
- 运行 `go test -race ./asshc/service/...` 全部通过
- 确保 16 个测试用例全部通过

---

## 2. P1 短期修复

### 2.1 七牛云明文 HTTP 修复

**问题**: 同步数据通过明文 HTTP 传输  
**文件**: `asshc/infra/sync/qiniu/qiniu.go`  
**预计工作量**: 1h

#### 修复步骤

1. **启用 HTTPS**:
```go
// asshc/infra/sync/qiniu/qiniu.go:68

// 替换前
s.cfg = storage.Config{
    Zone:          zone,
    UseHTTPS:      false,
    UseCdnDomains: false,
}

// 替换后
s.cfg = storage.Config{
    Zone:          zone,
    UseHTTPS:      true,
    UseCdnDomains: false,
}
```

2. **修改 Pull URL**:
```go
// asshc/infra/sync/qiniu/qiniu.go:139

// 替换前
downloadURL := fmt.Sprintf("http://%s/%s", domain, syncFileName)

// 替换后
downloadURL := fmt.Sprintf("https://%s/%s", domain, syncFileName)
```

#### 验证方法

- 测试 Push 操作使用 HTTPS
- 测试 Pull 操作使用 HTTPS
- 抓包验证无明文 HTTP 流量

---

### 2.2 PRAGMA SQL 注入修复

**问题**: 表名直接拼接到 PRAGMA 语句中  
**文件**: `asshc/infra/store/db.go:168`  
**预计工作量**: 1h

#### 修复步骤

1. **添加表名白名单验证**:
```go
// asshc/infra/store/db.go

// 定义允许的表名
var allowedTables = map[string]bool{
    "servers":         true,
    "cloud_accounts":  true,
    "jump_history":    true,
    "config":          true,
    "server_changelog": true,
}

func (s *Store) columnExists(table, column string) (bool, error) {
    // 验证表名
    if !allowedTables[table] {
        return false, fmt.Errorf("table %q is not allowed", table)
    }
    
    rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
    if err != nil {
        return false, err
    }
    defer rows.Close()
    
    for rows.Next() {
        var cid int
        var name, typ string
        var notnull int
        var dfltValue interface{}
        var pk int
        if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
            return false, err
        }
        if name == column {
            return true, nil
        }
    }
    
    return false, nil
}
```

#### 验证方法

- 测试合法表名
- 测试非法表名（应返回错误）
- 运行 `go test ./asshc/infra/store/...`

---

### 2.3 SmartProxy.Stop() 竞态修复

**问题**: Stop() 不等待 goroutine 完成  
**文件**: `asshc/infra/proxy/listener.go:110-131`  
**预计工作量**: 1h

#### 修复步骤

1. **修改 Stop() 方法**:
```go
// asshc/infra/proxy/listener.go

func (sp *SmartProxy) Stop() error {
    sp.mu.Lock()
    
    // 关闭监听器
    if sp.socks5Listener != nil {
        sp.socks5Listener.Close()
    }
    if sp.httpListener != nil {
        sp.httpListener.Close()
    }
    
    // 关闭所有活跃连接
    for conn := range sp.active {
        conn.Close()
    }
    sp.active = make(map[net.Conn]struct{})
    
    // 清空客户端
    sp.client = nil
    
    // 取消 context
    sp.cancel()
    
    sp.mu.Unlock()
    
    // 等待所有 goroutine 完成
    sp.wg.Wait()
    
    return nil
}
```

#### 验证方法

- 测试 Stop() 后无残留 goroutine
- 运行 `go test -race ./asshc/infra/proxy/...`

---

### 2.4 tunnelManager.Close() 死锁修复

**问题**: Close() 可能永久阻塞  
**文件**: `asshc/infra/proxy/tunnel_manager.go:241-245`  
**预计工作量**: 1h

#### 修复步骤

1. **添加超时机制**:
```go
// asshc/infra/proxy/tunnel_manager.go

func (m *tunnelManager) Close() {
    m.cancel()
    
    // 等待 done channel 关闭，超时 5 秒
    select {
    case <-m.done:
        // 正常关闭
    case <-time.After(5 * time.Second):
        log.Warnf("tunnel manager close timeout")
    }
}
```

2. **或者预关闭 channel**:
```go
// 在 NewTunnelManager() 中
func NewTunnelManager(client *ssh.Client) *tunnelManager {
    ctx, cancel := context.WithCancel(context.Background())
    done := make(chan struct{})
    close(done) // 预关闭，如果 reconnectLoop 未启动，Close() 不会阻塞
    
    return &tunnelManager{
        client:  client,
        ctx:     ctx,
        cancel:  cancel,
        done:    done,
        forwards: make(map[string]*LocalForward),
    }
}
```

#### 验证方法

- 测试正常关闭（reconnectLoop 已启动）
- 测试直接关闭（reconnectLoop 未启动）
- 运行 `go test ./asshc/infra/proxy/...`

---

### 2.5 批处理命令 OOM 修复

**问题**: 命令输出无大小限制  
**文件**: `cmd/connect.go:298-360`  
**预计工作量**: 2h

#### 修复步骤

1. **添加输出大小限制**:
```go
// cmd/connect.go

const maxOutputSize = 10 * 1024 * 1024 // 10MB

// 在收集输出时限制大小
go func(n string) {
    output, runErr := a.connectSvc.RunWithOutput(client, cmd)
    
    // 限制输出大小
    if len(output) > maxOutputSize {
        output = output[:maxOutputSize] + "\n... (truncated)"
    }
    
    result := bcServerResult{
        Name:   n,
        Output: output,
        Err:    runErr,
    }
    
    resultsMu.Lock()
    allResults = append(allResults, result)
    resultsMu.Unlock()
}(name)
```

2. **添加并发限制**:
```go
// 添加信号量限制并发数
sem := make(chan struct{}, 10) // 最多 10 个并发

for _, server := range servers {
    sem <- struct{}{} // 获取信号量
    go func(s *domain.Server) {
        defer func() { <-sem }() // 释放信号量
        // ... 执行命令
    }(server)
}
```

#### 验证方法

- 测试大输出命令（如 `cat /dev/urandom | head -c 20M`）
- 测试大量服务器并发执行
- 监控内存使用情况

---

### 2.6 SFTP 递归深度限制

**问题**: 递归遍历无深度限制  
**文件**: `asshc/infra/sftp/upload.go:232-254`, `asshc/infra/sftp/download.go:205-227`  
**预计工作量**: 1h

#### 修复步骤

1. **添加深度和数量限制**:
```go
// asshc/infra/sftp/upload.go

const maxDepth = 100
const maxFiles = 100000

func collectLocalFiles(baseDir, currentDir string, depth int) ([]string, error) {
    // 检查深度
    if depth > maxDepth {
        return nil, fmt.Errorf("max directory depth exceeded (%d)", maxDepth)
    }
    
    entries, err := os.ReadDir(currentDir)
    if err != nil {
        return nil, err
    }
    
    var files []string
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
        
        // 检查文件数量
        if len(files) > maxFiles {
            return nil, fmt.Errorf("max file count exceeded (%d)", maxFiles)
        }
    }
    
    return files, nil
}
```

2. **修改调用方**:
```go
// 调用时传入初始深度 0
files, err := collectLocalFiles(baseDir, baseDir, 0)
```

#### 验证方法

- 测试正常目录遍历
- 测试超深目录结构
- 测试超多文件数量
- 运行 `go test ./asshc/infra/sftp/...`

---

### 2.7 qiniu Pull 响应体限制

**问题**: HTTP 响应体无大小限制  
**文件**: `asshc/infra/sync/qiniu/qiniu.go:150-155`  
**预计工作量**: 0.5h

#### 修复步骤

1. **添加大小限制**:
```go
// asshc/infra/sync/qiniu/qiniu.go

const maxSyncDataSize = 50 * 1024 * 1024 // 50MB

// 替换前
buf := new(bytes.Buffer)
_, err = buf.ReadFrom(resp.Body)

// 替换后
limitedReader := io.LimitReader(resp.Body, maxSyncDataSize+1) // +1 用于检测是否超限
buf := new(bytes.Buffer)
_, err = buf.ReadFrom(limitedReader)
if err != nil {
    return nil, fmt.Errorf("failed to read sync data: %w", err)
}

if int64(buf.Len()) > maxSyncDataSize {
    return nil, fmt.Errorf("sync data exceeds maximum size (%d bytes)", maxSyncDataSize)
}
```

#### 验证方法

- 测试正常大小的同步数据
- 测试超大数据（应返回错误）
- 运行 `go test ./asshc/infra/sync/...`

---

## 3. P2 中期修复

### 3.1 加密密钥同库存储修复

**问题**: AES 密钥与密文存储在同一数据库中  
**文件**: `asshc/infra/store/`  
**预计工作量**: 8h

#### 修复方案

1. **使用操作系统密钥链（推荐）**:
```go
// asshc/infra/store/keychain.go

package store

import (
    "github.com/keybase/go-keychain"
)

const service = "com.assh.client"

func getEncryptionKey() ([]byte, error) {
    // 尝试从密钥链获取
    query := keychain.NewItem()
    query.SetSecClass(keychain.SecClassGenericPassword)
    query.SetService(service)
    query.SetAccount("encryption-key")
    query.SetMatchLimit(keychain.MatchLimitOne)
    query.SetReturnData(true)
    
    results, err := keychain.QueryItem(query)
    if err != nil {
        return nil, err
    }
    
    if len(results) > 0 {
        return results[0].Data, nil
    }
    
    // 生成新密钥
    key := make([]byte, 32)
    if _, err := rand.Read(key); err != nil {
        return nil, err
    }
    
    // 存储到密钥链
    item := keychain.NewItem()
    item.SetSecClass(keychain.SecClassGenericPassword)
    item.SetService(service)
    item.SetAccount("encryption-key")
    item.SetData(key)
    item.SetAccessible(keychain.AccessibleWhenUnlocked)
    
    if err := keychain.AddItem(item); err != nil {
        return nil, err
    }
    
    return key, nil
}
```

2. **迁移现有密钥**:
```go
// 创建迁移函数
func migrateEncryptionKey(db *sql.DB) error {
    // 从 config 表读取现有密钥
    var keyBase64 string
    err := db.QueryRow("SELECT value FROM config WHERE key = 'encryption_key'").Scan(&keyBase64)
    if err != nil {
        return err
    }
    
    key, err := base64.StdEncoding.DecodeString(keyBase64)
    if err != nil {
        return err
    }
    
    // 存储到密钥链
    if err := storeInKeychain(key); err != nil {
        return err
    }
    
    // 从 config 表删除（可选，或标记为已迁移）
    _, err = db.Exec("DELETE FROM config WHERE key = 'encryption_key'")
    return err
}
```

#### 验证方法

- 测试新安装时密钥存储在密钥链
- 测试从旧版本迁移密钥
- 测试加密/解密功能正常

---

### 3.2 变更日志明文密码修复

**问题**: 快照中的密码以明文存储  
**文件**: `asshc/infra/store/server.go:112-116`  
**预计工作量**: 4h

#### 修复步骤

1. **创建快照时加密密码字段**:
```go
// asshc/infra/store/server.go

func (s *Store) createSnapshot(server *domain.Server) ([]byte, error) {
    // 创建副本
    snapshot := *server
    
    // 加密密码字段
    if snapshot.Auth.Password != "" {
        encrypted, err := s.encryptPassword(snapshot.Auth.Password)
        if err != nil {
            return nil, fmt.Errorf("failed to encrypt snapshot password: %w", err)
        }
        snapshot.Auth.Password = encrypted
    }
    
    return json.Marshal(snapshot)
}
```

2. **恢复快照时解密密码字段**:
```go
func (s *Store) restoreSnapshot(snapshotJSON []byte) (*domain.Server, error) {
    var server domain.Server
    if err := json.Unmarshal(snapshotJSON, &server); err != nil {
        return nil, err
    }
    
    // 解密密码字段
    if server.Auth.Password != "" {
        decrypted, err := s.decryptPassword(server.Auth.Password)
        if err != nil {
            return nil, fmt.Errorf("failed to decrypt snapshot password: %w", err)
        }
        server.Auth.Password = decrypted
    }
    
    return &server, nil
}
```

#### 验证方法

- 测试创建快照时密码加密
- 测试恢复快照时密码解密
- 检查数据库中快照无明文密码

---

### 3.3 AES ECB 模式暴露修复

**问题**: ECB 模式通过公开 API 暴露  
**文件**: `asshc/infra/crypto/aes.go`  
**预计工作量**: 2h

#### 修复步骤

1. **标记为 deprecated**:
```go
// asshc/infra/crypto/aes.go

// Deprecated: AESEncryptECB 使用不安全的 ECB 模式，仅用于兼容旧系统。
// 请使用 AESEncryptGCM 替代。
func AESEncryptECB(key, plaintext []byte) ([]byte, error) {
    // ... 现有实现
}

// Deprecated: AESDecryptECB 使用不安全的 ECB 模式，仅用于兼容旧系统。
// 请使用 AESDecryptGCM 替代。
func AESDecryptECB(key, ciphertext []byte) ([]byte, error) {
    // ... 现有实现
}
```

2. **添加文档警告**:
```go
// 在 package doc 中添加警告
// Package crypto 提供加密和解密功能。
//
// 警告：请勿使用 ECB 模式（AESEncryptECB/AESDecryptECB），它不安全。
// 始终使用 GCM 模式（AESEncryptGCM/AESDecryptGCM）。
package crypto
```

#### 验证方法

- 运行 `go vet ./...` 检查 deprecated 使用
- 确保新代码不使用 ECB 模式

---

### 3.4 加解密代码重复修复

**问题**: 3 份相同的 AES-GCM 实现  
**文件**: `key.go`, `account.go`, `aes.go`  
**预计工作量**: 4h

#### 修复步骤

1. **统一到 `aes.go`**:
```go
// asshc/infra/crypto/aes.go

// EncryptGCM 使用 AES-256-GCM 加密数据
func EncryptGCM(key, plaintext []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
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
    
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    return ciphertext, nil
}

// DecryptGCM 使用 AES-256-GCM 解密数据
func DecryptGCM(key, ciphertext []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, fmt.Errorf("ciphertext too short")
    }
    
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, err
    }
    
    return plaintext, nil
}
```

2. **修改 `key.go` 和 `account.go` 使用统一实现**:
```go
// key.go
func (s *Store) encryptPassword(password string) (string, error) {
    encrypted, err := crypto.EncryptGCM(s.cryptoKey, []byte(password))
    if err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(encrypted), nil
}
```

#### 验证方法

- 运行 `go test ./asshc/infra/crypto/...`
- 运行 `go test ./asshc/infra/store/...`
- 确保加密/解密功能正常

---

### 3.5 JSON/时间解析错误修复

**问题**: 多处解析错误被静默忽略  
**文件**: 多个文件  
**预计工作量**: 2h

#### 修复步骤

1. **添加日志记录**:
```go
// server.go:465
if optionsJSON != "" && optionsJSON != "{}" {
    if err := json.Unmarshal([]byte(optionsJSON), &server.Options); err != nil {
        log.Warnf("failed to parse options for server %s.%s: %v", group, name, err)
        // 保持为空 map，但记录警告
    }
}

// jump.go:74,102
if err := json.Unmarshal([]byte(decrypted), &jh.PathData); err != nil {
    log.Warnf("failed to parse jump history path data: %v", err)
    // 保持为空，但记录警告
}

// account.go:212-229
acct.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
if err != nil {
    log.Warnf("failed to parse created_at: %v", err)
    acct.CreatedAt = time.Time{} // 使用零值
}
```

#### 验证方法

- 测试损坏的 JSON 数据
- 测试无效的时间格式
- 检查日志输出

---

### 3.6 generateID fallback 修复

**问题**: fallback 输出指针地址  
**文件**: `asshc/infra/proxy/port.go:256-257`  
**预计工作量**: 0.5h

#### 修复步骤

1. **修改 fallback 实现**:
```go
// asshc/infra/proxy/port.go

func generateID() string {
    b := make([]byte, 8)
    if _, err := rand.Read(b); err != nil {
        // 使用时间戳和 PID 作为 fallback
        return fmt.Sprintf("tunnel-%d-%d", time.Now().UnixNano(), os.Getpid())
    }
    return fmt.Sprintf("%x", b)
}
```

#### 验证方法

- 测试正常情况（rand.Read 成功）
- 测试 fallback 情况（模拟 rand.Read 失败）

---

## 4. P3 长期改进

### 4.1 密钥轮换机制

**目标**: 实现定期自动密钥轮换和重加密  
**预计工作量**: 16h

#### 设计方案

1. **密钥版本管理**:
```go
type KeyVersion struct {
    Version   int
    Key       []byte
    CreatedAt time.Time
    RetiredAt *time.Time
}
```

2. **轮换流程**:
   - 生成新密钥
   - 使用新密钥重新加密所有数据
   - 更新密钥版本
   - 保留旧密钥用于解密历史数据

3. **定时任务**:
```go
// 每 90 天轮换一次
func (s *Store) scheduleKeyRotation() {
    ticker := time.NewTicker(90 * 24 * time.Hour)
    go func() {
        for range ticker.C {
            if err := s.rotateEncryptionKey(); err != nil {
                log.Errorf("key rotation failed: %v", err)
            }
        }
    }()
}
```

---

### 4.2 Known Hosts 完整实现

**目标**: 支持 `known_hosts` 文件管理和指纹验证  
**预计工作量**: 8h

#### 功能列表

- 支持标准 `known_hosts` 格式
- 支持多种密钥类型（RSA, ED25519, ECDSA）
- 支持指纹验证
- 支持交互式添加
- 支持命令行管理（`assh hostkey add/list/remove`）

---

### 4.3 代理组件统一抽象

**目标**: 创建统一的 `ConnectionPool` 接口  
**预计工作量**: 12h

#### 接口设计

```go
type ConnectionPool interface {
    Add(conn net.Conn)
    Remove(conn net.Conn)
    CloseAll() error
    Count() int
    Wait() // 等待所有连接关闭
}
```

---

### 4.4 HTTP 代理 TLS 支持

**目标**: 添加 TLS 加密和证书管理  
**预计工作量**: 8h

#### 功能列表

- 支持自签名证书
- 支持 Let's Encrypt
- 支持证书轮换
- 支持 TLS 1.3

---

### 4.5 统一 shell escape 工具

**目标**: 创建安全的命令构建工具包  
**预计工作量**: 4h

#### 工具函数

```go
package shell

// Escape 转义 shell 参数
func Escape(arg string) string {
    return shellquote.Join(arg)
}

// Command 构建安全的命令
func Command(args ...string) string {
    return shellquote.Join(args...)
}
```

---

### 4.6 资源泄漏修复

**目标**: 修复所有资源泄漏问题  
**预计工作量**: 4h

#### 修复列表

- 添加 `defer session.Close()`
- 添加 `defer conn.Close()`
- 添加 `defer file.Close()`
- 使用 context 管理 goroutine 生命周期

---

## 5. 修复验证清单

### 5.1 单元测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./asshc/infra/ssh/...
go test ./asshc/infra/sftp/...
go test ./asshc/infra/proxy/...
go test ./asshc/service/...
```

### 5.2 静态分析

```bash
# 运行 go vet
go vet ./...

# 运行 staticcheck（如果安装）
staticcheck ./...

# 运行 golangci-lint（如果安装）
golangci-lint run
```

### 5.3 数据竞争检测

```bash
# 运行 race 检测
go test -race ./...
```

### 5.4 安全扫描

```bash
# 运行 govulncheck（如果安装）
govulncheck ./...

# 运行 gosec（如果安装）
gosec ./...
```

### 5.5 集成测试

- 测试 SSH 连接（正常、跳板机）
- 测试 SFTP 上传/下载
- 测试代理功能（SOCKS5、HTTP CONNECT）
- 测试云同步（Push、Pull）
- 测试密钥管理

---

## 附录

### A. 依赖项更新

```bash
# 更新所有依赖
go get -u ./...

# 更新特定依赖
go get golang.org/x/crypto@latest
go get modernc.org/sqlite@latest

# 清理依赖
go mod tidy
```

### B. 代码格式化

```bash
# 格式化代码
gofmt -w .

# 检查格式
gofmt -l .
```

### C. 文档更新

修复完成后，更新以下文档：
- [ ] `CHANGELOG.md` — 添加修复记录
- [ ] `docs/TODO.md` — 更新任务状态
- [ ] `docs/bugs.md` — 添加已修复问题

---

*文档生成工具: opencode (mi/mimo-v2.5-pro)*  
*最后更新: 2026-05-19*