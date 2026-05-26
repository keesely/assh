# 逻辑边界审查报告

**审查日期**: 2026-05-19
**审查范围**: `asshv2/` 全部 Go 源文件（asshc/, cmd/, config/, log/）
**审查工具**: 静态代码分析 + go vet

---

## 审查摘要

| 风险等级 | 发现数量 |
|---------|---------|
| 🔴 HIGH | 3 |
| 🟡 MEDIUM | 6 |
| 🟢 LOW | 8 |

---

## 🔴 HIGH 严重问题

### H-001: 远程命令注入 — `sha256sum` + 拼接路径

**文件**: `asshc/infra/sftp/upload.go:305`, `asshc/infra/sftp/verify.go:140`

```go
out, err := session.Output("sha256sum " + remotePath)
```

**问题**: `remotePath` 直接拼接到 shell 命令中，如果路径包含 shell 元字符（`; rm -rf /`、`$(malicious)`、反引号），可在远程服务器上执行任意命令。

**风险等级**: 🔴 HIGH — 远程代码执行

**修复建议**:
```go
// 方案一：使用 filepath.Base 并 shellescape
import "github.com/kballard/go-shellquote"
safePath := shellquote.Join(remotePath)
out, err := session.Output("sha256sum " + safePath)

// 方案二：使用 printf %s 避免 shell 解释
out, err := session.Output(fmt.Sprintf("sha256sum -- %s", shellquote.Join(remotePath)))

// 方案三（推荐）：改用 SFTP 读取文件内容，本地计算 hash
// 完全避免远程命令执行
```

---

### H-002: 公钥部署中的命令注入

**文件**: `asshc/service/deploy.go:121-132`

```go
pubKeyStr := strings.ReplaceAll(string(pubKey), "'", "'\"'\"'")
cmd := fmt.Sprintf(
    `mkdir -p ~/.ssh && chmod 700 ~/.ssh && printf '%%s\n' '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`,
    pubKeyStr,
)
```

**问题**: 仅转义了单引号，但 `pubKeyStr` 中的 `$(...)` 或反引号表达式仍会被 shell 执行。如果公钥内容被恶意构造（理论上公钥格式有限，但防御不足），可能导致远程命令执行。

**风险等级**: 🔴 HIGH — 远程代码执行（需恶意公钥输入）

**修复建议**:
```go
// 方案一：通过 stdin 传入公钥，避免 shell 转义
func (s *DeployService) appendToAuthorizedKeys(session *ssh.Session, pubKey []byte) error {
    session.Stdin = bytes.NewReader(append(pubKey, '\n'))
    cmd := `mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`
    return session.Run(cmd)
}

// 方案二：使用 base64 编码传输
```

---

### H-003: HTTP CONNECT 握手无读取限制（Slowloris 攻击面）

**文件**: `asshc/infra/proxy/http_connect.go:151-205`

```go
func (p *HTTPConnectProxy) httpHandshake(conn net.Conn) (string, error) {
    br := bufio.NewReader(conn)
    reqLine, err := br.ReadString('\n')
    // ...
    for {
        line, err := br.ReadString('\n')
        // ...
        if line == "" {
            break
        }
        // ...
    }
}
```

**问题**: 读取 HTTP 头部的 `for` 循环没有任何限制：
1. 恶意客户端可以无限发送头部行（永不发送空行），耗尽服务端内存
2. 没有读取超时（`conn.SetReadDeadline`），恶意客户端可慢速发送数据（Slowloris）
3. `bufio.NewReader` 默认缓冲区 4KB，但 `ReadString` 可动态分配

**风险等级**: 🔴 HIGH — DoS 攻击

**修复建议**:
```go
func (p *HTTPConnectProxy) httpHandshake(conn net.Conn) (string, error) {
    // 设置读取超时
    conn.SetReadDeadline(time.Now().Add(30 * time.Second))
    defer conn.SetReadDeadline(time.Time{})

    br := bufio.NewReaderSize(conn, 4096) // 固定缓冲区大小
    // ...
    maxHeaders := 100
    headerCount := 0
    for {
        line, err := br.ReadString('\n')
        // ...
        headerCount++
        if headerCount > maxHeaders {
            return "", fmt.Errorf("too many headers")
        }
        // ...
    }
}
```

---

## 🟡 MEDIUM 中等问题

### M-001: SmartProxy.Stop() 不等待活跃连接完成

**文件**: `asshc/infra/proxy/listener.go:110-131`

```go
func (sp *SmartProxy) Stop() error {
    // ...
    for conn := range sp.active {
        conn.Close()
    }
    sp.active = make(map[net.Conn]struct{})
    sp.client = nil
    sp.cancel()
    return nil
    // 注意：没有 sp.wg.Wait()
}
```

**问题**: `Stop()` 关闭了所有连接并取消 context，但没有等待 `wg.Wait()`。`acceptLoop` 中的 goroutine 可能仍在处理连接，导致在 `Stop()` 返回后仍有活跃的 goroutine 访问已置 nil 的 `sp.client`。

**风险等级**: 🟡 MEDIUM — 竞态条件 / panic

**修复建议**:
```go
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

    sp.wg.Wait() // 等待所有 goroutine 完成
    return nil
}
```

---

### M-002: tunnelManager.Close() 可能永久阻塞

**文件**: `asshc/infra/proxy/tunnel_manager.go:241-245`

```go
func (m *tunnelManager) Close() {
    m.cancel()
    <-m.done
}
```

**问题**: `m.done` channel 只在 `reconnectLoop` 的 defer 中被 close。如果 `reconnectLoop` 从未被调用（正常直连模式下不会调用），`Close()` 会永久阻塞在 `<-m.done`。

**风险等级**: 🟡 MEDIUM — 死锁

**修复建议**:
```go
func (m *tunnelManager) Close() {
    m.cancel()
    select {
    case <-m.done:
    case <-time.After(5 * time.Second):
        log.Warnf("tunnel manager close timeout")
    }
}
```
或者在 `NewTunnelManager()` 中初始化时预关闭 channel，并在需要时用 `sync.Once` 保护。

---

### M-003: 批处理命令无输出大小限制

**文件**: `cmd/connect.go:298-360`

```go
var allResults []bcServerResult
// ...
go func(n string) {
    output, runErr := a.connectSvc.RunWithOutput(client, cmd)
    // ...
    resultsMu.Lock()
    allResults = append(allResults, result) // result.Stdout 可能非常大
    resultsMu.Unlock()
}(name)
```

**问题**: 并发对所有服务器执行命令并将完整输出存入 `allResults`。如果命令输出巨大（如 `cat /dev/urandom`）或服务器数量极大，可能导致 OOM。

**风险等级**: 🟡 MEDIUM — 内存溢出

**修复建议**:
```go
// 1. 限制输出大小
const maxOutputSize = 10 * 1024 * 1024 // 10MB
if len(output) > maxOutputSize {
    output = output[:maxOutputSize] + "\n... (truncated)"
}

// 2. 限制并发数
sem := make(chan struct{}, 10) // 最多10个并发
```

---

### M-004: SFTP 递归遍历无深度限制

**文件**: `asshc/infra/sftp/upload.go:232-254`, `asshc/infra/sftp/download.go:205-227`

```go
func collectLocalFiles(baseDir, currentDir string) ([]string, error) {
    // ...
    for _, entry := range entries {
        path := filepath.Join(currentDir, entry.Name())
        if entry.IsDir() {
            subFiles, err := collectLocalFiles(baseDir, path) // 无限递归
            // ...
```

**问题**: 递归目录遍历没有深度限制。虽然 `os.ReadDir` 不跟踪符号链接（避免循环），但如果目录结构极深（恶意构造），可能导致栈溢出。同时收集的文件列表没有数量限制。

**风险等级**: 🟡 MEDIUM — 栈溢出 / OOM

**修复建议**:
```go
const maxDepth = 100
const maxFiles = 100000

func collectLocalFiles(baseDir, currentDir string, depth int) ([]string, error) {
    if depth > maxDepth {
        return nil, fmt.Errorf("max directory depth exceeded")
    }
    // ...
    subFiles, err := collectLocalFiles(baseDir, path, depth+1)
    // ...
}
```

---

### M-005: qiniu Pull 响应体无大小限制

**文件**: `asshc/infra/sync/qiniu/qiniu.go:150-155`

```go
buf := new(bytes.Buffer)
_, err = buf.ReadFrom(resp.Body)
```

**问题**: `ReadFrom` 读取 HTTP 响应体到内存中，没有大小限制。如果云端数据被篡改或返回异常大文件，可能导致 OOM。

**风险等级**: 🟡 MEDIUM — 内存溢出

**修复建议**:
```go
const maxSyncDataSize = 50 * 1024 * 1024 // 50MB
limitedReader := io.LimitReader(resp.Body, maxSyncDataSize)
buf := new(bytes.Buffer)
_, err = buf.ReadFrom(limitedReader)
if int64(buf.Len()) >= maxSyncDataSize {
    return nil, fmt.Errorf("sync data exceeds maximum size")
}
```

---

### M-006: server.Options JSON 反序列化错误被静默忽略

**文件**: `asshc/infra/store/server.go:464-466`

```go
if optionsJSON != "" && optionsJSON != "{}" {
    json.Unmarshal([]byte(optionsJSON), &server.Options)
}
```

**问题**: `json.Unmarshal` 的错误被静默忽略。如果数据库中存在损坏的 JSON 数据，`server.Options` 会保持为初始化的空 map，导致配置丢失且无任何日志。

**风险等级**: 🟡 MEDIUM — 数据丢失/静默失败

**修复建议**:
```go
if optionsJSON != "" && optionsJSON != "{}" {
    if err := json.Unmarshal([]byte(optionsJSON), &server.Options); err != nil {
        log.Warnf("failed to parse options for server %s.%s: %v", group, name, err)
    }
}
```

---

## 🟢 LOW 低风险问题

### L-001: strconv.Atoi 错误被忽略

**文件**: `asshc/service/connect.go:166,209`

```go
port, _ = strconv.Atoi(portStr)
```

**问题**: 端口解析失败时 port 为 0，后续 `Connect` 会尝试连接端口 0。

**修复建议**: 检查错误，返回明确的错误信息。

---

### L-002: SyncHistory 记录失败被静默忽略

**文件**: `asshc/service/sync.go:167,177,209,229,352`

```go
_ = s.store.RecordSyncHistory(&domain.SyncHistory{...})
```

**问题**: 同步历史记录失败被忽略。不影响核心功能，但会导致审计日志缺失。

---

### L-003: generateID fallback 输出指针地址

**文件**: `asshc/infra/proxy/port.go:256-257`

```go
if _, err := rand.Read(b); err != nil {
    return fmt.Sprintf("tunnel-%d", &b) // 输出指针地址，不是有意义的 ID
}
```

**修复建议**: 使用 `time.Now().UnixNano()` 或进程 PID 作为 fallback。

---

### L-004: qiniu SrcUpHosts 可能为空切片

**文件**: `asshc/infra/sync/qiniu/qiniu.go:136`

```go
domain := s.cfg.Zone.SrcUpHosts[0]
```

**问题**: 如果 `SrcUpHosts` 为空，会 panic（index out of range）。

**修复建议**:
```go
if len(s.cfg.Zone.SrcUpHosts) == 0 {
    return nil, fmt.Errorf("qiniu: no upload hosts available")
}
domain := s.cfg.Zone.SrcUpHosts[0]
```

---

### L-005: keepAlive goroutine 仅靠 SendRequest 失败退出

**文件**: `asshc/infra/ssh/client.go:103-113`

```go
func (c *Connector) keepAlive(client *ssh.Client, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for range ticker.C {
        _, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
        if err != nil {
            return
        }
    }
}
```

**问题**: 如果 SSH 客户端被关闭但 `SendRequest` 不报错（极端情况），goroutine 不会退出。`client.Close()` 调用后 ticker 仍在运行。

**修复建议**: 增加 context 或 done channel 支持主动退出。

---

### L-006: writeError 不检查写入错误

**文件**: `asshc/infra/proxy/http_connect.go:255-257`

```go
func (p *HTTPConnectProxy) writeError(conn net.Conn, statusCode, statusText string) {
    fmt.Fprintf(conn, "HTTP/1.1 %s %s\r\n\r\n", statusCode, statusText)
}
```

**问题**: 写入错误被忽略。连接已关闭时写入会失败，但忽略此错误是可接受的（best-effort）。

---

### L-007: DeployService readAuthorizedKeys session 未 defer Close

**文件**: `asshc/service/deploy.go:67-74`

```go
session, err := client.NewSession()
// ...
authorizedKeys, err := s.readAuthorizedKeys(session)
session.Close() // 手动关闭，非 defer
```

**问题**: 如果 `readAuthorizedKeys` 和 `session.Close()` 之间发生 panic，session 不会被关闭。

**修复建议**: 使用 `defer session.Close()`。

---

### L-008: 测试代码与接口签名不匹配

**文件**: `asshc/service/connect_test.go:63`

```
go vet: cannot use &mockConnector{} as port.SSHConnector value:
    have ConnectChain(*domain.Server, []*domain.Server) (interface{}, error)
    want ConnectChain(*domain.Server, []*domain.Server) (*ssh.Client, error)
```

**问题**: mock 的 `ConnectChain` 返回 `interface{}` 而非 `*ssh.Client`，不满足接口定义。

**修复建议**: 更新 mock 使返回类型匹配。

---

## 架构级安全建议

### 1. 远程命令执行模式需要统一安全处理

当前代码中有 4 处通过 SSH 执行 shell 命令且拼接了外部输入：
- `upload.go:305` — `sha256sum` + path
- `verify.go:140` — `sha256sum` + path
- `deploy.go:130` — `printf` + pubKey
- `session.go:89` — 通用 `Run(cmd)` （由调用方负责安全）

**建议**: 创建统一的 shell escape 工具函数，或改用 SFTP 文件操作 + 本地计算避免远程命令执行。

### 2. 代理组件需要统一的资源生命周期管理

SOCKS5Proxy、HTTPConnectProxy、SmartProxy、LocalForward、RemoteForward 都有各自的 `active map` 和锁机制。建议抽象为统一的 `ConnectionPool` 接口，确保一致的资源管理行为。

### 3. HTTP 代理需要安全加固

- 添加 TLS 支持（当前明文传输代理认证凭据）
- 添加连接速率限制
- 添加每连接读写超时

---

*报告生成工具: opencode (mi/mimo-v2.5-pro)*
