# ASSH v2.0.0 综合安全审查报告

**审查日期**: 2026-05-19  
**审查范围**: `asshv2/` 全量 Go 源码  
**审查工具**: 静态分析 (`go vet`)、数据竞争检测 (`go test -race`)、OSV 漏洞数据库、手动代码审计、逻辑边界分析  
**报告版本**: 1.0

---

## 目录

1. [执行摘要](#1-执行摘要)
2. [依赖项安全审查](#2-依赖项安全审查)
3. [机密扫描审查](#3-机密扫描审查)
4. [代码扫描审查（逻辑漏洞）](#4-代码扫描审查逻辑漏洞)
5. [代码质量审查](#5-代码质量审查)
6. [存储加密质量审查](#6-存储加密质量审查)
7. [数据竞争检测](#7-数据竞争检测)
8. [逻辑边界审查](#8-逻辑边界审查)
9. [风险分级汇总](#9-风险分级汇总)
10. [修复方案](#10-修复方案)

---

## 1. 执行摘要

本次安全审查对 `asshv2/` 目录下的 Go 代码进行了全面的安全评估，涵盖依赖项安全、机密扫描、代码扫描、代码质量、存储加密质量、数据竞争检测和逻辑边界审查七个方面。

### 关键发现

| 风险等级 | 数量 | 主要问题 |
|----------|------|----------|
| 🔴 **严重** | 6 | Host Key 验证缺失、CVE-2025-47914、远程命令注入、HTTP 慢速攻击 |
| 🟠 **高** | 2 | PRAGMA SQL 注入模式、七牛云明文 HTTP |
| 🟡 **中** | 12 | 加密密钥同库存储、变更日志明文密码、内存溢出风险等 |
| 🟢 **低** | 11 | 错误忽略、资源泄漏风险等 |

### 立即行动项

1. **升级 `golang.org/x/crypto`** 至 v0.35.0+ 以修复 CVE-2025-47914
2. **修复 Host Key 验证**逻辑倒置问题
3. **修复远程命令注入**漏洞（sha256sum、公钥部署）
4. **修复 HTTP CONNECT 握手**无限制问题

---

## 2. 依赖项安全审查

### 2.1 已知漏洞

| 依赖 | 版本 | 漏洞 ID | 严重度 | 说明 |
|------|------|---------|--------|------|
| `golang.org/x/crypto` | `v0.25.0` | **CVE-2025-47914** (GHSA-f6x5-jh6r-wrfv) | **中** (MODERATE) | SSH Agent 服务在处理畸形消息时存在越界读取，可导致 panic |
| `modernc.org/sqlite` | `v1.50.0` | 无 | - | - |
| `github.com/pkg/sftp` | `v1.13.7` | 无 | - | - |
| `github.com/rs/zerolog` | `v1.32.0` | 无 | - | - |
| `github.com/urfave/cli` | `v1.22.2` | 无 | - | - |
| `github.com/qiniu/api.v7/v7` | `v7.4.0` | 无 | - | - |

> **建议**: `golang.org/x/crypto` 至少升级到 `v0.35.0` 以修复 CVE-2025-47914。

### 2.2 依赖版本状态

当前 Go 版本要求为 `go 1.25.0`（`go.mod`），属于较新版本，编译链本身安全性良好。

依赖项均为稳定版本，无 deprecated 依赖。

---

## 3. 机密扫描审查

### 3.1 硬编码凭据

- **未发现** 明显的硬编码 API 密钥、密码或令牌。

### 3.2 敏感信息管理

| 敏感数据类型 | 存储位置 | 保护措施 |
|-------------|---------|----------|
| SSH 密码 | SQLite `servers.password_encrypted` | AES-GCM 加密（12B nonce + 密文 + 16B tag） |
| 云账户 SecretKey | SQLite `cloud_accounts.secret_key` | AES-GCM 加密 |
| 跳板链路径数据 | SQLite `jump_history.path_data` | AES-GCM 加密 |
| AES 加密密钥 | SQLite `config` 表（Base64 编码） | **和加密数据存储在同一个数据库中** |
| 后端私钥 | 文件系统 `data/keys/` | 可选 account passphrase 加密 |

### 3.3 密钥管理

| 发现 | 风险等级 | 说明 |
|------|---------|------|
| 加密密钥与密文同库存储 | **⚠ 中** | AES-GCM 密钥存储在同一个 SQLite 数据库的 `config` 表中。任何能读取数据库文件的人都可以获取密钥并解密所有数据 |
| 无密钥轮换机制 | **⚠ 低** | 密钥一旦生成，除非手动删除数据库重新生成，否则不会自动轮换 |
| 备份密钥解密 | **ℹ 信息** | `Connector.accountPassphrase` 用于解密 `data/keys/` 中的备份密钥，属于合理设计 |

---

## 4. 代码扫描审查（逻辑漏洞）

### 4.1 🔴 严重：Host Key 验证完全缺失

**文件**: `asshc/infra/ssh/client.go:362-369`

```go
func (c *Connector) getHostKeyCallback(server *domain.Server) ssh.HostKeyCallback {
    if skip, ok := server.Options["insecure_skip_hostkey"]; ok {
        if v, ok := skip.(bool); ok && v {
            return ssh.InsecureIgnoreHostKey()
        }
    }
    return ssh.InsecureIgnoreHostKey()  // ← 始终返回不校验！
}
```

**问题**: 
- `getHostKeyCallback()` 即使 `insecure_skip_hostkey` 选项未设置或为 `false`，也默认返回 `InsecureIgnoreHostKey()`
- 逻辑倒置：应默认校验，仅当选项为 `true` 时跳过
- `KnownHostsChecker.Check()` (`hostkey.go:21`) 始终返回 `nil`，是纯粹的占位实现

**影响**: 无法防范中间人（MitM）攻击。任何能拦截 SSH 连接的人可以伪造服务器身份。

**修复建议**: 
1. 实现 `known_hosts` 文件校验
2. 将逻辑修正为：默认校验 → 选项 `insecure_skip_hostkey: true` 时跳过

### 4.2 🔴 严重：CVE-2025-47914 SSH Agent 漏洞

**直接影响包**: `golang.org/x/crypto v0.25.0`  
**影响文件**: `asshc/infra/ssh/client.go:243-264`（tryAgent 使用 SSH Agent）

在 `tryAgent()` 中，代码连接 SSH Agent (`SSH_AUTH_SOCK`) 并调用 `agentClient.Signers()`。CVE-2025-47914 表明当 SSH Agent 返回畸形消息时可能导致越界读取和 panic。

**修复建议**: 升级 `golang.org/x/crypto` 到 `v0.35.0` 或更高。

### 4.3 🟠 高：PRAGMA SQL 注入漏洞

**文件**: `asshc/infra/store/db.go:168`

```go
rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
```

**问题**: 表名通过 `fmt.Sprintf` 直接拼接到 SQL 中。尽管 `columnExists()` 当前仅由内部 `migrate()` 调用且 `table` 参数硬编码，该模式仍然是 SQL 注入风险。如果未来有代码路径将用户输入传递给 `table` 参数，后果严重。

**当前利用难度**: 低（参数不可控）  
**风险**: 未来扩展可能引入此漏洞  

**修复建议**: 使用参数化查询的 PRAGMA（SQLite 不支持 `PRAGMA table_info(?)` 参数化），或对表名做白名单校验。

### 4.4 🟠 高：七牛云同步使用明文 HTTP

**文件**: `asshc/infra/sync/qiniu/qiniu.go`

```go
// 第68行
s.cfg = storage.Config{
    Zone:          zone,
    UseHTTPS:      false,     // ← 明文传输
    UseCdnDomains: false,
}

// 第139行
downloadURL := fmt.Sprintf("http://%s/%s", domain, syncFileName)  // ← HTTP!
```

**问题**: 
- Pull 操作通过 HTTP（非 HTTPS）下载同步数据
- 同步数据包含完整的服务器配置（名称、主机、端口、用户、加密密码）
- 网络路径上的攻击者可拦截或篡改同步数据

**影响**: 中间人攻击者可获取完整的服务器配置信息。

**修复建议**: 
- 设置 `UseHTTPS: true`
- Pull 使用 HTTPS URL

### 4.5 🟡 中：变更日志快照包含明文密码

**文件**: `asshc/infra/store/server.go:112-116`

```go
// 序列化全量快照（密码以明文保存，用于回滚恢复）
snapshotJSON, err := json.Marshal(server)
```

**问题**: `server` 对象的 `Auth.Password` 在变更日志快照中以明文 JSON 存储。所有历史版本的密码快照均可从 `server_changelog` 表中读取。

**缓解因素**: 变更日志表与服务器表在同一数据库中，受相同访问控制保护。  
**影响**: 同一数据库泄露会导致所有历史密码暴露，而不仅是当前密码。

### 4.6 🟡 中：generateID 回退路径缺陷

**文件**: `asshc/infra/proxy/port.go:253-260`

```go
func generateID() string {
    b := make([]byte, 8)
    if _, err := rand.Read(b); err != nil {
        // 极低概率 fallback
        return fmt.Sprintf("tunnel-%d", &b)   // ← 打印的是内存地址！
    }
    return fmt.Sprintf("%x", b)
}
```

**问题**: `fmt.Sprintf("tunnel-%d", &b)` 输出的是 slice header 的内存地址（指针），不是可用的 ID。如果 `crypto/rand.Read` 失败（极低概率），生成的 ID 毫无意义且不唯一。

### 4.7 🟡 中：多个 JSON Unmarshal 错误被忽略

| 文件位置 | 代码 | 风险 |
|---------|------|------|
| `server.go:465` | `json.Unmarshal([]byte(optionsJSON), &server.Options)` | 错误静默忽略，解析失败后 Options 保持为 `make(map[string]interface{})` |
| `jump.go:74` | `json.Unmarshal([]byte(decrypted), &jh.PathData)` | 解密后的 JSON 解析错误被静默忽略 |
| `jump.go:102` | `json.Unmarshal([]byte(decrypted), &jh.PathData)` | 同上 |

### 4.8 🟡 中：Daemonize 进程 fork 风险

**文件**: `asshc/infra/proxy/tunnel_manager.go:249-266`

```go
func Daemonize() error {
    exe, err := os.Executable()
    proc, err := os.StartProcess(exe, os.Args, &os.ProcAttr{...})
    os.Exit(0)
}
```

**问题**: 以当前命令行参数完整重新启动进程。如果 `os.StartProcess` 失败，`os.Exit(0)` 已执行，父进程不会收到错误。没有对调用频率做任何限制。

### 4.9 🟢 低：多处时间解析错误忽略

| 文件位置 | 代码 |
|---------|------|
| `account.go:212` | `acct.CreatedAt, _ = time.Parse(...)` |
| `account.go:213` | `acct.UpdatedAt, _ = time.Parse(...)` |
| `account.go:229` | `t, _ := time.Parse(...)` |

时间格式不匹配时将使用零值时间，下游可能产生意外行为。

---

## 5. 代码质量审查

### 5.1 🔴 构建阻断：测试接口不匹配

**文件**: `asshc/service/connect_test.go:63`  
**错误**: 
```
cannot use &mockConnector as port.SSHConnector: 
  have ConnectChain(*domain.Server, []*domain.Server) (interface{}, error)
  want ConnectChain(*domain.Server, []*domain.Server) (*ssh.Client, error)
```

**影响**: 
- `go vet ./...` 输出错误
- `go test -race ./...` 中 `asshc/service` 包构建失败
- 该包的 16 个测试用例、数据竞争检测全部无法运行

**修复**: mock 的 `ConnectChain` 返回类型需改为 `(*ssh.Client, error)`。

### 5.2 🟡 中：AES ECB 模式公开暴露

**文件**: `asshc/infra/crypto/aes.go:52,74,199-224`

ECB 模式通过 `AESEncrypt`/`AESDecrypt` 公共 API 暴露。虽然已有注释标明"不安全的，仅用于兼容旧系统"，但它在公开 API 中仍然可选，存在被误用的风险。

### 5.3 🟡 中：加解密代码重复

`key.go:69-140` 中的 `encryptPassword`/`decryptPassword` 与 `account.go:242-314` 中的 `encryptData`/`decryptData` 几乎完全相同的 AES-GCM 加解密逻辑。共有 **3 份** 相同的 AES-GCM 实现（key.go, account.go, aes.go 中的 GCM 模式）。

### 5.4 🟡 中：tunnel_manager.Close() 可能永久阻塞

**文件**: `asshc/infra/proxy/tunnel_manager.go:242-245`

```go
func (m *tunnelManager) Close() {
    m.cancel()
    <-m.done  // 如果 reconnectLoop 从未启动，将永久阻塞
}
```

`done` channel 仅在 `reconnectLoop()` 退出时关闭。如果从未调用过 `SetReconnect()`，则 `reconnectLoop` 未启动，`Close()` 将永远阻塞。

### 5.5 🟢 低：conn.Write 错误被忽略

多个代理协议处理函数中，`conn.Write()` 的返回值被忽略：
- `socks5.go:237,284` — 认证失败响应写入
- `http_connect.go:256,261` — HTTP 错误响应写入

在代理场景中，写入错误通常是连接已断开，影响较小但仍不符合最佳实践。

---

## 6. 存储加密质量审查

### 6.1 加密方案总评

| 方面 | 评价 |
|------|------|
| 加密算法 | ✅ AES-256-GCM（行业标准认证加密） |
| 随机数生成 | ✅ `crypto/rand.Reader`（密码学安全） |
| Nonce 使用 | ✅ 每次加密生成独立随机 nonce（12 字节） |
| 认证标签 | ✅ GCM 提供 16 字节认证标签，防止篡改 |
| 附加数据 (AD) | ⚠️ 所有 Seal/Open 调用使用 `nil` AD，无上下文绑定 |
| 密钥生成 | ⚠️ 32 字节随机——无 KDF（Argon2/PBKDF2） |
| 密钥存储 | ⚠️ 与密文同库——架构性弱点 |
| 提权路径 | ⚠️ 解密函数 `decryptPassword`/`decryptData` 在 `Store` 的多个方法中可用，无独立鉴权 |

### 6.2 关键发现

**Nonce 复用风险**: ✅ 正确。每次加密调用都会生成新的随机 nonce，没有 nonce 复用风险。

**IV/Nonce 长度**: ✅ GCM 使用 12 字节 standard nonce，CBC 使用 16 字节 IV，均正确。

**AEAD 使用正确性**: ✅ `gcm.Seal(nonce, nonce, plain, nil)` — 使用 nonce 作为 Seal 的前缀参数，正确。

**密钥存储安全性**: ⚠️ 加密密钥（`Store.cryptoKey`）与加密数据（`password_encrypted`, `secret_key` 列）存储在**同一个 SQLite 文件中**。数据库文件泄露 = 所有加密数据泄露。这是一个**架构层面的设计限制**，而非实现缺陷。

### 6.3 改进建议

1. **密钥与数据分离**: 将加密密钥存储在操作系统密钥链（macOS Keychain）、环境变量或独立配置文件中
2. **引入 KDF**: 若需要密码派生密钥，使用 Argon2id 替代直接随机密钥
3. **上下文绑定**: 将 `additionalData` 设置为表名+列名，绑定密文到特定上下文
4. **密钥轮换**: 实现定期密钥轮换机制和重加密功能

---

## 7. 数据竞争检测

### 7.1 测试结果

```
包                             结果
assh/asshc/domain               ✅ 通过 (6.191s)
assh/asshc/infra/keymgr         ✅ 通过 (21.137s)
assh/asshc/infra/proxy          ✅ 通过 (4.849s)
assh/asshc/infra/ssh            ✅ 通过 (4.426s)
assh/asshc/infra/store          ✅ 通过 (5.964s)
assh/log                        ✅ 通过 (5.674s)
assh/asshc/service              ❌ 构建失败（接口不匹配导致无法运行 race 测试）
```

### 7.2 并发安全分析

| 组件 | 保护机制 | 评估 |
|------|---------|------|
| `Store` (SQLite) | `sync.RWMutex` | ✅ 读写分离，所有公开方法正确加锁 |
| `SOCKS5Proxy` | `sync.Mutex` | ✅ active map 和 listener 在锁保护下操作 |
| `HTTPConnectProxy` | `sync.Mutex` | ✅ 同上 |
| `LocalForward`/`RemoteForward` | `sync.Mutex` | ✅ 同上 |
| `tunnelManager` | `sync.Mutex` | ✅ 正确保护 forwards map |
| `SmartProxy` | `sync.Mutex` + `sync.WaitGroup` + `atomic` | ✅ sessionCounter 使用 atomic，active map 使用 mutex |
| `bidirectionalCopy` | `sync.WaitGroup` | ✅ 正确同步两个 goroutine 退出 |
| `RuleEngine` | `sync.RWMutex` | ✅ 读操作使用 RLock |

**结论**: 已测试的 5 个包全部通过 data race 检测。当前实现的并发安全模式正确。

### 7.3 潜在风险点

- `asshc/service` 包因构建失败未通过 race 检测，其中的 goroutine 使用（如 `health.go:109` 的并发健康检查）未经验证

---

## 8. 逻辑边界审查

### 8.1 审查摘要

| 风险等级 | 发现数量 |
|---------|---------|
| 🔴 HIGH | 3 |
| 🟡 MEDIUM | 6 |
| 🟢 LOW | 8 |

### 8.2 🔴 HIGH 严重问题

#### H-001: 远程命令注入 — `sha256sum` + 拼接路径

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

#### H-002: 公钥部署中的命令注入

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

#### H-003: HTTP CONNECT 握手无读取限制（Slowloris 攻击面）

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

### 8.3 🟡 MEDIUM 中等问题

#### M-001: SmartProxy.Stop() 不等待活跃连接完成

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

#### M-002: tunnelManager.Close() 可能永久阻塞

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

#### M-003: 批处理命令无输出大小限制

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

#### M-004: SFTP 递归遍历无深度限制

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

#### M-005: qiniu Pull 响应体无大小限制

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

#### M-006: server.Options JSON 反序列化错误被静默忽略

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

### 8.4 🟢 LOW 低风险问题

#### L-001: strconv.Atoi 错误被忽略

**文件**: `asshc/service/connect.go:166,209`

```go
port, _ = strconv.Atoi(portStr)
```

**问题**: 端口解析失败时 port 为 0，后续 `Connect` 会尝试连接端口 0。

**修复建议**: 检查错误，返回明确的错误信息。

#### L-002: SyncHistory 记录失败被静默忽略

**文件**: `asshc/service/sync.go:167,177,209,229,352`

```go
_ = s.store.RecordSyncHistory(&domain.SyncHistory{...})
```

**问题**: 同步历史记录失败被忽略。不影响核心功能，但会导致审计日志缺失。

#### L-003: generateID fallback 输出指针地址

**文件**: `asshc/infra/proxy/port.go:256-257`

```go
if _, err := rand.Read(b); err != nil {
    return fmt.Sprintf("tunnel-%d", &b) // 输出指针地址，不是有意义的 ID
}
```

**修复建议**: 使用 `time.Now().UnixNano()` 或进程 PID 作为 fallback。

#### L-004: qiniu SrcUpHosts 可能为空切片

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

#### L-005: keepAlive goroutine 仅靠 SendRequest 失败退出

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

#### L-006: writeError 不检查写入错误

**文件**: `asshc/infra/proxy/http_connect.go:255-257`

```go
func (p *HTTPConnectProxy) writeError(conn net.Conn, statusCode, statusText string) {
    fmt.Fprintf(conn, "HTTP/1.1 %s %s\r\n\r\n", statusCode, statusText)
}
```

**问题**: 写入错误被忽略。连接已关闭时写入会失败，但忽略此错误是可接受的（best-effort）。

#### L-007: DeployService readAuthorizedKeys session 未 defer Close

**文件**: `asshc/service/deploy.go:67-74`

```go
session, err := client.NewSession()
// ...
authorizedKeys, err := s.readAuthorizedKeys(session)
session.Close() // 手动关闭，非 defer
```

**问题**: 如果 `readAuthorizedKeys` 和 `session.Close()` 之间发生 panic，session 不会被关闭。

**修复建议**: 使用 `defer session.Close()`。

#### L-008: 测试代码与接口签名不匹配

**文件**: `asshc/service/connect_test.go:63`

```
go vet: cannot use &mockConnector{} as port.SSHConnector value:
    have ConnectChain(*domain.Server, []*domain.Server) (interface{}, error)
    want ConnectChain(*domain.Server, []*domain.Server) (*ssh.Client, error)
```

**问题**: mock 的 `ConnectChain` 返回 `interface{}` 而非 `*ssh.Client`，不满足接口定义。

**修复建议**: 更新 mock 使返回类型匹配。

### 8.5 架构级安全建议

#### 1. 远程命令执行模式需要统一安全处理

当前代码中有 4 处通过 SSH 执行 shell 命令且拼接了外部输入：
- `upload.go:305` — `sha256sum` + path
- `verify.go:140` — `sha256sum` + path
- `deploy.go:130` — `printf` + pubKey
- `session.go:89` — 通用 `Run(cmd)` （由调用方负责安全）

**建议**: 创建统一的 shell escape 工具函数，或改用 SFTP 文件操作 + 本地计算避免远程命令执行。

#### 2. 代理组件需要统一的资源生命周期管理

SOCKS5Proxy、HTTPConnectProxy、SmartProxy、LocalForward、RemoteForward 都有各自的 `active map` 和锁机制。建议抽象为统一的 `ConnectionPool` 接口，确保一致的资源管理行为。

#### 3. HTTP 代理需要安全加固

- 添加 TLS 支持（当前明文传输代理认证凭据）
- 添加连接速率限制
- 添加每连接读写超时

---

## 9. 风险分级汇总

### 🔴 严重 (6)

| # | 发现 | 文件位置 | 来源 |
|---|------|---------|------|
| 1 | Host Key 验证完全缺失（默认 MitM 无防护） | `client.go:362-369` | 代码扫描 |
| 2 | CVE-2025-47914 SSH Agent 越界读漏洞 | `go.mod` → `x/crypto v0.25.0` | 依赖项安全 |
| 3 | 测试接口不匹配导致构建阻断，Race 检测无法覆盖 | `connect_test.go:63` | 代码质量 |
| 4 | 远程命令注入 — `sha256sum` + 拼接路径 | `upload.go:305`, `verify.go:140` | 逻辑边界 |
| 5 | 公钥部署中的命令注入 | `deploy.go:121-132` | 逻辑边界 |
| 6 | HTTP CONNECT 握手无读取限制（Slowloris 攻击面） | `http_connect.go:151-205` | 逻辑边界 |

### 🟠 高 (2)

| # | 发现 | 文件位置 | 来源 |
|---|------|---------|------|
| 7 | PRAGMA SQL 注入模式（只读，当前不可利用） | `db.go:168` | 代码扫描 |
| 8 | 七牛云同步使用明文 HTTP | `qiniu.go:68,139` | 代码扫描 |

### 🟡 中 (12)

| # | 发现 | 文件位置 | 来源 |
|---|------|---------|------|
| 9 | 加密密钥与密文同库存储 | `key.go` / `db.go` | 存储加密 |
| 10 | 变更日志快照包含明文密码 | `server.go:112-116` | 代码扫描 |
| 11 | generateID fallback 输出内存地址 | `port.go:257` | 代码扫描 |
| 12 | 多处 JSON/时间解析错误静默忽略 | `server.go:465`, `jump.go:74,102`, `account.go:212-229` | 代码扫描 |
| 13 | AES ECB 模式公开暴露（虽已标注不安全） | `aes.go:52,199-224` | 代码质量 |
| 14 | tunnel_manager.Close() 可能永久阻塞 | `tunnel_manager.go:242-245` | 代码质量/逻辑边界 |
| 15 | SmartProxy.Stop() 不等待活跃连接完成 | `listener.go:110-131` | 逻辑边界 |
| 16 | 批处理命令无输出大小限制 | `connect.go:298-360` | 逻辑边界 |
| 17 | SFTP 递归遍历无深度限制 | `upload.go:232-254`, `download.go:205-227` | 逻辑边界 |
| 18 | qiniu Pull 响应体无大小限制 | `qiniu.go:150-155` | 逻辑边界 |
| 19 | server.Options JSON 反序列化错误被静默忽略 | `server.go:464-466` | 逻辑边界 |
| 20 | 加解密代码重复（3 份 AES-GCM 实现） | `key.go`, `account.go`, `aes.go` | 代码质量 |

### 🟢 低 (11)

| # | 发现 | 文件位置 | 来源 |
|---|------|---------|------|
| 21 | conn.Write 错误被忽略（代理响应） | `socks5.go`, `http_connect.go` | 代码质量 |
| 22 | Daemonize 进程无限制 fork | `tunnel_manager.go:249-266` | 代码扫描 |
| 23 | AES-GCM 无 additionalData 上下文绑定 | `key.go:103`, `account.go:278` | 存储加密 |
| 24 | strconv.Atoi 错误被忽略 | `connect.go:166,209` | 逻辑边界 |
| 25 | SyncHistory 记录失败被静默忽略 | `sync.go:167,177,209,229,352` | 逻辑边界 |
| 26 | generateID fallback 输出指针地址 | `port.go:256-257` | 逻辑边界 |
| 27 | qiniu SrcUpHosts 可能为空切片 | `qiniu.go:136` | 逻辑边界 |
| 28 | keepAlive goroutine 仅靠 SendRequest 失败退出 | `client.go:103-113` | 逻辑边界 |
| 29 | writeError 不检查写入错误 | `http_connect.go:255-257` | 逻辑边界 |
| 30 | DeployService readAuthorizedKeys session 未 defer Close | `deploy.go:67-74` | 逻辑边界 |
| 31 | 测试代码与接口签名不匹配 | `connect_test.go:63` | 逻辑边界 |

---

## 10. 修复方案

### 10.1 立即修复（P0）— 安全关键

| 优先级 | 问题 | 修复方案 | 预计工作量 |
|--------|------|----------|-----------|
| P0-1 | CVE-2025-47914 | 升级 `golang.org/x/crypto` 到 v0.35.0+ | 0.5h |
| P0-2 | Host Key 验证缺失 | 修复 `getHostKeyCallback()` 逻辑，实现 `known_hosts` 校验 | 4h |
| P0-3 | 远程命令注入 (sha256sum) | 改用 SFTP 读取 + 本地计算 hash，或使用 shellquote | 2h |
| P0-4 | 公钥部署命令注入 | 改用 stdin 传入公钥，避免 shell 转义 | 1h |
| P0-5 | HTTP CONNECT 无限制 | 添加读取超时、头部数量限制、请求体大小限制 | 2h |
| P0-6 | 测试构建阻断 | 修复 mock 接口返回类型 | 0.5h |

**总计**: ~10h

### 10.2 短期修复（P1）— 高风险

| 优先级 | 问题 | 修复方案 | 预计工作量 |
|--------|------|----------|-----------|
| P1-1 | 七牛云明文 HTTP | 设置 `UseHTTPS: true`，Pull 使用 HTTPS URL | 1h |
| P1-2 | PRAGMA SQL 注入 | 对表名做白名单校验 | 1h |
| P1-3 | SmartProxy.Stop() 竞态 | 添加 `wg.Wait()` 等待 goroutine 完成 | 1h |
| P1-4 | tunnelManager.Close() 死锁 | 添加超时机制或预关闭 channel | 1h |
| P1-5 | 批处理命令 OOM | 限制输出大小和并发数 | 2h |
| P1-6 | SFTP 递归深度限制 | 添加深度和文件数量限制 | 1h |
| P1-7 | qiniu Pull 响应体限制 | 使用 `io.LimitReader` | 0.5h |

**总计**: ~7.5h

### 10.3 中期修复（P2）— 中风险

| 优先级 | 问题 | 修复方案 | 预计工作量 |
|--------|------|----------|-----------|
| P2-1 | 加密密钥同库存储 | 将密钥迁移到操作系统密钥链或独立配置文件 | 8h |
| P2-2 | 变更日志明文密码 | 对快照中的密码字段单独加密 | 4h |
| P2-3 | AES ECB 模式暴露 | 移除公开 API 或标记为 deprecated | 2h |
| P2-4 | 加解密代码重复 | 统一为单一 AES-GCM 实现 | 4h |
| P2-5 | JSON/时间解析错误 | 添加日志记录 | 2h |
| P2-6 | generateID fallback | 使用时间戳或 PID 作为 fallback | 0.5h |

**总计**: ~20.5h

### 10.4 长期改进（P3）— 低风险/架构优化

| 优先级 | 问题 | 修复方案 | 预计工作量 |
|--------|------|----------|-----------|
| P3-1 | 密钥轮换机制 | 实现定期自动密钥轮换和重加密 | 16h |
| P3-2 | Known Hosts 完整实现 | 支持 `known_hosts` 文件管理和指纹验证 | 8h |
| P3-3 | 代理组件统一抽象 | 创建 `ConnectionPool` 接口 | 12h |
| P3-4 | HTTP 代理 TLS 支持 | 添加 TLS 加密和证书管理 | 8h |
| P3-5 | 统一 shell escape 工具 | 创建安全的命令构建工具包 | 4h |
| P3-6 | 资源泄漏修复 | 添加 defer Close、context 取消等 | 4h |

**总计**: ~52h

---

## 附录

### A. 审查工具

- `go vet ./...` — 静态分析
- `go test -race ./...` — 数据竞争检测
- OSV 漏洞数据库 — 依赖项安全检查
- 手动代码审计 — 逻辑漏洞、加密质量、边界检查

### B. 相关文档

- [原始安全审查报告](security-review-2026-05-19.md)
- [逻辑边界审查报告](LOGIC_BOUNDARY_AUDIT.md)
- [详细修复方案](fix-plan.md)
- [项目架构文档](../architecture.md)
- [存储设计文档](../storage.md)

### C. 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-05-19 | 初始综合安全审查报告 |

---

*报告生成工具: opencode (mi/mimo-v2.5-pro)*  
*审查人员: PM Agent + 安全审查子代理*