# Phase 6 P6.2 密钥管理基础设施层实现

**日期**: 2026-05-12
**阶段**: Phase 6 (Key Management) — P6.2 infra 层
**版本**: v2.0.0-phase-6 (开发中)

---

## 1. 完成内容

### P6.2-a 密钥生成 (`asshc/infra/keymgr/generate.go`)
- 实现 `KeyManager.Generate()` — RSA/Ed25519/ECDSA 密钥生成
- 参数验证：RSA bits ≥ 2048（默认 4096）、ECDSA bits ∈ {256,384,521}（默认 384）、Ed25519 忽略 bits
- 私钥使用 `ssh.MarshalPrivateKey`/`ssh.MarshalPrivateKeyWithPassphrase` 生成 OpenSSH 格式 PEM
- 公钥使用 `ssh.NewPublicKey` + `ssh.MarshalAuthorizedKey`（authorized_keys 格式）
- 指纹计算：SHA256(PEM 内容)，作为文件名 `{fingerprint}.pem`
- 文件权限 0600，目录自动创建

### P6.2-c 密钥备份 (`asshc/infra/keymgr/backup.go`)
- 实现 `KeyManager.BackupKey()` — 复制密钥到 data/keys/
- 权限检查：group/other 访问位非零 → 返回错误
- 幂等：相同指纹文件已存在 → 返回已有路径
- 索引管理：`data/keys/.index.json` 维护 `absPath → fingerprint` 映射
- 实现 `KeyManager.LookupByPath()` — 通过索引快速查找备份

### KeyManager 结构 (`asshc/infra/keymgr/keymgr.go`)
- `KeyManager` 结构体：keysDir, passphrase, stdin 字段（无互斥锁，状态由调用方管理）
- `New(keysDir, passphrase)` 构造函数（路径展开 + passphrase 复制）
- `GetAccountPassphrase()` — 返回 passphrase 副本
- `ReadKeyFromStdin()` — 从 stdin 读取并保存密钥
- `DeployPublicKey()` 桩 — 返回 "not implemented"（P6.2-b 暂缓）

### P6.2-d 单元测试 (`asshc/infra/keymgr/keymgr_test.go`)
- 35 个测试用例，全部通过

**测试覆盖：**
- 构造/GetAccountPassphrase: 3 个
- Generate RSA: 5 个（4096、默认、2048、太小、无效 bits）
- Generate Ed25519: 2 个（标准、忽略 bits）
- Generate ECDSA: 4 个（默认、P256、P521、无效 bits）
- 边界: 3 个（无效类型、passphrase 加密、文件定位）
- BackupKey: 7 个（标准、同源同目标、不同文件覆盖、权限错误、不存在、目录、只读源）
- BackupPath: 4 个（查找、未找到、不同服务器、覆盖后查找）
- ReadKeyFromStdin: 2 个（标准、空输入）
- 其他: 3 个（DeployPublicKey 桩、不存在的目录、自动创建目录）

## 2. 设计决策

### 密钥指纹计算
- 指纹 = SHA256(文件内容)，用于文件名 `{fingerprint}.pem`
- Generate 和 Backup 使用相同算法，确保一致性

### 公钥格式
- 使用 `ssh.MarshalAuthorizedKey` 而非 `key.ToOpenSSHPublicKey()`
- 原因 1：`ssh.Marshal()` 返回 wire format，非 authorized_keys 格式
- 原因 2：`ssh.Marshal()` 在 Ed25519 上有 reflect panic bug（v0.25.0）

### 索引文件
- `data/keys/.index.json` 存储 `{absPath: fingerprint}` 映射
- 原因：避免每次 LookupByPath 都扫描所有密钥文件
- 更新时机：BackupKey 成功时自动更新

### Passphrase 处理
- 使用 `ssh.MarshalPrivateKeyWithPassphrase`（非 `MarshalPrivateKey`）
- `ssh.MarshalPrivateKey` 第二个参数是 comment，非 passphrase
- Passphrase 类型为 `[]byte`，New 和 GetAccountPassphrase 均返回副本

### 权限策略
- 遵循接口定义：group/other 访问位非零 → 返回错误（比设计文档的 auto-chmod 更保守）
- 生成的密钥默认 0600，备份文件保持 0600

## 3. 文档对齐

- [x] `docs/TODO.md` — P6.2-a/c 标记 ✅，新增 P6.2-d ✅
- [x] `docs/modules/key-management.md` — 状态更新
- [ ] `docs/architecture.md` — 未变更（keymgr 是新模块，不改变架构）

## 4. 下一步待办

1. [P6.3] KeyService 服务层（`asshc/service/key.go`）— 用例编排
2. [P6.4] keygen 命令（`cmd/keygen.go`）
3. [~~P6.2-b~~ 暂缓] 公钥部署
4. [P6.5-e] keygen 关联 known-servers
5. [P6.6] SSH 客户端备份优先策略
6. [P6.7] DI 注入

## 5. git 日志

```
asshv2/ (assh 仓库): 新增 asshc/infra/keymgr/ (4 files, 35 tests)
assh-builder/ (文档仓库): TODO.md + key-management.md 状态更新
```

---

## 6. 设计修订（2026-05-12 第二轮）

应产品要求，对备份快照设计做出以下调整：

### 修订内容

| 项目 | 初始设计 | 修订后 |
|------|----------|--------|
| 备份命名 | `{fingerprint}.pem`（内容指纹） | `{sha256(serverName)}.pem`（服务器名哈希） |
| BackupKey 签名 | `BackupKey(srcPath)` → 自动决定路径 | `BackupKey(srcPath, destPath)` → 调用者指定 |
| 索引文件 | 需要 `.index.json` | 不需要，服务器名哈希即为索引 |
| LookupByPath | 通过索引查找 | 替换为 `BackupPath(name)` 直接计算+检查 |
| Auth.KeyFile 变更 | 备份时也更新配置 | 备份时不修改（快照模式），Generate 时更新 |

### 影响

- 接口：`port/keymanager.go` — BackupKey 签名变更，LookupByPath → BackupPath
- 实现：`infra/keymgr/backup.go` — 移除索引，使用 `destPath` 参数
- 测试：测试数量从 33 → 35（新增 BackupPath 测试，移除索引测试）

### 文件状态

- `asshc/port/keymanager.go` — 接口已更新
- `asshc/infra/keymgr/keymgr.go` — 移除索引相关方法，新增 BackupPath
- `asshc/infra/keymgr/backup.go` — 重写为 destPath 模式
- `asshc/infra/keymgr/keymgr_test.go` — 36 个测试
- `docs/modules/key-management.md` — 设计同步更新
- `docs/TODO.md` — 状态同步
