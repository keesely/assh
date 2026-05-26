# ASSH v2.0.0 安全修复方案

**文档日期**: 2026-05-19  
**关联报告**: [SECURITYS.md](SECURITYS.md)（综合安全审查报告）  
**源文档**: `docs/securitys/fix-plan.md`（详细修复步骤）、`docs/securitys/security-audit-2026-05-19.md §10`（修复摘要）  
**目标**: 为所有安全问题提供分优先级、可执行的修复步骤和验证方法  
**版本**: 2.0（合并去重版）

---

## 目录

1. [P0 立即修复](#1-p0-立即修复)
2. [P1 短期修复](#2-p1-短期修复)
3. [P2 中期修复](#3-p2-中期修复)
4. [P3 长期改进](#4-p3-长期改进)
5. [修复验证清单](#5-修复验证清单)
6. [附录](#6-附录)

---

## 1. P0 立即修复

> 总计预估工作量: **~10h**
> 安全关键，阻塞构建，需优先处理

---

### 1.1 CVE-2025-47914 + CVE-2025-58181: 升级 golang.org/x/crypto

**安全报告编号**: 🔴 #2  
**文件**: `go.mod`  
**预估工作量**: 0.5h

#### 修复步骤

```bash
cd asshv2
go get golang.org/x/crypto@v0.45.0
go mod tidy
```

#### 验证方法

- `go.mod` 中 `golang.org/x/crypto` 版本 ≥ v0.45.0
- `go vet ./...` 无错误
- `go test ./...` 全部通过

---

### 1.2 Host Key 验证缺失

**安全报告编号**: 🔴 #1  
**文件**: `asshc/infra/ssh/client.go:362-369` + `hostkey.go`  
**预估工作量**: 4h

#### 修复步骤

**Step 1: 修复 `getHostKeyCallback()` 逻辑**

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

**Step 2: 实现 `verifyHostKey()`**

```go
func (c *Connector) verifyHostKey(hostname string, remote net.Addr, key ssh.PublicKey, server *domain.Server) error {
    knownHostsPath := c.getKnownHostsPath()

    hosts, err := knownhosts.New(knownHostsPath)
    if err != nil {
        if os.IsNotExist(err) {
            return c.handleUnknownHost(hostname, key, knownHostsPath)
        }
        return fmt.Errorf("failed to read known_hosts: %w", err)
    }

    err = hosts(hostname, remote, key)
    if err != nil {
        var keyErr *knownhosts.KeyError
        if errors.As(err, &keyErr) {
            return c.handleUnknownHost(hostname, key, knownHostsPath)
        }
        return err
    }
    return nil
}
```

**Step 3: 实现 `handleUnknownHost()`**

```go
func (c *Connector) handleUnknownHost(hostname string, key ssh.PublicKey, knownHostsPath string) error {
    fingerprint := ssh.FingerprintSHA256(key)

    if term.IsTerminal(int(os.Stdin.Fd())) {
        fmt.Printf("The authenticity of host '%s' can't be established.\n", hostname)
        fmt.Printf("%s key fingerprint is %s\n", key.Type(), fingerprint)
        fmt.Printf("Are you sure you want to continue connecting (yes/no)? ")

        var response string
        fmt.Scanln(&response)

        if strings.ToLower(response) != "yes" {
            return fmt.Errorf("host key verification failed: user rejected")
        }
        return c.addToKnownHosts(hostname, key, knownHostsPath)
    }

    return fmt.Errorf("host key verification failed: unknown host %s (fingerprint: %s)", hostname, fingerprint)
}
```

**Step 4: 实现 `addToKnownHosts()`**

```go
func (c *Connector) addToKnownHosts(hostname string, key ssh.PublicKey, knownHostsPath string) error {
    dir := filepath.Dir(knownHostsPath)
    if err := os.MkdirAll(dir, 0700); err != nil {
        return fmt.Errorf("failed to create known_hosts directory: %w", err)
    }

    f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
    if err != nil {
        return fmt.Errorf("failed to open known_hosts: %w", err)
    }
    defer f.Close()

    line := knownhosts.Line([]string{hostname}, key)
    if _, err := f.WriteString(line + "\n"); err != nil {
        return fmt.Errorf("failed to write known_hosts: %w", err)
    }
    return nil
}
```

**Step 5: 添加依赖**

```bash
go get golang.org/x/crypto/ssh/knownhosts
```

#### 验证方法

- 测试默认连接时验证 host key
- 测试 `insecure_skip_hostkey: true` 时跳过
- 测试首次连接交互式提示
- `go test ./asshc/infra/ssh/...`

---

### 1.3 远程命令注入修复 (sha256sum)

**安全报告编号**: 🔴 #3  
**文件**: `asshc/infra/sftp/upload.go:305`、`asshc/infra/sftp/verify.go:140`  
**预估工作量**: 2h

#### 修复方案 A: 使用 shellquote（快速修复）

```bash
go get github.com/kballard/go-shellquote
```

```go
// upload.go:305 & verify.go:140
import "github.com/kballard/go-shellquote"

safePath := shellquote.Join(remotePath)
out, err := session.Output("sha256sum " + safePath)
```

#### 修复方案 B: 本地计算 hash（更安全，推荐）

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

修改验证逻辑：通过 SFTP 读取远程文件内容，本地计算 hash，完全避免远程命令执行。

#### 验证方法

- 测试含特殊字符路径（如 `; rm -rf /`）
- 测试含空格路径
- 测试正常路径
- `go test ./asshc/infra/sftp/...`

---

### 1.4 公钥部署命令注入修复

**安全报告编号**: 🔴 #4  
**文件**: `asshc/service/deploy.go:121-132`  
**预估工作量**: 1h

#### 修复步骤

```go
// asshc/service/deploy.go

func (s *DeployService) appendToAuthorizedKeys(session *ssh.Session, pubKey []byte) error {
    // 使用 stdin 传入公钥，避免 shell 转义
    session.Stdin = bytes.NewReader(append(pubKey, '\n'))

    cmd := `mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`

    return session.Run(cmd)
}
```

修改调用方：

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
- 测试含特殊字符的公钥（防御性）
- 检查 authorized_keys 文件权限

---

### 1.5 HTTP CONNECT 握手无限制修复

**安全报告编号**: 🔴 #5  
**文件**: `asshc/infra/proxy/http_connect.go:151-205`  
**预估工作量**: 2h

#### 修复步骤

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

    parts := strings.SplitN(strings.TrimSpace(reqLine), " ", 3)
    if len(parts) < 3 {
        return "", fmt.Errorf("invalid request line: %s", reqLine)
    }

    method, target := parts[0], parts[1]

    if !strings.EqualFold(method, "CONNECT") {
        return "", fmt.Errorf("unsupported method: %s", method)
    }

    // 读取头部（限制数量和大小）
    maxHeaders := 100
    maxHeaderSize := 8192
    headerCount := 0

    for {
        line, err := br.ReadString('\n')
        if err != nil {
            return "", fmt.Errorf("failed to read header: %w", err)
        }

        line = strings.TrimRight(line, "\r\n")
        if line == "" {
            break  // 空行 = 头部结束
        }

        headerCount++
        if headerCount > maxHeaders {
            return "", fmt.Errorf("too many headers (max %d)", maxHeaders)
        }
        if len(line) > maxHeaderSize {
            return "", fmt.Errorf("header too large (max %d bytes)", maxHeaderSize)
        }
    }

    return target, nil
}
```

可选：添加连接速率限制：

```go
type HTTPConnectProxy struct {
    // ... 现有字段
    rateLimiter *rate.Limiter
}

// 在 NewHTTPConnectProxy() 中
rateLimiter := rate.NewLimiter(rate.Limit(100), 200)
```

#### 验证方法

- 测试正常 HTTP CONNECT 请求
- 测试超时（慢速请求）
- 测试头部数量/大小限制
- `go test ./asshc/infra/proxy/...`

---

### 1.6 测试构建阻断修复

**安全报告编号**: 🔴 #6  
**文件**: `asshc/service/connect_test.go:63`  
**预估工作量**: 0.5h

#### 修复步骤

```go
// asshc/service/connect_test.go

// 替换前
func (m *mockConnector) ConnectChain(server *domain.Server, jumpServers []*domain.Server) (interface{}, error) {
    if m.connectFunc != nil {
        return m.connectFunc(server)
    }
    return &ssh.Client{}, nil
}

// 替换后
func (m *mockConnector) ConnectChain(server *domain.Server, jumpServers []*domain.Server) (*ssh.Client, error) {
    if m.connectFunc != nil {
        return m.connectFunc(server)
    }
    return &ssh.Client{}, nil
}
```

#### 验证方法

- `go vet ./...` 无错误
- `go test -race ./asshc/service/...` 全部通过
- 确保 16 个测试用例全部通过

---

## 2. P1 短期修复

> 总计预估工作量: **~7.5h**
> 高风险，建议集成测试前完成

---

### 2.1 七牛云明文 HTTP 修复

**安全报告编号**: 🟠 #8  
**文件**: `asshc/infra/sync/qiniu/qiniu.go:68,139`  
**预估工作量**: 1h

#### 修复步骤

```go
// qiniu.go:68 — 启用 HTTPS
s.cfg = storage.Config{
    Zone:          zone,
    UseHTTPS:      true,     // ← false → true
    UseCdnDomains: false,
}

// qiniu.go:139 — Pull URL 改为 HTTPS
downloadURL := fmt.Sprintf("https://%s/%s", domain, syncFileName)  // ← http → https
```

#### 验证方法

- 测试 Push/Pull 使用 HTTPS
- 抓包验证无明文 HTTP 流量

---

### 2.2 PRAGMA SQL 注入修复

**安全报告编号**: 🟠 #7  
**文件**: `asshc/infra/store/db.go:168`  
**预估工作量**: 1h

#### 修复步骤

```go
// asshc/infra/store/db.go

var allowedTables = map[string]bool{
    "servers":          true,
    "cloud_accounts":   true,
    "jump_history":     true,
    "config":           true,
    "server_changelog": true,
}

func (s *Store) columnExists(table, column string) (bool, error) {
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
- `go test ./asshc/infra/store/...`

---

### 2.3 SmartProxy.Stop() 竞态修复

**安全报告编号**: 🟡 #11  
**文件**: `asshc/infra/proxy/listener.go:110-131`  
**预估工作量**: 1h

#### 修复步骤

```go
// asshc/infra/proxy/listener.go

func (sp *SmartProxy) Stop() error {
    sp.mu.Lock()

    if sp.socks5Listener != nil {
        sp.socks5Listener.Close()
    }
    if sp.httpListener != nil {
        sp.httpListener.Close()
    }
    for conn := range sp.active {
        conn.Close()
    }
    sp.active = make(map[net.Conn]struct{})
    sp.client = nil
    sp.cancel()

    sp.mu.Unlock()

    sp.wg.Wait()  // ← 新增：等待所有 goroutine 完成

    return nil
}
```

#### 验证方法

- 测试 Stop() 后无残留 goroutine
- `go test -race ./asshc/infra/proxy/...`

---

### 2.4 tunnelManager.Close() 死锁修复

**安全报告编号**: 🟡 #12  
**文件**: `asshc/infra/proxy/tunnel_manager.go:241-245`  
**预估工作量**: 1h

#### 修复方案 A: 添加超时

```go
func (m *tunnelManager) Close() {
    m.cancel()

    select {
    case <-m.done:
        // 正常关闭
    case <-time.After(5 * time.Second):
        log.Warnf("tunnel manager close timeout")
    }
}
```

#### 修复方案 B: 预关闭 channel（推荐）

```go
// 在 NewTunnelManager() 中
func NewTunnelManager(client *ssh.Client) *tunnelManager {
    ctx, cancel := context.WithCancel(context.Background())
    done := make(chan struct{})
    close(done)  // 预关闭，reconnectLoop 启动时会重新创建

    return &tunnelManager{
        client:   client,
        ctx:      ctx,
        cancel:   cancel,
        done:     done,
        forwards: make(map[string]*LocalForward),
    }
}
```

#### 验证方法

- 测试正常关闭（reconnectLoop 已启动）
- 测试直接关闭（reconnectLoop 未启动）
- `go test ./asshc/infra/proxy/...`

---

### 2.5 批处理命令 OOM 修复

**安全报告编号**: 🟡 #13  
**文件**: `cmd/connect.go:298-360`  
**预估工作量**: 2h

#### 修复步骤

```go
// cmd/connect.go

const maxOutputSize = 10 * 1024 * 1024 // 10MB

// 限制输出大小
go func(n string) {
    output, runErr := a.connectSvc.RunWithOutput(client, cmd)

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

// 限制并发数
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
- 监控内存使用

---

### 2.6 SFTP 递归深度/数量限制

**安全报告编号**: 🟡 #14  
**文件**: `asshc/infra/sftp/upload.go:232-254`、`asshc/infra/sftp/download.go:205-227`  
**预估工作量**: 1h

#### 修复步骤

```go
// asshc/infra/sftp/upload.go

const maxDepth = 100
const maxFiles = 100000

func collectLocalFiles(baseDir, currentDir string, depth int) ([]string, error) {
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

        if len(files) > maxFiles {
            return nil, fmt.Errorf("max file count exceeded (%d)", maxFiles)
        }
    }
    return files, nil
}
```

修改调用方传入初始深度 0：

```go
files, err := collectLocalFiles(baseDir, baseDir, 0)
```

#### 验证方法

- 测试正常目录遍历
- 测试超深/超多文件
- `go test ./asshc/infra/sftp/...`

---

### 2.7 qiniu Pull 响应体限制

**安全报告编号**: 🟡 #15  
**文件**: `asshc/infra/sync/qiniu/qiniu.go:150-155`  
**预估工作量**: 0.5h

#### 修复步骤

```go
// asshc/infra/sync/qiniu/qiniu.go

const maxSyncDataSize = 50 * 1024 * 1024 // 50MB

// 替换前
buf := new(bytes.Buffer)
_, err = buf.ReadFrom(resp.Body)

// 替换后
limitedReader := io.LimitReader(resp.Body, maxSyncDataSize+1)
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

- 测试正常大小同步数据
- 测试超大数据（应返回错误）
- `go test ./asshc/infra/sync/...`

---

## 3. P2 中期修复

> 总计预估工作量: **~20.5h**
> 中风险，架构改进

---

### 3.1 加密密钥同库存储修复

**安全报告编号**: 🟡 #9  
**文件**: `asshc/infra/store/`  
**预估工作量**: 8h

#### 设计方案：迁移至操作系统密钥链

```go
// asshc/infra/store/keychain.go
package store

import (
    "github.com/keybase/go-keychain"
)

const service = "com.assh.client"

func getEncryptionKey() ([]byte, error) {
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

#### 迁移现有密钥

```go
func migrateEncryptionKey(db *sql.DB) error {
    var keyBase64 string
    err := db.QueryRow("SELECT value FROM config WHERE key = 'encryption_key'").Scan(&keyBase64)
    if err != nil {
        return err
    }

    key, err := base64.StdEncoding.DecodeString(keyBase64)
    if err != nil {
        return err
    }

    if err := storeInKeychain(key); err != nil {
        return err
    }

    _, err = db.Exec("DELETE FROM config WHERE key = 'encryption_key'")
    return err
}
```

#### 验证方法

- 新安装时密钥存储在密钥链
- 从旧版本迁移密钥
- 加密/解密功能正常

---

### 3.2 变更日志明文密码修复

**安全报告编号**: 🟡 #10  
**文件**: `asshc/infra/store/server.go:112-116`  
**预估工作量**: 4h

#### 修复步骤

创建快照时加密密码字段：

```go
func (s *Store) createSnapshot(server *domain.Server) ([]byte, error) {
    snapshot := *server

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

恢复快照时解密：

```go
func (s *Store) restoreSnapshot(snapshotJSON []byte) (*domain.Server, error) {
    var server domain.Server
    if err := json.Unmarshal(snapshotJSON, &server); err != nil {
        return nil, err
    }

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

- 创建快照时密码加密
- 恢复快照时密码解密
- 数据库快照无明文密码

---

### 3.3 AES ECB 模式暴露修复

**安全报告编号**: 🟡 #17  
**文件**: `asshc/infra/crypto/aes.go:52,74,199-224`  
**预估工作量**: 2h

#### 修复步骤

标记为 deprecated：

```go
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

Package doc 添加警告：

```go
// Package crypto 提供加密和解密功能。
//
// 警告：请勿使用 ECB 模式（AESEncryptECB/AESDecryptECB），它不安全。
// 始终使用 GCM 模式（AESEncryptGCM/AESDecryptGCM）。
package crypto
```

#### 验证方法

- `go vet ./...` 检查 deprecated 使用
- 新代码不使用 ECB 模式

---

### 3.4 加解密代码重复修复

**安全报告编号**: 🟡 #18  
**文件**: `asshc/infra/store/key.go:69-140`、`asshc/infra/store/account.go:242-314`、`asshc/infra/crypto/aes.go`  
**预估工作量**: 4h

#### 修复步骤

统一到 `asshc/infra/crypto/aes.go`：

```go
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

修改 `key.go` 和 `account.go` 使用统一实现：

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

- `go test ./asshc/infra/crypto/...`
- `go test ./asshc/infra/store/...`
- 加密/解密功能正常

---

### 3.5 JSON/时间解析错误修复

**安全报告编号**: 🟡 #19  
**文件**: 多处  
**预估工作量**: 2h

#### 修复步骤

```go
// server.go:465 — 添加日志
if optionsJSON != "" && optionsJSON != "{}" {
    if err := json.Unmarshal([]byte(optionsJSON), &server.Options); err != nil {
        log.Warnf("failed to parse options for server %s.%s: %v", group, name, err)
    }
}

// jump.go:74,102 — 添加日志
if err := json.Unmarshal([]byte(decrypted), &jh.PathData); err != nil {
    log.Warnf("failed to parse jump history path data: %v", err)
}

// account.go:212-229 — 添加日志
acct.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
if err != nil {
    log.Warnf("failed to parse created_at: %v", err)
    acct.CreatedAt = time.Time{}
}
```

#### 验证方法

- 测试损坏的 JSON/时间数据
- 检查日志输出

---

### 3.6 generateID fallback 修复

**安全报告编号**: 🟡 #20  
**文件**: `asshc/infra/proxy/port.go:256-257`  
**预估工作量**: 0.5h

#### 修复步骤

```go
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
- 测试 fallback（模拟失败）

---

## 4. P3 长期改进

> 总计预估工作量: **~52h**
> 低风险 / 架构优化

### 4.1 密钥轮换机制

**安全报告编号**: 🟡 #9（关联）  
**预估工作量**: 16h

**方案**: 支持密钥版本管理 + 定期轮换 + 重加密

```go
type KeyVersion struct {
    Version   int
    Key       []byte
    CreatedAt time.Time
    RetiredAt *time.Time
}

// 定期轮换：每 90 天
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

**安全报告编号**: 🔴 #1（关联）  
**预估工作量**: 8h

功能：
- 支持标准 `known_hosts` 格式
- 支持多种密钥类型（RSA, ED25519, ECDSA）
- 支持指纹验证
- 交互式添加 + 命令行管理（`assh hostkey add/list/remove`）

---

### 4.3 代理组件统一抽象

**安全报告编号**: 🟡 #11（关联）  
**预估工作量**: 12h

```go
type ConnectionPool interface {
    Add(conn net.Conn)
    Remove(conn net.Conn)
    CloseAll() error
    Count() int
    Wait()
}
```

---

### 4.4 HTTP 代理 TLS 支持

**安全报告编号**: 🔴 #5（关联）  
**预估工作量**: 8h

- 支持自签名证书 / Let's Encrypt
- 支持证书轮换
- 支持 TLS 1.3

---

### 4.5 统一 shell escape 工具

**安全报告编号**: 🔴 #3, #4（关联）  
**预估工作量**: 4h

```go
package shell

func Escape(arg string) string {
    return shellquote.Join(arg)
}

func Command(args ...string) string {
    return shellquote.Join(args...)
}
```

---

### 4.6 资源泄漏修复

**安全报告编号**: 🟢 #24, #26, #29  
**预估工作量**: 4h

- 添加 `defer session.Close()`
- 添加 `defer conn.Close()`
- 添加 `defer file.Close()`
- 使用 context 管理 goroutine 生命周期
- `Daemonize` 添加错误传递和频率限制（#28）
- `keepAlive` 添加 context/done channel 退出（#24）

---

## 5. 修复验证清单

### 5.1 单元测试

```bash
# 运行所有测试
go test ./...

# 特定包
go test ./asshc/infra/ssh/...
go test ./asshc/infra/sftp/...
go test ./asshc/infra/proxy/...
go test ./asshc/infra/store/...
go test ./asshc/infra/crypto/...
go test ./asshc/service/...
go test ./asshc/infra/sync/...
```

### 5.2 静态分析

```bash
go vet ./...

# 可选（安装后）
# staticcheck ./...
# golangci-lint run
```

### 5.3 数据竞争检测

```bash
go test -race ./...
```

### 5.4 安全扫描

```bash
# 可选（安装后）
# govulncheck ./...
# gosec ./...
```

### 5.5 集成测试

- SSH 连接（正常、跳板机、host key 校验）
- SFTP 上传/下载（含特殊路径、深度限制）
- 代理功能（SOCKS5、HTTP CONNECT、限速）
- 云同步（Push、Pull、响应体限制）
- 密钥管理（生成、备份、恢复）
- 变更日志（快照加密、恢复）
- 批处理命令（大输出、高并发）

### 5.6 文档更新

修复完成后更新以下文档：

- [ ] `CHANGELOG.md` — 添加修复记录
- [ ] `docs/TODO.md` — 更新任务状态
- [ ] `docs/bugs.md` — 添加已修复问题

---

## 6. 附录

### A. 依赖项更新

```bash
# 更新所有依赖
go get -u ./...

# 更新特定依赖
go get golang.org/x/crypto@latest
go get modernc.org/sqlite@latest

# 新依赖
go get golang.org/x/crypto/ssh/knownhosts
go get github.com/kballard/go-shellquote

# 清理
go mod tidy
```

### B. 修复优先级速查表

| 优先级 | 问题 | 影响 | 预估 |
|--------|------|------|------|
| **P0** | CVE-2025-47914 升级 | 安全漏洞 | 0.5h |
| **P0** | Host Key 验证 | MitM 无防护 | 4h |
| **P0** | 远程命令注入(sha256sum) | RCE | 2h |
| **P0** | 公钥部署命令注入 | RCE | 1h |
| **P0** | HTTP CONNECT 无限制 | DoS | 2h |
| **P0** | 测试构建阻断 | 所有测试阻塞 | 0.5h |
| **P1** | 七牛云明文 HTTP | 情报泄露 | 1h |
| **P1** | PRAGMA SQL 注入 | SQLi | 1h |
| **P1** | SmartProxy.Stop() 竞态 | panic | 1h |
| **P1** | tunnelManager 死锁 | 永久阻塞 | 1h |
| **P1** | 批处理 OOM | OOM | 2h |
| **P1** | SFTP 递归限制 | 栈溢出 | 1h |
| **P1** | qiniu Pull 响应体 | OOM | 0.5h |
| **P2** | 加密密钥同库 | 架构弱点 | 8h |
| **P2** | 变更日志明文密码 | 情报泄露 | 4h |
| **P2** | ECB 模式暴露 | 误用风险 | 2h |
| **P2** | 加解密代码重复 | 维护风险 | 4h |
| **P2** | JSON/时间解析错误 | 静默失败 | 2h |
| **P2** | generateID fallback | 缺陷 | 0.5h |
| **P3** | 密钥轮换 / Known Hosts / 统一抽象 / TLS / shell escape / 资源泄漏 | 架构优化 | 52h |

### C. 安全报告 #ID 对照

| 本计划中使用 #ID | 对应 SECURITYS.md 风险分级 # | 风险等级 |
|------------------|------------------------------|----------|
| #1 | 🔴 #1 | Host Key 验证缺失 |
| #2 | 🔴 #2 | CVE-2025-47914 |
| #3 | 🔴 #3 | sha256sum 注入 |
| #4 | 🔴 #4 | 公钥部署注入 |
| #5 | 🔴 #5 | HTTP CONNECT 无限制 |
| #6 | 🔴 #6 | 测试构建阻断 |
| #7 | 🟠 #7 | PRAGMA SQL 注入 |
| #8 | 🟠 #8 | 七牛云明文 HTTP |
| #9 | 🟡 #9 | 加密密钥同库 |
| #10 | 🟡 #10 | 变更日志明文密码 |
| #11 | 🟡 #11 | SmartProxy.Stop() 竞态 |
| #12 | 🟡 #12 | tunnelManager 死锁 |
| #13 | 🟡 #13 | 批处理 OOM |
| #14 | 🟡 #14 | SFTP 递归限制 |
| #15 | 🟡 #15 | qiniu Pull 响应体 |
| #16 | 🟡 #16 | server.Options JSON 忽略 |
| #17 | 🟡 #17 | ECB 模式暴露 |
| #18 | 🟡 #18 | 加解密代码重复 |
| #19 | 🟡 #19 | JSON/时间解析错误 |
| #20 | 🟡 #20 | generateID fallback |

---

*本修复方案由 `docs/securitys/fix-plan.md` 和 `docs/securitys/security-audit-2026-05-19.md §10` 合并去重生成，原始文件未修改。*
