# ASSH v2.0.0 安全审查报告

**审查日期**: 2026-05-19  
**审查范围**: `asshv2/` 全量 Go 源码  
**审查工具**: 静态分析 (`go vet`)、数据竞争检测 (`go test -race`)、OSV 漏洞数据库、手动代码审计

---

## 目录

1. [依赖项安全审查](#1-依赖项安全审查)
2. [机密扫描审查](#2-机密扫描审查)
3. [代码扫描审查（逻辑漏洞）](#3-代码扫描审查逻辑漏洞)
4. [代码质量审查](#4-代码质量审查)
5. [存储加密质量审查](#5-存储加密质量审查)
6. [数据竞争检测](#6-数据竞争检测)
7. [风险分级汇总](#7-风险分级汇总)

---

## 1. 依赖项安全审查

### 1.1 已知漏洞

| 依赖 | 版本 | 漏洞 ID | 严重度 | 说明 |
|------|------|---------|--------|------|
| `golang.org/x/crypto` | `v0.25.0` | **CVE-2025-47914** (GHSA-f6x5-jh6r-wrfv) | **中** (MODERATE) | SSH Agent 服务在处理畸形消息时存在越界读取，可导致 panic |
| `modernc.org/sqlite` | `v1.50.0` | 无 | - | - |
| `github.com/pkg/sftp` | `v1.13.7` | 无 | - | - |
| `github.com/rs/zerolog` | `v1.32.0` | 无 | - | - |
| `github.com/urfave/cli` | `v1.22.2` | 无 | - | - |
| `github.com/qiniu/api.v7/v7` | `v7.4.0` | 无 | - | - |

> **建议**: `golang.org/x/crypto` 至少升级到 `v0.35.0` 以修复 CVE-2025-47914。

### 1.2 依赖版本状态

当前 Go 版本要求为 `go 1.25.0`（`go.mod`），属于较新版本，编译链本身安全性良好。

依赖项均为稳定版本，无 deprecated 依赖。

---

## 2. 机密扫描审查

### 2.1 硬编码凭据

- **未发现** 明显的硬编码 API 密钥、密码或令牌。

### 2.2 敏感信息管理

| 敏感数据类型 | 存储位置 | 保护措施 |
|-------------|---------|----------|
| SSH 密码 | SQLite `servers.password_encrypted` | AES-GCM 加密（12B nonce + 密文 + 16B tag） |
| 云账户 SecretKey | SQLite `cloud_accounts.secret_key` | AES-GCM 加密 |
| 跳板链路径数据 | SQLite `jump_history.path_data` | AES-GCM 加密 |
| AES 加密密钥 | SQLite `config` 表（Base64 编码） | **和加密数据存储在同一个数据库中** |
| 后端私钥 | 文件系统 `data/keys/` | 可选 account passphrase 加密 |

### 2.3 密钥管理

| 发现 | 风险等级 | 说明 |
|------|---------|------|
| 加密密钥与密文同库存储 | **⚠ 中** | AES-GCM 密钥存储在同一个 SQLite 数据库的 `config` 表中。任何能读取数据库文件的人都可以获取密钥并解密所有数据 |
| 无密钥轮换机制 | **⚠ 低** | 密钥一旦生成，除非手动删除数据库重新生成，否则不会自动轮换 |
| 备份密钥解密 | **ℹ 信息** | `Connector.accountPassphrase` 用于解密 `data/keys/` 中的备份密钥，属于合理设计 |

---

## 3. 代码扫描审查（逻辑漏洞）

### 3.1 🔴 严重：Host Key 验证完全缺失

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

### 3.2 🔴 严重：CVE-2025-47914 SSH Agent 漏洞

**直接影响包**: `golang.org/x/crypto v0.25.0`  
**影响文件**: `asshc/infra/ssh/client.go:243-264`（tryAgent 使用 SSH Agent）

在 `tryAgent()` 中，代码连接 SSH Agent (`SSH_AUTH_SOCK`) 并调用 `agentClient.Signers()`。CVE-2025-47914 表明当 SSH Agent 返回畸形消息时可能导致越界读取和 panic。

**修复建议**: 升级 `golang.org/x/crypto` 到 `v0.35.0` 或更高。

### 3.3 🟠 高：PRAGMA SQL 注入漏洞

**文件**: `asshc/infra/store/db.go:168`

```go
rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
```

**问题**: 表名通过 `fmt.Sprintf` 直接拼接到 SQL 中。尽管 `columnExists()` 当前仅由内部 `migrate()` 调用且 `table` 参数硬编码，该模式仍然是 SQL 注入风险。如果未来有代码路径将用户输入传递给 `table` 参数，后果严重。

**当前利用难度**: 低（参数不可控）  
**风险**: 未来扩展可能引入此漏洞  

**修复建议**: 使用参数化查询的 PRAGMA（SQLite 不支持 `PRAGMA table_info(?)` 参数化），或对表名做白名单校验。

### 3.4 🟠 高：七牛云同步使用明文 HTTP

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

### 3.5 🟡 中：变更日志快照包含明文密码

**文件**: `asshc/infra/store/server.go:112-116`

```go
// 序列化全量快照（密码以明文保存，用于回滚恢复）
snapshotJSON, err := json.Marshal(server)
```

**问题**: `server` 对象的 `Auth.Password` 在变更日志快照中以明文 JSON 存储。所有历史版本的密码快照均可从 `server_changelog` 表中读取。

**缓解因素**: 变更日志表与服务器表在同一数据库中，受相同访问控制保护。  
**影响**: 同一数据库泄露会导致所有历史密码暴露，而不仅是当前密码。

### 3.6 🟡 中：generateID 回退路径缺陷

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

### 3.7 🟡 中：多个 JSON Unmarshal 错误被忽略

| 文件位置 | 代码 | 风险 |
|---------|------|------|
| `server.go:465` | `json.Unmarshal([]byte(optionsJSON), &server.Options)` | 错误静默忽略，解析失败后 Options 保持为 `make(map[string]interface{})` |
| `jump.go:74` | `json.Unmarshal([]byte(decrypted), &jh.PathData)` | 解密后的 JSON 解析错误被静默忽略 |
| `jump.go:102` | `json.Unmarshal([]byte(decrypted), &jh.PathData)` | 同上 |

### 3.8 🟡 中：Daemonize 进程 fork 风险

**文件**: `asshc/infra/proxy/tunnel_manager.go:249-266`

```go
func Daemonize() error {
    exe, err := os.Executable()
    proc, err := os.StartProcess(exe, os.Args, &os.ProcAttr{...})
    os.Exit(0)
}
```

**问题**: 以当前命令行参数完整重新启动进程。如果 `os.StartProcess` 失败，`os.Exit(0)` 已执行，父进程不会收到错误。没有对调用频率做任何限制。

### 3.9 🟢 低：多处时间解析错误忽略

| 文件位置 | 代码 |
|---------|------|
| `account.go:212` | `acct.CreatedAt, _ = time.Parse(...)` |
| `account.go:213` | `acct.UpdatedAt, _ = time.Parse(...)` |
| `account.go:229` | `t, _ := time.Parse(...)` |

时间格式不匹配时将使用零值时间，下游可能产生意外行为。

---

## 4. 代码质量审查

### 4.1 🔴 构建阻断：测试接口不匹配

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

### 4.2 🟡 中：AES ECB 模式公开暴露

**文件**: `asshc/infra/crypto/aes.go:52,74,199-224`

ECB 模式通过 `AESEncrypt`/`AESDecrypt` 公共 API 暴露。虽然已有注释标明"不安全的，仅用于兼容旧系统"，但它在公开 API 中仍然可选，存在被误用的风险。

### 4.3 🟡 中：加解密代码重复

`key.go:69-140` 中的 `encryptPassword`/`decryptPassword` 与 `account.go:242-314` 中的 `encryptData`/`decryptData` 几乎完全相同的 AES-GCM 加解密逻辑。共有 **3 份** 相同的 AES-GCM 实现（key.go, account.go, aes.go 中的 GCM 模式）。

### 4.4 🟡 中：tunnel_manager.Close() 可能永久阻塞

**文件**: `asshc/infra/proxy/tunnel_manager.go:242-245`

```go
func (m *tunnelManager) Close() {
    m.cancel()
    <-m.done  // 如果 reconnectLoop 从未启动，将永久阻塞
}
```

`done` channel 仅在 `reconnectLoop()` 退出时关闭。如果从未调用过 `SetReconnect()`，则 `reconnectLoop` 未启动，`Close()` 将永远阻塞。

### 4.5 🟢 低：conn.Write 错误被忽略

多个代理协议处理函数中，`conn.Write()` 的返回值被忽略：
- `socks5.go:237,284` — 认证失败响应写入
- `http_connect.go:256,261` — HTTP 错误响应写入

在代理场景中，写入错误通常是连接已断开，影响较小但仍不符合最佳实践。

---

## 5. 存储加密质量审查

### 5.1 加密方案总评

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

### 5.2 关键发现

**Nonce 复用风险**: ✅ 正确。每次加密调用都会生成新的随机 nonce，没有 nonce 复用风险。

**IV/Nonce 长度**: ✅ GCM 使用 12 字节 standard nonce，CBC 使用 16 字节 IV，均正确。

**AEAD 使用正确性**: ✅ `gcm.Seal(nonce, nonce, plain, nil)` — 使用 nonce 作为 Seal 的前缀参数，正确。

**密钥存储安全性**: ⚠️ 加密密钥（`Store.cryptoKey`）与加密数据（`password_encrypted`, `secret_key` 列）存储在**同一个 SQLite 文件中**。数据库文件泄露 = 所有加密数据泄露。这是一个**架构层面的设计限制**，而非实现缺陷。

### 5.3 改进建议

1. **密钥与数据分离**: 将加密密钥存储在操作系统密钥链（macOS Keychain）、环境变量或独立配置文件中
2. **引入 KDF**: 若需要密码派生密钥，使用 Argon2id 替代直接随机密钥
3. **上下文绑定**: 将 `additionalData` 设置为表名+列名，绑定密文到特定上下文
4. **密钥轮换**: 实现定期密钥轮换机制和重加密功能

---

## 6. 数据竞争检测

### 6.1 测试结果

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

### 6.2 并发安全分析

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

### 6.3 潜在风险点

- `asshc/service` 包因构建失败未通过 race 检测，其中的 goroutine 使用（如 `health.go:109` 的并发健康检查）未经验证

---

## 7. 风险分级汇总

### 🔴 严重 (3)

| # | 发现 | 文件位置 |
|---|------|---------|
| 1 | Host Key 验证完全缺失（默认 MitM 无防护） | `client.go:362-369` |
| 2 | CVE-2025-47914 SSH Agent 越界读漏洞 | `go.mod` → `x/crypto v0.25.0` |
| 3 | 测试接口不匹配导致构建阻断，Race 检测无法覆盖 | `connect_test.go:63` |

### 🟠 高 (2)

| # | 发现 | 文件位置 |
|---|------|---------|
| 4 | PRAGMA SQL 注入模式（只读，当前不可利用） | `db.go:168` |
| 5 | 七牛云同步使用明文 HTTP | `qiniu.go:68,139` |

### 🟡 中 (6)

| # | 发现 | 文件位置 |
|---|------|---------|
| 6 | 加密密钥与密文同库存储 | `key.go` / `db.go` |
| 7 | 变更日志快照包含明文密码 | `server.go:112-116` |
| 8 | generateID fallback 输出内存地址 | `port.go:257` |
| 9 | 多处 JSON/时间解析错误静默忽略 | `server.go:465`, `jump.go:74,102`, `account.go:212-229` |
| 10 | AES ECB 模式公开暴露（虽已标注不安全） | `aes.go:52,199-224` |
| 11 | tunnel_manager.Close() 可能永久阻塞 | `tunnel_manager.go:242-245` |

### 🟢 低 (3)

| # | 发现 | 文件位置 |
|---|------|---------|
| 12 | conn.Write 错误被忽略（代理响应） | `socks5.go`, `http_connect.go` |
| 13 | Daemonize 进程无限制 fork | `tunnel_manager.go:249-266` |
| 14 | AES-GCM 无 additionalData 上下文绑定 | `key.go:103`, `account.go:278` |

---

## 快速修复优先级建议

### 立即修复（P0）

1. **Host Key 验证**: 修复 `getHostKeyCallback()` 逻辑倒置，实现 `known_hosts` 校验
2. **CVE-2025-47914**: 升级 `golang.org/x/crypto` 到 `v0.35.0+`
3. **测试构建**: 修复 `connect_test.go` 的 `ConnectChain` 返回类型

### 短期修复（P1）

4. **七牛云 HTTPS**: 设置 `UseHTTPS: true`，Pull 使用 HTTPS URL
5. **PRAGMA 注入**: 对 `columnExists` 的表名参数做白名单校验
6. **ECB 模式**: 从公开 API 移除或改为私有

### 中长期（P2）

7. **密钥与数据分离**: 将加密密钥移出 SQLite 数据库
8. **Known Hosts 实现**: 完整的 `known_hosts` 文件校验
9. **密钥轮换**: 实现定期自动密钥轮换
10. **代码去重**: 合并 3 份重复的 AES-GCM 实现
