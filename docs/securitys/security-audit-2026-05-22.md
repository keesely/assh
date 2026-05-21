# ASSH v2.0.1 安全审查报告

**审查日期**: 2026-05-22  
**审查范围**: `asshv2/` 全量 Go 源码（~21,095 行）  
**审查版本**: v2.0.1 (Phase 10 P1 完成, X1 进行中)  
**审查目标**: `asshc/`, `cmd/`, `config/`, `log/`  
**审查工具**: `govulncheck`, `go vet`, `go test -race`, `gocyclo`, `staticcheck`, 手动代码审计  
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
8. [风险分级汇总](#8-风险分级汇总)
9. [修复建议](#9-修复建议)

---

## 1. 执行摘要

本次安全审查对 `asshv2/` 全量代码进行了六个维度的安全评估，共发现 **4 🔴 严重、12 🟡 高、19 🟠 中、24 🟢 低** 共 59 项安全问题。

### 关键发现 Top 3

| 排名 | 问题 | 风险 |
|------|------|------|
| 🥇 | **RollbackTo() 明文密码写入 Changelog** — `store/server.go:421` 在快照中保存了未加密的明文密码 | 数据库泄露导致全部密码失窃 |
| 🥈 | **加密密钥明文存储在 DB** — `store/key.go:25` AES-256-GCM 密钥以 Base64 明文与加密数据同库存储 | 一损俱损：密钥+密文同时被获取 |
| 🥉 | **SFTP Shell 本地命令注入** — `cmd/app.go:839` `!` 命令将用户输入直接传给 `exec.Command` | 任意本地命令执行 |

### 与 v2.0.0 上一轮审查对比

| 指标 | v2.0.0 (2026-05-19) | v2.0.1 (2026-05-22) | 变化 |
|------|:-------------------:|:-------------------:|:----:|
| 🔴 严重 | 6 | 4 | 减少 2（已修复 Host Key 验证、命令注入等） |
| 🟡 高 | 2+12 | 12 | 分类规范化后增加 |
| 🟠 中 | — | 19 | 细化了中危分类 |
| 🟢 低 | 11 | 24 | 更多细节发现 |
| **总计** | **29** | **59** | 审查范围更全面 |

> **注意**: 数量增加主要是因为审查更细致，并非安全性下降。上一轮报告的 6 个严重问题中，Host Key 验证缺失、CVE 漏洞、HTTP Slowloris 等已在 v2.0.1 修复。

---

## 2. 依赖项安全审查

### 2.1 已知漏洞扫描（govulncheck）

| 漏洞 ID | 严重度 | 描述 | 影响路径 | 修复版本 |
|---------|--------|------|---------|---------|
| GO-2026-4971 | HIGH | Windows `net.Dial`/`LookupPort` NUL 字节 panic | `ssh/client.go:252`、`qiniu.go:145`、`listener.go:79` | Go 1.26.3 |
| GO-2026-4918 | MEDIUM | HTTP/2 SETTINGS_MAX_FRAME_SIZE 无限循环 | `qiniu.go:145`（通过七牛 SDK） | Go 1.26.3 |
| GO-2026-4981 | LOW | `net` 包长 CNAME 响应崩溃 | 未直接调用 | Go 1.26.3 |
| GO-2026-4976 | LOW | `net/http/httputil` 查询参数超限 | 未直接调用 | Go 1.26.3 |
| GO-2026-4986/4977 | LOW | `net/mail` 二次方字符串拼接 | 未直接调用 | Go 1.26.3 |
| GO-2026-4982/4980 | LOW | `html/template` XSS 绕过 | 未直接调用 | Go 1.26.3 |

> **结论**: 8 个漏洞均位于 Go 标准库，**无第三方依赖被调用漏洞**。所有漏洞需升级 Go 1.26.2 → 1.26.3 修复。当前 Go 版本为 1.26.2。

### 2.2 第三方依赖版本滞后

| 依赖 | 当前版本 | 最新版本 | 滞后 | 建议 |
|------|---------|---------|------|------|
| `qiniu/api.v7/v7` | v7.4.0 | **v7.8.2** | 🔴 8 个小版本 | 需升级，含功能和修复 |
| `urfave/cli` | v1.22.2 | **v1.22.17** | 🔴 15 个小版本 | 需升级，含多个 bug 修复 |
| `golang.org/x/crypto` | v0.45.0 | **v0.51.0** | 🟡 6 个小版本 | 建议升级 |
| `rs/zerolog` | v1.32.0 | **v1.35.1** | 🟡 3 个小版本 | 建议升级 |
| `modernc.org/sqlite` | v1.50.0 | v1.50.1 | 🟢 1 个补丁 | 可选升级 |
| `pkg/sftp` | v1.13.7 | v1.13.10 | 🟢 3 个补丁 | 建议升级 |

### 2.3 校验和验证

```
go mod verify: ✅ 所有模块校验通过
go vet ./...:  ✅ 0 个问题
```

---

## 3. 机密扫描审查

### 🔴 高危：Git 历史中的硬编码 RSA 私钥

| 文件（历史中） | 提交 | 日期 | 密钥信息 |
|---------------|------|------|---------|
| `src/keygen/private.pem` | `17b124d` | 2019-11-12 | 1024-bit RSA，注释 `kee@VeryVN7` |
| `src/keygen/public.pem` | `17b124d` | 2019-11-12 | 对应公钥 |

该密钥对仍然存在于 git 历史中（删除提交 `59c90ef` 后仍可通过历史访问），可从所有活跃分支检出。

**建议**:
1. 立即轮换该密钥对（若仍在使用）
2. 使用 `git filter-repo` 或 BFG Repo-Cleaner 从 git 历史中彻底清除
3. 后续添加 `pre-commit` 钩子阻止私钥文件提交

### ✅ 当前代码库：无敏感信息泄露

`asshv2/` 当前工作树中**未发现**硬编码的 API 密钥、令牌、数据库凭据或连接字符串。

**测试桩数据**（非真实凭据）:
- `store/server_test.go:45` — `Password: "secret"`
- `domain/server_test.go:97,116` — `Password: "secret"`
- `ssh/client_test.go:23,41` — `Password: "testpass"` / `"secret"`

---

## 4. 代码扫描审查（逻辑漏洞）

### 4.1 🔴 CRITICAL

#### #C1 — SFTP 交互式 Shell 本地命令注入

**文件**: `cmd/app.go:839-840, 1114-1122`

```go
case "!":
    handleLocalCmd(args)  // args 来自 strings.Fields(scanner.Text())

func handleLocalCmd(args []string) {
    cmd := exec.Command(args[0], args[1:]...)  // 无验证！
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
}
```

**风险**: SFTP 交互式 Shell 的 `!` 命令将用户输入直接传给 `exec.Command`，可通过 `!rm -rf ~` 等执行任意本地命令。重复实现：`handleSftpLocalCmd`（line 1286-1294）也存在同样问题。

#### #C2 — SSH Host Key 验证可被绕过

**文件**: `asshc/infra/ssh/client.go:367-370`

```go
if v, ok := skip.(bool); ok && v {
    return ssh.InsecureIgnoreHostKey()  // 完全跳过密钥检查
}
```

**风险**: `insecure_skip_hostkey` 选项存储在 SQLite 服务器选项中，攻击者若获得数据库写入权限可对任意服务器实施 MITM 攻击。

#### #C3 — RollbackTo() 明文密码写入 Changelog

**文件**: `asshc/infra/store/server.go:377-421`

```go
decrypted, _ := s.decryptPassword([]byte(restored.Auth.Password))
restored.Auth.Password = decrypted       // 解密为明文
// ...
snapshotJSON, _ := json.Marshal(restored) // ❌ 含明文的 JSON 快照
tx.Exec(`INSERT INTO server_changelog ...`, string(snapshotJSON))
```

**风险**: 执行 `rollback` 后，新的 changelog 条目包含**未加密的明文密码**，而正确的 `Set()` 实现是先加密再快照。攻击者只需查看 `change_type='rollback'` 条目即可获取密码。

#### #C4 — 加密密钥明文存储在数据库

**文件**: `asshc/infra/store/key.go:24-25`

```go
encoded := base64.StdEncoding.EncodeToString(key)
db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", keyConfigKey, encoded)
```

**风险**: AES-256-GCM 加密密钥以 Base64 编码的明文存储在 SQLite 的 `config` 表中，与加密数据（密码、云密钥等）同库。数据库文件泄露即全面失守。

### 4.2 🟡 HIGH

#### #H1 — 服务器名引起的路径遍历（BC 日志）

**文件**: `cmd/connect.go:365, 450-458`

```go
writeLogFile(logDir+"/"+n+".log", output)  // n 为服务器名（用户输入）
```

`--servers` 或 `--group` 传入的服务器名如 `../../etc/crontab` 可导致日志写入预期目录之外。`writeLogFile` 无路径清理。

#### #H2 — SQL 注入 — 防御性不足

**文件**: `asshc/infra/store/db.go:180`

```go
rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
```

通过 `allowedTables` 白名单（`map[string]bool` 硬编码 7 个表名）缓解，白名单防御具有扩展脆弱性。`column` 参数在对比 195 行未参数化。

#### #H3 — 远程命令执行无验证（设计级）

**文件**: `cmd/connect.go:215, 277`

```go
cmd := strings.Join(c.Args()[1:], " ")     // run 命令
cmd := c.Args().Get(0)                      // bc 命令
```

用户从 CLI 传入的命令直接通过 SSH 远程执行。虽然属预期行为，但无任何白名单/黑名单/安全检查。

#### #H4 — AES-ECB 模式可用（不安全加密）

**文件**: `asshc/infra/crypto/aes.go:56-57, 252-278`

ECB 模式虽标注弃用，但通过公有 API 暴露。GCM 以外的模式不应出现在公有 API 中。

#### #H5 — 无密钥轮换机制

**文件**: 全局性问题

无密钥轮换能力。一旦当前 DEK 泄露，所有历史加密数据不可挽回。Server changelog 历史快照使用同一密钥。

#### #H6 — 敏感内存未清零

**文件**: `store/db.go:31`, `ssh/client.go:33`, `keymgr.go:43-46`

`cryptoKey`、`accountPassphrase` 等敏感字节切片在 Go 堆中长期存在，Go 字符串不可变产生多处副本，无法安全清除。

#### #H7 — PKCS1v15 加密函数已导出

**文件**: `asshc/infra/crypto/rsa.go:198-206`

`RSAEncrypt`/`RSADecrypt` 使用 PKCS1v15，易受 Bleichenbacher padding oracle 攻击。OAEP 版本存在但调用方可能误用。

#### #H8 — CBC 模式 Padding Oracle 风险

**文件**: `asshc/infra/crypto/aes.go:85-121`

CBC 解密始终执行 PKCS#7 unpadding，错误信息（`"invalid padding"`）泄露可能导致 padding oracle 攻击。

### 4.3 🟠 MEDIUM

| # | 文件:行 | 问题描述 |
|---|---------|---------|
| M1 | `cmd/connect.go:450-458` | `writeLogFile` 路径未清理，`--log` 来源的路径可能任意写 |
| M2 | `proxy/tunnel_manager.go:276-286` | PID 文件路径未验证，攻击者可控制路径写入任意位置 |
| M3 | `main.go:112-119` | `expandCombinedFlags` 组合短参数解析有限 |
| M4 | `main.go:165-189` | `resolveConfigDir` 不处理 `--config=value` 格式 |
| M5 | 多处 | TOCTOU — `os.Stat` 先于 `os.Open` 的竞态窗口 |
| M6 | `sftp/transfer.go:367-368` | `LocalExists` 先 stat 后 open 的 TOCTOU |
| M7 | `service/transfer.go:380-381` | Stat 检查在路径解析前的 TOCTOU |
| M8 | `crypto/aes.go:173,221` | AES-GCM AAD 未使用，密文无上下文绑定 |
| M9 | `crypto/aes.go:157-250` | 两套 GCM 加密/解密函数（代码重复） |
| M10 | `domain/known_server.go:36-51` | 密码认证指纹使用无盐 SHA256，易受彩虹表攻击 |
| M11 | `store/server.go:109,310,393` | JSON 编组错误静默忽略 |
| M12 | `store/account.go:236` | 时间解析错误静默忽略，产生零值 |
| M13 | `service/sync.go:167+` | 同步历史记录错误被 `_ =` 忽略 |
| M14 | `cmd/app.go:1298-1308` | `init()` 中 `os.MkdirAll` 错误被忽略 |

### 4.4 🟢 LOW

| # | 文件:行 | 问题描述 |
|---|---------|---------|
| L1 | `cmd/connect.go:312-313` | BC 日志使用 `/tmp/assh/bclogs`（世界可写/可读） |
| L2 | `cmd/app.go:958,978+` | SFTP 交互模式 `filepath.Join` 用户可控路径 |
| L3 | `crypto/rsa.go:103` | 公钥文件 0644，共享系统上可被其他用户读取 |
| L4 | `proxy/socks5.go:381` | 类型断言后字符串比较（状态码字符串比较脆弱） |
| L5 | `proxy/tunnel_manager.go:236` | `time.After` 未 `Stop()` 导致微小泄漏 |

---

## 5. 代码质量审查

### 5.1 🔴 严重代码质量问题

| # | 文件:行 | 问题 | 说明 |
|---|---------|------|------|
| Q1 | `cmd/app.go:892-1294` | **~400 行死代码** | 两套完全重复的 SFTP 交互 handler，一组未使用（14 个函数，`staticcheck` U1000） |
| Q2 | 9 处 | **所有操作使用 `context.Background()`** | SSH 传输、同步、SFTP 等全部无超时上下文 |
| Q3 | 7 处 | **`fmt.Scanln()` 在服务层** | `transfer.go:95,171,248,417` 等 — 业务逻辑直接调用用户输入，阻塞脚本模式，破坏可测试性 |
| Q4 | `cmd/app.go:688` | **`os.Exit(0)` 在 CLI `Before` 钩子** | 跳过 defer 清理，使测试集成不可用。同样在 `tunnel_manager.go:269` |

### 5.2 🟠 圈复杂度超标

| 函数 | 复杂度 | 文件:行 |
|------|--------|---------|
| `serverSetAction` | **54** 🔴 | `cmd/server.go:191` |
| `pushSingleFile` | 34 | `asshc/infra/sftp/upload.go:62` |
| `fsPullAction` | 31 | `cmd/app.go:432` |
| `syncAccountSetAction` | 30 | `cmd/sync.go:86` |
| `pullSingleFile` | 30 | `asshc/infra/sftp/download.go:47` |
| `bcAction` | 29 | `cmd/connect.go:276` |
| `serverSetAction` | 25 | `cmd/server.go:191` (不同入口) |
| `Set` | 21 | `asshc/infra/store/server.go:83` |

### 5.3 🟡 其他质量问题

| # | 严重度 | 发现 | 数量 |
|---|--------|------|------|
| Q5 | MEDIUM | 死代码（`staticcheck U1000`）— `recordDirectConnectForApp`、`expandPathSimple`、`keyTypeDefaultFilenames`、`reconnectLoop` 等 | 8 项 |
| Q6 | MEDIUM | 长参数列表（>5 参数）— `PushFile`、`PullFile`、`pushSingleFile`、`handleGet` 等 | 7 处 |
| Q7 | MEDIUM | `defer client.Close()` 静默丢弃错误（SFTP 传输时 Close 可能刷新数据的错误） | 全局性 |
| Q8 | MEDIUM | `if err == nil` 正向检查替代守卫子句 | 15+ 处 |
| Q9 | MEDIUM | `init()` 副作用 — 修改全局 CLI 模板和执行 I/O | 1 处 |
| Q10 | LOW | `interface{}` vs `any` 混用 | 15+ 处 |
| Q11 | LOW | 缺少编译期接口合规检查（仅 1 处 `JumpRecorder` 实现 `var _` 检查） | 全局性 |
| Q12 | LOW | 包注释 `sync.go` 重复（标准 Go 应为 `doc.go` 或第一个文件） | 1 处 |
| Q13 | LOW | 映射惰性初始化模式重复（`server.go:40-41,240-241`） | 2 处 |

---

## 6. 存储加密质量审查

### 6.1 加密策略覆盖

| 字段 | 表 | 加密状态 | 算法 |
|------|-----|---------|------|
| `password_encrypted` | `servers` | ✅ AES-GCM | `encryptData()`/`decryptData()` |
| `secret_key` | `cloud_accounts` | ✅ AES-GCM | `encryptData()`/`decryptData()` |
| `path_data` | `jump_history` | ✅ AES-GCM | `encryptData()`/`decryptData()` |
| `snapshot` (via Set) | `server_changelog` | ✅ AES-GCM | `encryptPassword()` 后快照 |
| `snapshot` (via RollbackTo) | `server_changelog` | 🔴 **明文** | 未加密直接快照（#C3） |
| `auth_fingerprint` | `known_servers` | 🟡 **无盐 SHA256** | `sha256.Sum256([]byte(password))` |
| `key_file` | `servers` | 🟢 明文（设计如此） | 外部文件由文件系统保护 |

### 6.2 密钥管理

| 维度 | 实现 | 评价 |
|------|------|------|
| **密钥生成** | `crypto/rand` 32 字节 | ✅ 正确 |
| **密钥存储** | Base64 编码明文在 SQLite config 表 | 🔴 **与密文同库** |
| **密钥保护 (KEK)** | 无 | 🔴 无用户口令派生 KEK |
| **密钥缓存** | 内存 `s.cryptoKey` 字段 | 🟡 从不在内存中清零 |
| **密钥轮换** | 无 | 🔴 无法轮换 |
| **密钥备份** | 无 | 🟡 无法导出/备份 |
| **AAD (关联数据)** | 未使用（`nil`） | 🟡 缺少上下文绑定 |
| **Nonce 生成** | `crypto/rand` 12 字节 | ✅ 密码学安全 |
| **Nonce 重用风险** | 低 | ✅ 每次随机生成 |
| **Tag 验证** | Go stdlib `gcm.Open()` | ✅ 自动验证 |

### 6.3 非对称加密

| 函数 | 状态 | 建议 |
|------|------|------|
| `RSAEncryptOAEP` / `RSADecryptOAEP` | ✅ 安全（SHA-256 + MGF1） | 推荐使用 |
| `RSAEncrypt` / `RSADecrypt` (PKCS1v15) | 🔴 **已知脆弱** | 应取消导出或标记弃用 |

---

## 7. 数据竞争检测

### 7.1 测试运行

```
go test -race ./... -timeout 120s
✅ 通过 — 无数据竞争被检测到
```

### 7.2 静态分析发现

| # | 严重度 | 文件:行 | 问题 |
|---|--------|---------|------|
| R1 | 🟠 MEDIUM | `store/key.go:36-45` | `getCryptoKey()` 在首次缓存前，调用者持有 **RLock** 时写入 `s.cryptoKey`。多个并发读 goroutine 竞争写入。 |
| R2 | 🟠 MEDIUM | `store/jump.go:24-136` | `JumpRecorder` 所有方法直接访问 `r.store.db` 和 `encryptData`/`decryptData`，**未持有** `s.mu` 锁。加密时会调用 `getCryptoKey()` 竞争写入 `s.cryptoKey`。 |
| R3 | 🟢 LOW | `tunnel_manager.go:236` | `time.After` 在重试循环中未调用 `timer.Stop()`，context 取消时产生微小泄漏。 |

### 7.3 并发安全性评估

| 组件 | 状态 | 分析 |
|------|------|------|
| SQLite Store | ⚠️ R1, R2 | 大部分方法锁使用正确，`JumpRecorder` 绕过锁 |
| Proxy/Tunnel | ✅ | 所有 `active` 映射由 Mutex 保护，Stop 模式正确 |
| Health Check | ✅ | Semaphore + WaitGroup + 独立连接，设计正确 |
| SSH Client | ✅ | Connector 构造后不可变，无共享状态 |
| ProxyService | ✅ | Mutex 保护可变状态，信号处理模式正确 |
| BC 命令 | ✅ | WaitGroup + Mutex + 信号量通道，设计正确 |

> **结论**: 当前单 CLI 进程使用模式下 R1-R3 不可实际利用。修复可消除理论数据竞争。

---

## 8. 风险分级汇总

### 8.1 按严重度

| 分类 | 🔴 CRITICAL | 🟡 HIGH | 🟠 MEDIUM | 🟢 LOW | 总计 |
|------|:-----------:|:--------:|:---------:|:------:|:----:|
| 依赖项安全 | 0 | 0(1)* | 1 | 7 | 8 |
| 机密扫描 | 0 | 1 | 0 | 0 | 1 |
| 逻辑漏洞扫描 | 3 | 5 | 2 | 3 | 13 |
| 代码质量 | 0 | 4 | 9 | 10 | 23 |
| 加密质量 | 1 | 2 | 5 | 3 | 11 |
| 数据竞争 | 0 | 0 | 2 | 1 | 3 |
| **总计** | **4** | **12** | **19** | **24** | **59** |

*\* 标准库 1 个 HIGH 漏洞（GO-2026-4971）通过 Go 升级修复*

### 8.2 按文件/模块

| 模块 | 严重问题 | 中等 | 低 | 合计 |
|------|---------|------|-----|------|
| `cmd/app.go` | #C1, Q1, Q3, Q4 | Q6, Q8, Q9 | L2 | **8** |
| `cmd/connect.go` | — | #H1, M1 | L1, Q7 | **4** |
| `asshc/infra/store/` | #C3, #C4, #H5 | R1, R2, M11-M14, Q8 | — | **10** |
| `asshc/infra/ssh/` | #C2 | #H6 | — | **3** |
| `asshc/infra/crypto/` | — | #H4, #H7, #H8, M8-M9 | L3 | **6** |
| `asshc/domain/` | — | M10 | — | **1** |
| `asshc/service/` | — | Q3, Q6 | M7 | **4** |
| `asshc/infra/proxy/` | — | M2 | R3, L4-L5 | **4** |
| 全局性问题 | — | #H2, M5 | Q5, Q7, Q10-Q11 | **6** |
| Git 历史 | — | 机密扫描私钥 | — | **1** |
| Go 标准库 | — | GO-2026-4971 | 其他 7 个 | **8** |

---

## 9. 修复建议

### P0 — 必须立即修复

| ID | 问题 | 修复方案 | 估算 |
|----|------|---------|------|
| #C3 | RollbackTo() 明文密码写入 Changelog | 快照前加密密码（参考 `Set()` 实现） | 0.5h |
| #C4 | 加密密钥明文存储在 DB | 引入用户口令派生 KEK（Argon2id）保护 DEK | 4h |
| #C1 | SFTP Shell 本地命令注入 | 白名单验证或路径沙箱 | 1h |
| #C2 | Host Key 验证可绕过 | 移除或加固 `insecure_skip_hostkey` 选项 | 1h |

### P1 — 高优先级

| ID | 问题 | 修复方案 | 估算 |
|----|------|---------|------|
| Q2 | 缺少 context timeout | 为 SSH/传输/同步操作添加超时上下文 | 3h |
| Q3 | 服务层 UI 交互 | 将 `fmt.Scanln` 移到 CLI 层，通过参数传递用户选择 | 4h |
| #H5 | 无密钥轮换 | 增加 key_version 和 re-encrypt 能力 | 6h |
| #H6 | 敏感内存未清零 | 对字节切片使用 `clear()` 或手动零值化 | 2h |
| #H1 | 路径遍历 | 对服务器名进行路径清理（`filepath.Clean` + 前缀检查） | 0.5h |
| #H7 | PKCS1v15 导出 | 取消导出或添加 `// Deprecated` 标记 | 0.5h |
| R1 | 读锁写入 `cryptoKey` | 使用 `atomic.Value` 或分离读写锁 | 1h |
| R2 | JumpRecorder 绕过锁 | 增加 Store 方法或获取 `s.mu` | 1h |
| 机密 | Git 历史私钥清除 | 轮换密钥 + `git filter-repo` 清除 | 2h |
| 依赖 | Go 升级 1.26.3 | 修复 8 个标准库漏洞 | 0.5h |
| 依赖 | 第三方依赖升级 | 批量更新 `qiniu`/`urfave/cli`/`x/crypto`/`zerolog` | 2h |

### P2 — 中等优先级

| ID | 问题 | 修复方案 | 估算 |
|----|------|---------|------|
| Q1 | 死代码清理 | 移除未使用的 SFTP handler 约 400 行 | 1h |
| Q4 | `os.Exit(0)` 在钩子中 | 改为返回错误或使用返回模式 | 2h |
| Q5 | 其他死代码 | 移除 `reconnectLoop`、`recordDirectConnectForApp` 等 | 1h |
| #H8 | CBC padding oracle 风险 | 添加或增强错误信息模糊化 | 1h |
| M8 | GCM AAD 未使用 | 添加 context 绑定的 AAD | 3h |
| M10 | 无盐 SHA256 哈希 | 使用 HMAC-SHA256 或加盐 | 2h |
| Q7 | defer 错误静默 | 添加 `closeAndLog` 辅助函数 | 1h |
| Q11 | 编译期接口检查 | 为所有实现添加 `var _` 检查 | 1h |
| M11-M14 | 错误静默忽略 | 关键路径增加日志记录 | 2h |

### P3 — 低优先级/长期改进

| ID | 问题 | 修复方案 | 估算 |
|----|------|---------|------|
| Q6 | 长参数列表 | 提取参数结构体或 builder 模式 | 3h |
| L1 | /tmp 日志不安全 | 移至 `~/.assh/logs/` | 0.5h |
| L4 | 字符串错误比较 | 使用 `errors.Is(err, net.ErrClosed)` | 0.5h |
| R3 | timer 泄漏 | 使用 `time.NewTimer` + `Stop()` | 0.5h |
| Q8 | 守卫子句重构 | `if err != nil { return }` 模式 | 2h |
| Q9 | init() 副作用 | 移至 `NewApp()` | 1h |

---

## 附录 A：审查命令参考

```bash
# 依赖项安全扫描
govulncheck ./...

# 竞态检测
go test -race ./... -timeout 120s

# 静态分析
go vet ./...
gocyclo -over 15 .
staticcheck ./...

# 校验和验证
go mod verify

# 可用更新检查
go list -m -u all
```

## 附录 B：相关文档

| 文档 | 说明 |
|------|------|
| `docs/SECURITYS.md` | 综合安全审查总索引（v2.0.0 合并版） |
| `docs/securitys/security-audit-2026-05-19.md` | 上一轮安全审查（v2.0.0） |
| `docs/securitys/fix-plan.md` | 上一轮修复方案 |
| `docs/bugs.md` | Bug 追踪 |
| `docs/hotfix-plan.md` | 安全修复方案 |
| `docs/TODO.md` | 开发任务清单 |

## 附录 C：版本变更

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-05-22 | 初始 v2.0.1 安全审查报告 |
