# ASSH 综合安全审查报告（索引）

**最近更新**: 2026-05-22  
**审查范围**: `asshv2/` 全量 Go 源码  
**审查工具**: `govulncheck`、`go vet`、`go test -race`、`gocyclo`、`staticcheck`、OSV 漏洞数据库、手动代码审计  
**索引版本**: 3.0  

| 版本 | 日期 | 范围 | 报告 | 发现 |
|------|------|------|------|------|
| 1.0 | 2026-05-19 | v2.0.0 | 综合审查 + 逻辑边界 | 6🔴+2🟠+12🟡+9🟢 = 29 |
| 2.0 | 2026-05-19 | v2.0.0 | 去重合并为本索引 | 同 1.0 |
| 3.0 | 2026-05-22 | **v2.0.1** | `security-audit-2026-05-22.md` | **4🔴+12🟡+19🟠+24🟢 = 59** |

---

## 目录

1. [执行摘要](#1-执行摘要)
2. [依赖项安全审查](#2-依赖项安全审查)
3. [机密扫描审查](#3-机密扫描审查)
4. [代码扫描审查（逻辑漏洞）](#4-代码扫描审查逻辑漏洞)
5. [代码质量审查](#5-代码质量审查)
6. [存储加密质量审查](#6-存储加密质量审查)
7. [数据竞争检测](#7-数据竞争检测)
8. [架构级安全建议](#8-架构级安全建议)
9. [风险分级汇总（去重）](#9-风险分级汇总去重)

---

## 1. 执行摘要

本报告合并了安全审查和逻辑边界审查两份独立报告，经去重后共发现 **6 🔴 严重、2 🟠 高、12 🟡 中、9 🟢 低** 共 29 项安全问题。

### 关键发现 Top 3

| 排名 | 问题 | 风险 |
|------|------|------|
| 🥇 | **Host Key 验证完全缺失** — `getHostKeyCallback()` 始终返回 `InsecureIgnoreHostKey()` | MitM 攻击，无服务器身份校验 |
| 🥈 | **远程命令注入** — `sha256sum` 路径拼接 + 公钥部署 shell 拼接 | 远程代码执行 |
| 🥉 | **CVE-2025-47914** — `golang.org/x/crypto` SSH Agent 越界读 + **CVE-2025-58181** GSSAPI 内存消耗 | 畸形消息可致 panic / 无限制内存消耗 |

### 立即行动项（P0）

1. 升级 `golang.org/x/crypto` 至 v0.45.0+ 修复 CVE-2025-47914 + CVE-2025-58181
2. 修复 `getHostKeyCallback()` 逻辑倒置 + 实现 `known_hosts` 校验
3. 修复 `sha256sum` 和公钥部署中的命令注入
4. 修复 HTTP CONNECT 握手无读取限制（Slowloris 攻击面）
5. 修复测试 mock 接口不匹配（构建阻断 / race 检测阻塞）

---

## 2. 依赖项安全审查

### 2.1 已知漏洞

| 依赖 | 版本 | 漏洞 ID | 严重度 | 说明 |
|------|------|---------|--------|------|
| `golang.org/x/crypto` | `v0.25.0` → `v0.45.0` | **CVE-2025-47914** (GHSA-f6x5-jh6r-wrfv) + **CVE-2025-58181** | **中** | SSH Agent 畸形消息越界读取 → panic；GSSAPI 无限制内存消耗 |
| `modernc.org/sqlite` | `v1.50.0` | 无 | - | - |
| `github.com/pkg/sftp` | `v1.13.7` | 无 | - | - |
| `github.com/rs/zerolog` | `v1.32.0` | 无 | - | - |
| `github.com/urfave/cli` | `v1.22.2` | 无 | - | - |
| `github.com/qiniu/api.v7/v7` | `v7.4.0` | 无 | - | - |

> **建议**: `golang.org/x/crypto` 至少升级到 `v0.45.0`。

### 2.2 依赖版本状态

当前 Go 版本 `go 1.25.0`（`go.mod`）。依赖项均为稳定版本，无 deprecated 依赖。

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
| AES 加密密钥 | SQLite `config` 表（Base64 编码） | **⚠ 和加密数据存储在同一个数据库中** |
| 后端私钥 | 文件系统 `data/keys/` | 可选 account passphrase 加密 |

### 3.3 密钥管理

| 发现 | 风险等级 | 说明 |
|------|---------|------|
| 加密密钥与密文同库存储 | **🟡 中** | AES-GCM 密钥存储在同一个 SQLite `config` 表。数据库文件泄露 = 所有数据解密 |
| 无密钥轮换机制 | **🟢 低** | 密钥一次性生成，不会自动轮换 |
| 备份密钥解密 | ℹ 信息 | `Connector.accountPassphrase` 用于解密备份密钥，合理设计 |

---

## 4. 代码扫描审查（逻辑漏洞）

> 注：本部分合并了安全审查和逻辑边界审查中的代码漏洞发现，已做去重处理。

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

**问题**: 逻辑倒置——默认始终跳过 host key 校验。`KnownHostsChecker.Check()` (`hostkey.go:21`) 也是 no-op 实现。

**影响**: 无法防范 MitM 攻击。任何能拦截 SSH 连接的人可伪造服务器身份。

**修复建议**: 
1. 实现 `known_hosts` 文件校验
2. 逻辑修正为：默认校验 → 选项 `insecure_skip_hostkey: true` 时跳过

---

### 4.2 🔴 严重：CVE-2025-47914 SSH Agent 漏洞

**包**: `golang.org/x/crypto v0.25.0`  
**影响文件**: `asshc/infra/ssh/client.go:243-264`（tryAgent）

`tryAgent()` 连接 SSH Agent 并调用 `agentClient.Signers()`。CVE-2025-47914 表明畸形消息可致越界读取和 panic。

**修复建议**: 升级 `golang.org/x/crypto` 到 `v0.45.0+`。

---

### 4.3 🔴 严重：远程命令注入 — sha256sum 路径拼接

**文件**: `asshc/infra/sftp/upload.go:305`、`asshc/infra/sftp/verify.go:140`

```go
out, err := session.Output("sha256sum " + remotePath)
```

**问题**: `remotePath` 直接拼接到 shell 命令中。路径含 `; rm -rf /`、`$(malicious)`、反引号时可在远程服务器执行任意命令。

**风险等级**: 🔴 HIGH — 远程代码执行

**修复建议**:
```go
// 方案一：shellquote 转义
import "github.com/kballard/go-shellquote"
safePath := shellquote.Join(remotePath)
out, err := session.Output("sha256sum " + safePath)

// 方案二（推荐）：SFTP 读取 + 本地计算 hash，完全避免远程命令
```

---

### 4.4 🔴 严重：公钥部署命令注入

**文件**: `asshc/service/deploy.go:121-132`

```go
pubKeyStr := strings.ReplaceAll(string(pubKey), "'", "'\"'\"'")
cmd := fmt.Sprintf(
    `mkdir -p ~/.ssh && chmod 700 ~/.ssh && printf '%%s\n' '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`,
    pubKeyStr,
)
```

**问题**: 仅转义单引号，`$(...)` 或反引号仍会被 shell 执行。

**修复建议**: 通过 stdin 传入公钥，避免 shell 转义：
```go
session.Stdin = bytes.NewReader(append(pubKey, '\n'))
cmd := `mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`
return session.Run(cmd)
```

---

### 4.5 🔴 严重：HTTP CONNECT 握手无读取限制（Slowloris 攻击面）

**文件**: `asshc/infra/proxy/http_connect.go:151-205`

```go
for {
    line, err := br.ReadString('\n')
    // ... 无限制读取头部
}
```

**问题**:
1. 无限发送头部行 → 耗尽服务端内存
2. 无读取超时 → 慢速 DoS
3. 无缓冲区大小限制

**修复建议**: 添加超时 + 头部数量/大小限制：
```go
conn.SetReadDeadline(time.Now().Add(30 * time.Second))
maxHeaders := 100
maxHeaderSize := 8192
```

---

### 4.6 🟠 高：PRAGMA SQL 注入模式

**文件**: `asshc/infra/store/db.go:168`

```go
rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
```

**问题**: 表名通过 `fmt.Sprintf` 拼接。SQLite 不支持 PRAGMA 参数化，应做表名白名单校验。

**当前利用难度**: 低（参数来自内部 `migrate()` 硬编码调用）  
**风险**: 未来扩展可能引入此漏洞

**修复建议**: 添加白名单验证：
```go
var allowedTables = map[string]bool{"servers": true, "cloud_accounts": true, "jump_history": true, "config": true, "server_changelog": true}
if !allowedTables[table] { return false, fmt.Errorf("table %q not allowed", table) }
```

---

### 4.7 🟠 高：七牛云同步使用明文 HTTP

**文件**: `asshc/infra/sync/qiniu/qiniu.go:68,139`

```go
UseHTTPS: false     // ← 明文
downloadURL := fmt.Sprintf("http://%s/%s", domain, syncFileName)  // ← HTTP
```

**问题**: Pull 通过 HTTP 下载同步数据（含完整服务器配置），可被中间人拦截/篡改。

**修复建议**: `UseHTTPS: true` + 下载 URL 使用 `https://`。

---

### 4.8 🟡 中：变更日志快照包含明文密码

**文件**: `asshc/infra/store/server.go:112-116`

```go
snapshotJSON, err := json.Marshal(server)  // server.Auth.Password 以明文序列化
```

**问题**: 变更日志快照中密码为明文 JSON。历史所有密码版本均可从 `server_changelog` 表读取。

**修复建议**: 创建快照时对密码字段单独加密，恢复时解密。

---

### 4.9 🟡 中：SmartProxy.Stop() 不等待活跃连接完成

**文件**: `asshc/infra/proxy/listener.go:110-131`

```go
sp.active = make(map[net.Conn]struct{})
sp.client = nil   // ← 直接置 nil
sp.cancel()
return nil        // 没有 sp.wg.Wait()
```

**问题**: `Stop()` 没有 `wg.Wait()`，goroutine 可能在 `Stop()` 返回后访问已置 nil 的 `sp.client`。

**风险**: 竞态条件 / panic

**修复建议**: 添加 `sp.wg.Wait()`，在取消 context 后等待所有 goroutine 退出。

---

### 4.10 🟡 中：tunnelManager.Close() 可能永久阻塞

**文件**: `asshc/infra/proxy/tunnel_manager.go:241-245`

```go
func (m *tunnelManager) Close() {
    m.cancel()
    <-m.done   // 如果 reconnectLoop 从未启动，永久阻塞
}
```

**问题**: `m.done` 只在 `reconnectLoop` 的 defer 中关闭。直连模式下 `reconnectLoop` 未启动。

**修复建议**: 添加超时 `select` 或在新 `NewTunnelManager()` 时预关闭 channel。

---

### 4.11 🟡 中：批处理命令无输出大小限制

**文件**: `cmd/connect.go:298-360`

大输出（如 `cat /dev/urandom`）或多个服务器并发时可能导致 OOM。

**修复建议**: 限制输出大小（如 10MB）+ 限制并发数（如信号量 chan struct 限制 10 个并发）。

---

### 4.12 🟡 中：SFTP 递归遍历无深度/数量限制

**文件**: `asshc/infra/sftp/upload.go:232-254`、`asshc/infra/sftp/download.go:205-227`

深度递归无限制，极深目录结构可能导致栈溢出。

**修复建议**: 添加 `maxDepth = 100`、`maxFiles = 100000`。

---

### 4.13 🟡 中：qiniu Pull 响应体无大小限制

**文件**: `asshc/infra/sync/qiniu/qiniu.go:150-155`

```go
buf := new(bytes.Buffer)
_, err = buf.ReadFrom(resp.Body)  // 无大小限制
```

**修复建议**: 使用 `io.LimitReader(resp.Body, maxSyncDataSize)`。

---

### 4.14 🟡 中：server.Options JSON 反序列化错误被静默忽略

**文件**: `asshc/infra/store/server.go:464-466`

```go
json.Unmarshal([]byte(optionsJSON), &server.Options)  // 错误被忽略
```

**修复建议**: 至少记录日志 `log.Warnf("failed to parse options: %v", err)`。

---

### 4.15 🟡 中：AES ECB 模式公开暴露

**文件**: `asshc/infra/crypto/aes.go:52,74,199-224`

ECB 模式通过 `AESEncryptECB`/`AESDecryptECB` 公开。虽已标注"不安全，仅用于兼容旧系统"，但仍可被误用。

**修复建议**: 标记为 `Deprecated` 或改为私有。

---

### 4.16 🟡 中：加解密代码重复（3 份 AES-GCM 实现）

**文件**: `asshc/infra/store/key.go:69-140`、`asshc/infra/store/account.go:242-314`、`asshc/infra/crypto/aes.go`（GCM 模式）

几乎相同的 AES-GCM 加解密逻辑在 3 个文件中重复。维护风险，一处出错影响所有。

**修复建议**: 统一到 `asshc/infra/crypto/aes.go`，`key.go` 和 `account.go` 调用统一实现。

---

### 4.17 🟢 低：strconv.Atoi 错误被忽略

**文件**: `asshc/service/connect.go:166,209`

```go
port, _ = strconv.Atoi(portStr)  // 失败时 port=0，后续尝试连接端口 0
```

---

### 4.18 🟢 低：SyncHistory 记录失败被静默忽略

**文件**: `asshc/service/sync.go:167,177,209,229,352`

```go
_ = s.store.RecordSyncHistory(…)  // 审计日志缺失
```

---

### 4.19 🟢 低：generateID fallback 输出指针地址

**文件**: `asshc/infra/proxy/port.go:256-257`

```go
return fmt.Sprintf("tunnel-%d", &b)  // 输出 slice 内存地址，非唯一 ID
```

**修复建议**: 使用 `time.Now().UnixNano()` 或 PID 作为 fallback。

---

### 4.20 🟢 低：qiniu SrcUpHosts 可能为空切片

**文件**: `asshc/infra/sync/qiniu/qiniu.go:136`

```go
domain := s.cfg.Zone.SrcUpHosts[0]  // 空切片时 panic
```

**修复建议**: 先检查 `len(s.cfg.Zone.SrcUpHosts) == 0`。

---

### 4.21 🟢 低：keepAlive goroutine 退出机制脆弱

**文件**: `asshc/infra/ssh/client.go:103-113`

goroutine 仅靠 `SendRequest` 失败退出。如果 `client.Close()` 不触发 `SendRequest` 失败（极端情况），ticker 持续运行。

**修复建议**: 增加 context/done channel 支持主动退出。

---

### 4.22 🟢 低：conn.Write / writeError 错误被忽略

**文件**: `asshc/infra/proxy/socks5.go:237,284`、`http_connect.go:255-257`

代理协议中错误响应写入失败被忽略（best-effort，可接受但不符合最佳实践）。

---

### 4.23 🟢 低：DeployService session 未 defer Close

**文件**: `asshc/service/deploy.go:67-74`

```go
session, err := client.NewSession()
// ...
session.Close()  // 非 defer，panic 时泄露
```

**修复建议**: 改为 `defer session.Close()`。

---

### 4.24 🟢 低：时间解析错误被忽略

**文件**: `asshc/infra/store/account.go:212-229`

多处 `time.Parse` 错误被静默忽略，使用零值时间。

---

### 4.25 🟢 低：Daemonize 无限制进程 fork

**文件**: `asshc/infra/proxy/tunnel_manager.go:249-266`

```go
os.StartProcess(exe, os.Args, …)
os.Exit(0)  // 即使 StartProcess 失败也退出
```

无调用频率限制，无错误传递。

---

## 5. 代码质量审查

### 5.1 🔴 严重：测试构建阻断

**文件**: `asshc/service/connect_test.go:63`

**问题**: mock 的 `ConnectChain` 返回 `(interface{}, error)`，但接口要求 `(*ssh.Client, error)`。

**影响**:
- `go vet ./...` 报错
- `asshc/service` 包所有测试和 race 检测被阻塞
- 16 个测试用例 + 并发健康检查等 goroutine 未经验证

**修复**: 将 `ConnectChain` 返回类型改为 `(*ssh.Client, error)`。

### 5.2 🟢 低：AES-GCM 无 additionalData 上下文绑定

**文件**: `asshc/infra/store/key.go:103`、`account.go:278`

所有 `gcm.Seal`/`gcm.Open` 调用使用 `nil` 作为 `additionalData`。建议绑定到表名+列名，增强上下文完整性。

---

## 6. 存储加密质量审查

### 6.1 加密方案总评

| 方面 | 评价 |
|------|------|
| 加密算法 | ✅ AES-256-GCM（行业标准认证加密） |
| 随机数生成 | ✅ `crypto/rand.Reader`（密码学安全） |
| Nonce 使用 | ✅ 每次加密生成独立随机 nonce（12 字节） |
| 认证标签 | ✅ GCM 提供 16 字节认证标签 |
| ECDH / RSA | ✅ 正确使用 `crypto/rand.Reader` 生成 IV |
| 附加数据 (AD) | ⚠️ 全部使用 `nil` AD，无上下文绑定 |
| 密钥生成 | ⚠️ 32 字节随机，无 KDF（Argon2/PBKDF2） |
| 密钥存储 | ⚠️ **架构性弱点**：与密文同库（SQLite `config` 表） |
| 提权路径 | ⚠️ 解密函数在多处可用，无独立鉴权 |

### 6.2 关键确认

- **Nonce 复用风险**: ✅ 正确。每次加密新 nonce
- **IV/Nonce 长度**: ✅ GCM 12B、CBC 16B，均正确
- **AEAD 使用**: ✅ `gcm.Seal(nonce, nonce, plain, nil)` 规范
- **密钥存储**: ⚠️ `Store.cryptoKey` + 密文在同一 SQLite 文件。文件泄露 = 全量解密

### 6.3 改进建议

1. **密钥与数据分离**: 迁移至 OS 密钥链（macOS Keychain）或环境变量
2. **引入 KDF**: Argon2id 替代直接随机密钥
3. **上下文绑定**: `additionalData` 设为表名+列名
4. **密钥轮换**: 实现定期自动轮换和重加密

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
assh/asshc/service              ❌ 构建失败（接口不匹配 → race 检测阻塞）
```

### 7.2 并发安全组件分析

| 组件 | 保护机制 | 评估 |
|------|---------|------|
| `Store` (SQLite) | `sync.RWMutex` | ✅ 读写分离，正确加锁 |
| `SOCKS5Proxy` | `sync.Mutex` | ✅ |
| `HTTPConnectProxy` | `sync.Mutex` | ✅ |
| `LocalForward`/`RemoteForward` | `sync.Mutex` | ✅ |
| `tunnelManager` | `sync.Mutex` | ✅ |
| `SmartProxy` | `sync.Mutex` + `sync.WaitGroup` + `atomic` | ✅ sessionCounter 使用 atomic |
| `bidirectionalCopy` | `sync.WaitGroup` | ✅ |
| `RuleEngine` | `sync.RWMutex` | ✅ 读操作 RLock |

**结论**: 已测试 5 个包全部通过。`asshc/service` 包因构建阻断未经验证。

---

## 8. 架构级安全建议

### 8.1 远程命令执行模式需统一安全处理

当前 4 处通过 SSH 执行 shell 命令且拼接外部输入：
- `upload.go:305` — `sha256sum` + path
- `verify.go:140` — `sha256sum` + path
- `deploy.go:130` — `printf` + pubKey
- `session.go:89` — 通用 `Run(cmd)`（由调用方负责安全）

**建议**: 创建统一 shell escape 工具函数，或改用 SFTP 文件操作 + 本地计算。

### 8.2 代理组件需统一资源生命周期管理

SOCKS5Proxy、HTTPConnectProxy、SmartProxy、LocalForward、RemoteForward 各有独立 `active map` 和锁机制。

**建议**: 抽象为统一 `ConnectionPool` 接口。

### 8.3 HTTP 代理需安全加固

- 添加 TLS 支持（当前明文传输代理认证凭据）
- 添加连接速率限制
- 添加每连接读写超时

---

## 9. 风险分级汇总（去重）

> 以下列表已合并安全审查和逻辑边界审查的发现，每个问题只出现一次。

### 🔴 严重（6 项）

| # | 发现 | 文件位置 | 来源 |
|---|------|---------|------|
| 1 | **Host Key 验证完全缺失**（默认 MitM 无防护） | `client.go:362-369` | 代码扫描 |
| 2 | **CVE-2025-47914** SSH Agent 越界读 + **CVE-2025-58181** GSSAPI 内存消耗 | `go.mod` → `x/crypto v0.25.0` → `v0.45.0` | 依赖项 |
| 3 | **远程命令注入** — sha256sum 路径拼接 | `upload.go:305`, `verify.go:140` | 逻辑边界 |
| 4 | **远程命令注入** — 公钥部署 shell 拼接 | `deploy.go:121-132` | 逻辑边界 |
| 5 | **HTTP CONNECT 握手无读取限制**（Slowloris） | `http_connect.go:151-205` | 逻辑边界 |
| 6 | **测试接口不匹配** → 构建阻断 / race 阻塞 | `connect_test.go:63` | 代码质量 |

### 🟠 高（2 项）

| # | 发现 | 文件位置 | 来源 |
|---|------|---------|------|
| 7 | **PRAGMA SQL 注入模式**（只读，当前不可利用） | `db.go:168` | 代码扫描 |
| 8 | **七牛云同步使用明文 HTTP** | `qiniu.go:68,139` | 代码扫描 |

### 🟡 中（12 项）

| # | 发现 | 文件位置 | 来源 |
|---|------|---------|------|
| 9 | **加密密钥与密文同库存储**（架构性弱点） | `key.go` / `db.go` | 存储加密 |
| 10 | **变更日志快照包含明文密码** | `server.go:112-116` | 代码扫描 |
| 11 | **SmartProxy.Stop() 不等待 goroutine**（竞态） | `listener.go:110-131` | 逻辑边界 |
| 12 | **tunnelManager.Close() 可能永久阻塞** | `tunnel_manager.go:241-245` | 代码质量 |
| 13 | **批处理命令无输出大小限制**（OOM） | `connect.go:298-360` | 逻辑边界 |
| 14 | **SFTP 递归遍历无深度/数量限制** | `upload.go:232-254`, `download.go:205-227` | 逻辑边界 |
| 15 | **qiniu Pull 响应体无大小限制**（OOM） | `qiniu.go:150-155` | 逻辑边界 |
| 16 | **server.Options JSON 反序列化错误静默忽略** | `server.go:464-466` | 逻辑边界 |
| 17 | **AES ECB 模式公开暴露** | `aes.go:52,199-224` | 代码质量 |
| 18 | **加解密代码重复**（3 份 AES-GCM） | `key.go`, `account.go`, `aes.go` | 代码质量 |
| 19 | **多处 JSON/时间解析错误静默忽略** | `account.go:212-229`, `jump.go:74,102`, `server.go:465` | 代码扫描 |
| 20 | **generateID fallback 输出内存地址** | `port.go:257` | 代码扫描 |

### 🟢 低（9 项）

| # | 发现 | 文件位置 | 来源 |
|---|------|---------|------|
| 21 | **strconv.Atoi 错误被忽略** | `connect.go:166,209` | 逻辑边界 |
| 22 | **SyncHistory 记录失败静默忽略**（审计缺失） | `sync.go:167,177,209,229,352` | 逻辑边界 |
| 23 | **qiniu SrcUpHosts 可能为空切片→panic** | `qiniu.go:136` | 逻辑边界 |
| 24 | **keepAlive goroutine 退出机制脆弱** | `client.go:103-113` | 逻辑边界 |
| 25 | **conn.Write / writeError 错误被忽略** | `socks5.go:237,284`, `http_connect.go:256` | 代码质量 |
| 26 | **DeployService session 未 defer Close** | `deploy.go:67-74` | 逻辑边界 |
| 27 | **时间解析错误被忽略** | `account.go:212-229` | 代码扫描 |
| 28 | **Daemonize 无限制进程 fork** | `tunnel_manager.go:249-266` | 代码扫描 |
| 29 | **AES-GCM 无 additionalData 上下文绑定** | `key.go:103`, `account.go:278` | 存储加密 |

---

## 附录

### A. 审查工具

- `go vet ./...` — 静态分析
- `go test -race ./...` — 数据竞争检测
- OSV 漏洞数据库 — 依赖项安全检查
- 手动代码审计 — 逻辑漏洞、加密质量、边界检查

### B. 相关文档

| 文档 | 说明 |
|------|------|
| `docs/securitys/security-review-2026-05-19.md` | v2.0.0 原始安全审查报告（源文档） |
| `docs/securitys/LOGIC_BOUNDARY_AUDIT.md` | v2.0.0 原始逻辑边界审查报告（源文档） |
| `docs/securitys/security-audit-2026-05-19.md` | v2.0.0 综合审查报告（源文档，含修复方案摘要） |
| `docs/securitys/fix-plan.md` | v2.0.0 详细修复方案（P0-P3 代码级步骤） |
| **`docs/hotfix-plan.md`** | **v2.0.0 配套修复计划（合并去重版）** |
| **`docs/securitys/security-audit-2026-05-22.md`** | **v2.0.1 全量安全审查（本报告）** |

### C. 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-05-19 | 初始安全审查报告 (v2.0.0) |
| 2.0 | 2026-05-19 | 合并安全审查 + 逻辑边界审查，去重后归档为 `SECURITYS.md` |
| 3.0 | 2026-05-22 | v2.0.1 全量安全审查 — `security-audit-2026-05-22.md`（59 项发现：4 CRITICAL, 12 HIGH, 19 MEDIUM, 24 LOW） |

---

*本报告由 `docs/securitys/` 目录下 5 份源文档合并生成，原始文件未修改。*
