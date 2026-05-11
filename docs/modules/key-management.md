# 密钥管理模块

**计划阶段**: Phase 6 (Key Management)
**依赖**: Phase 1 (crypto) ✅ + Phase 3 (ssh) ✅ + Phase 4 (CLI) ✅
**架构决策**: ADR-018 (known-servers 隐性表)
**状态**: 🔄 开发中 (2026-05-11 起)

---

## 需求概述

1. **`assh keygen`** — 生成 SSH 密钥对，自动部署到服务器，更新服务器配置
2. **Key 备份** — add/set 配置 `-k` 时自动备份密钥到 `~/.assh/v2/data/keys/`，登录优先使用备份
3. **Stdin 重定向** — `-k` 支持从管道/重定向读取密钥内容
4. **`-k` 空参自动生成** — `-k` 无参或 `-k ""` 时自动调用 keygen 生成并配置

---

## 设计分析

### 1. 密钥生成的 `-k` 空参数行为

**约束**：urfave/cli v1 的 `StringFlag` 要求必须有值参数，`assh set myserver -k` 会报错。

**方案**：采用混合策略

| 用法 | 含义 | 实现方式 |
|------|------|----------|
| `assh set myserver -k ""` | 调用内部 keygen 生成密钥并配置 | 检测 `c.IsSet("key") && c.String("key") == ""` |
| `assh set myserver -k < key.pem` | 从 stdin 读取密钥内容 | 检测 stdin 非 tty → 读取 → 保存到 `data/keys/` |
| `assh set myserver -k ~/.ssh/id_rsa` | 引用已有密钥文件 | 复制到 `data/keys/` + 更新配置（现有行为扩展） |
| `assh set myserver -k -` | 从 stdin 读取（显式） | `-` 表示 stdin |

### 2. Key 备份机制

**存储路径**: `~/.assh/v2/data/keys/{fingerprint}.pem`

- add/set 时检测 `-k` 指向的文件 → 计算 SHA256 指纹 → 复制到 `data/keys/` 
- SSH 连接时（`client.go` tryKeyFile）→ 优先查找 `data/keys/` 中匹配的备份
- 原始路径文件被篡改/删除时，仍可用备份正常连接

### 3. `assh keygen` 命令

```
assh keygen <server>                     # RSA 4096 + 部署 + 更新配置
assh keygen <server> --type ed25519       # Ed25519 密钥
assh keygen <server> --type ecdsa --bits 384  # ECDSA P-384
assh keygen <server> -k ""                # 同 keygen（set 空参触发）
assh keygen -H host -u root -P pass       # 直连模式（无服务器配置）
```

**部署流程**：
```
1. 生成密钥对（OpenSSH 格式）
2. SSH 登录服务器（使用现有配置中的密码/旧密钥）
3. 检查 ~/.ssh/authorized_keys → 追加公钥（不覆盖）
4. 保存私钥到 data/keys/{fingerprint}.pem
5. 更新服务器配置 Auth.KeyFile 指向备份路径
```

---

## 影响范围

| 范围 | 文件 | 变更类型 |
|------|------|----------|
| 新建接口 | `asshc/port/keymanager.go` | KeyManager 接口 |
| 新建 infra | `asshc/infra/keymgr/generate.go` | 密钥生成包装（调用 crypto） |
| 新建 infra | `asshc/infra/keymgr/deploy.go` | SSH 部署公钥（ssh-copy-id 逻辑） |
| 新建 infra | `asshc/infra/keymgr/backup.go` | 密钥备份与查找 |
| 新建命令 | `cmd/keygen.go` | `assh keygen` 命令 |
| 修改 | `cmd/server.go` | add/set 中 `-k` 新增 stdin/空参/备份逻辑 |
| 修改 | `asshc/infra/ssh/client.go` | tryKeyFile 优先查 data/keys/ 备份 |
| 修改 | `config/path.go` | 新增 `KeysDir` 路径 |
| 修改 | `cmd/app.go` | NewApp 签名变更（注入 KeyService） |
| 修改 | `main.go` | DI 注入 KeyService |
| 新增依赖 | go.mod | 无（复用 golang.org/x/crypto/ssh） |

---

## 子任务分解

### P6.1 接口层

| 任务 | 文件 | 说明 |
|------|------|------|
| P6.1-a 接口定义 | `asshc/port/keymanager.go` | KeyManager 接口（Generate/DeployPublicKey/Backup/Lookup） |
| **验证** | `go vet ./asshc/port/...` | |

```go
type KeyManager interface {
    // Generate 生成指定类型的密钥对，使用 account password 加密私钥。
    // 私钥保存到 data/keys/{fingerprint}.pem。
    // 返回私钥文件路径、公钥内容、指纹。
    Generate(keyType string, bits int, passphrase string) (privPath string, pubKey []byte, fingerprint string, error)
    
    // DeployPublicKey SSH 登录服务器，将公钥追加到 ~/.ssh/authorized_keys。
    DeployPublicKey(server *domain.Server, pubKey []byte) error
    
    // BackupKey 将指定路径的密钥文件复制到 data/keys/ 目录，返回备份路径。
    BackupKey(srcPath string) (backupPath string, fingerprint string, error)
    
    // LookupByPath 在 data/keys/ 中查找与原始路径匹配的备份密钥。
    LookupByPath(originalPath string) (backupPath string, found bool)
    
    // ReadKeyFromStdin 从标准输入读取密钥内容并保存到 data/keys/ 目录。
    ReadKeyFromStdin() (savedPath string, fingerprint string, error)
    
    // GetAccountPassphrase 获取 account password，用于解密私钥。
    GetAccountPassphrase() string
}
```

### P6.2 基础设施层 — `asshc/infra/keymgr/`

| 任务 | 文件 | 说明 | 状态 |
|------|------|------|:----:|
| P6.2-a 密钥生成 | `generate.go` | 包装 crypto.RSA/Ed25519/ECDSA，OpenSSH 格式输出，指纹计算 | ⏳ |
| P6.2-b 公钥部署 | `deploy.go` | SSH 登录→检测 authorized_keys→追加公钥（幂等：指纹去重） | ⏳ 暂缓 |
| P6.2-c 密钥备份 | `backup.go` | 指纹命名 + 复制 + data/keys/ 索引维护；按原始路径查找备份 | ⏳ |
| P6.2-d 单元测试 | `*_test.go` | Mock SSH + 文件系统测试 | ⏳ |
| **验证** | `go test ./asshc/infra/keymgr/... -v` | | |

**P6.2-a 密钥生成设计**：
- 包装 `crypto.GenerateRSA` / `GenerateEd25519` / `GenerateECDSA`
- 使用 `ssh.MarshalPrivateKey(key, passphrase)` 输出 OpenSSH 格式 PEM
- `passphrase` 类型为 `[]byte`（接口契约已更新），便于使用后清零
- 保存路径：`data/keys/{fingerprint}.pem`
- 返回：(privPath, pubKey[], fingerprint)
- 指纹计算：`SHA256(privKeyContent)`（非 SSH fingerprint，用于文件名）

**P6.2-b 公钥部署设计（暂缓）**：
- **待确认细节**：
  - 部署前确认：交互式提示 `Deploy public key to <server>? [Y/n]`
  - 幂等策略：指纹相同则跳过
  - 失败处理：部署失败 → 保留私钥文件，记录警告（不删除私钥）
- **幂等流程**：
  ```
  1. SSH 登录服务器
  2. 读取 ~/.ssh/authorized_keys
  3. 解析每行公钥指纹
  4. 指纹已存在 → 跳过（幂等）
  5. 指纹不存在 → 追加公钥
  6. authorized_keys 不存在 → 创建（mkdir -p ~/.ssh + touch）
  ```

**P6.2-c 密钥备份设计**：
- **源文件权限检查**：权限 > 0600 → 自动 `chmod 0600` 后复制，同时输出警告
- **幂等策略**：相同指纹文件已存在 → 跳过，返回已存在的路径
- **索引维护**：在 `data/keys/` 目录中按指纹命名文件，无需额外索引文件
- **LookupByPath**：通过原始路径 → 绝对路径 → 查找对应备份（通过已知服务器表或文件系统扫描）

**密钥存储安全**：
- 生成的私钥使用 **account password** 作为 passphrase 加密（`ssh.MarshalPrivateKey(key, accountPass)`）
- account password 存储在 SQLite `config` 表中（AES-GCM 加密），运行时解密可用
- 连接时 `tryKeyFile` 使用 `ssh.ParsePrivateKeyWithPassphrase(key, accountPass)` 解析
- 如果未设置 account password，则使用空密码（兼容模式）

### P6.3 服务层

| 任务 | 文件 | 说明 |
|------|------|------|
| P6.3-a KeyService | `asshc/service/key.go` | 密钥管理用例编排（GenerateAndDeploy/BackupAndConfigure） |
| P6.3-b 单元测试 | `asshc/service/key_test.go` | Mock KeyManager + Mock ServerRepository |
| **验证** | `go test ./asshc/service/... -v` | |

```go
type KeyService struct {
    keymgr port.KeyManager
    repo   port.ServerRepository
    ssh    port.SSHConnector  // 用于 keygen 部署时的连接
}

// GenerateAndDeploy 生成密钥 → 部署到服务器 → 更新配置
func (s *KeyService) GenerateAndDeploy(name string, keyType string, bits int) error

// HandleKeyFlag 处理 -k 参数的三种情况：
//   空字符串 → GenerateAndDeploy
//   "-" 或 stdin pipe → 读取 stdin 并保存
//   文件路径 → BackupKey 并更新配置
func (s *KeyService) HandleKeyFlag(name, keyValue string) error
```

### P6.4 CLI 命令

| 任务 | 文件 | 说明 |
|------|------|------|
| P6.4-a keygen 命令 | `cmd/keygen.go` | `assh keygen <server> [--type] [--bits]` |
| P6.4-b 命令注册 | `cmd/app.go` | registerKeygenCommands() + NewApp 签名变更 |
| P6.4-c add/set -k 增强 | `cmd/server.go` | 空参/stdin/备份逻辑 |
| **验证** | `go build ./... && assh help` | |

**`cmd/keygen.go` 命令设计**：
```go
{
    Name:      "keygen",
    Usage:     "Generate SSH key and deploy to server",
    ArgsUsage: "<server>",
    Flags: []cli.Flag{
        cli.StringFlag{Name: "type, t", Value: "rsa", Usage: "key type: rsa|ed25519|ecdsa"},
        cli.IntFlag{Name: "bits, b", Value: 4096, Usage: "key bits (for RSA/ECDSA)"},
        // 直连参数（复用已有模式）
        cli.StringFlag{Name: "H, host", Usage: "host address"},
        cli.StringFlag{Name: "u, user", Usage: "username"},
        cli.StringFlag{Name: "P, password", Usage: "password"},
        cli.IntFlag{Name: "p, port", Value: 22, Usage: "port"},
    },
}
```

### P6.5 known-servers 隐性表（ADR-018）

| 任务 | 文件 | 说明 |
|------|------|------|
| P6.5-a 接口扩展 | `asshc/port/repository.go` | 新增 `KnownServerRecorder` 接口或 `RecordDirectConnect` / `LookupKnownServer` 方法 |
| P6.5-b 数据库迁移 | `asshc/infra/store/db.go` | `migrate()` 新增 `known_servers` 建表语句 |
| P6.5-c 存储实现 | `asshc/infra/store/known.go` | 新增文件：upsert/lookup 实现 |
| P6.5-d 直连接入点 | `cmd/connect.go` + `cmd/app.go` | 直连成功后调用 `RecordDirectConnect()` |
| P6.5-e keygen 关联 | `asshc/infra/keymgr/deploy.go` | 直连 keygen 后更新 `key_backup_path` |
| **验证** | `go test ./asshc/infra/store/... -v` | |

**表结构**（详见 ADR-018）：
```sql
CREATE TABLE IF NOT EXISTS known_servers (
    id                TEXT PRIMARY KEY,     -- SHA256(user@host:port#auth_fingerprint)
    host              TEXT NOT NULL,
    port              INTEGER NOT NULL DEFAULT 22,
    user_name         TEXT NOT NULL DEFAULT 'root',
    auth_fingerprint  TEXT NOT NULL DEFAULT '',
    key_backup_path   TEXT DEFAULT '',
    last_connected_at TEXT,
    connect_count     INTEGER DEFAULT 1,
    created_at        TEXT DEFAULT (datetime('now')),
    updated_at        TEXT DEFAULT (datetime('now'))
);
```

**ID 生成**：`SHA256(fmt.Sprintf("%s@%s:%d#%s", user, host, port, authFingerprint))`

**auth_fingerprint 计算**：
| 认证方式 | 计算方式 |
|----------|----------|
| 密码 | `SHA256(password)` |
| 密钥文件 | `SHA256(绝对路径)` |
| 两者 | `SHA256(password):SHA256(keypath)` 组合 |
| 仅 agent | 空字符串 `""` |

### P6.6 SSH 连接层修改

| 任务 | 文件 | 说明 |
|------|------|------|
| P6.6-a 备份优先策略 | `asshc/infra/ssh/client.go` | tryKeyFile 优先查 data/keys/ 备份 |
| P6.6-b known-servers 密钥查找 | `asshc/infra/ssh/client.go` | 直连时查 known_servers 的 key_backup_path |
| **验证** | `go test ./asshc/infra/ssh/... -v` | |

**修改逻辑**（`tryKeyFile` 方法）：
```go
func (c *Connector) tryKeyFile(keyFile string) ssh.AuthMethod {
    // 1. 优先查找 data/keys/ 中的备份
    if backupPath := c.lookupBackup(keyFile); backupPath != "" {
        key, err := os.ReadFile(backupPath)
        if err == nil {
            if signer, err := ssh.ParsePrivateKey(key); err == nil {
                return ssh.PublicKeys(signer)
            }
        }
    }
    // 2. 回退到原始路径
    key, err := os.ReadFile(keyFile)
    if err != nil {
        return nil
    }
    signer, err := ssh.ParsePrivateKey(key)
    if err != nil {
        return nil
    }
    return ssh.PublicKeys(signer)
}
```

### P6.7 配置路径 & DI

| 任务 | 文件 | 说明 |
|------|------|------|
| P6.7-a KeysDir 路径 | `config/path.go` | 新增 `KeysDir = "~/.assh/v2/data/keys"` |
| P6.7-b DI 注入 — assh | `main.go` | 创建 KeyManager + KeyService + KnownServerRecorder，注入 cmd.NewApp() |
| P6.7-c DI 注入 — assh-fs | `cmd/fs/main.go` | 如有需要，同步注入 KeyService |
| P6.7-d App 签名变更 | `cmd/app.go` | NewApp 增加 `*service.KeyService` |
| **验证** | `go build ./... && go vet ./...` | |

---

## 验收标准

| # | 场景 | 操作 | 预期 |
|---|------|------|------|
| F1 | keygen 生成+部署 | `assh keygen myserver` | RSA 4096 密钥生成，SSH 登录，公钥追加到 authorized_keys，配置更新 |
| F2 | Ed25519 类型 | `assh keygen myserver --type ed25519` | Ed25519 密钥生成并部署 |
| F3 | 幂等部署 | 再次 `assh keygen myserver` | 相同公钥不会重复追加到 authorized_keys |
| F4 | 直连 keygen | `assh keygen -H host -u root -P pass` | 直连模式生成密钥，部署公钥，写入 known_servers |
| F5 | `-k ""` 触发 keygen | `assh set myserver -k ""` | 自动生成密钥，部署，配置更新 |
| F6 | `-k < key.pem` stdin | `assh set myserver -k < ~/.ssh/id_rsa` | 读取 key.pem 内容，备份到 data/keys/，配置更新 |
| F7 | `-k -` 显式 stdin | `assh set myserver -k -` (输入密钥内容后 Ctrl+D) | 从 stdin 读取，保存，配置 |
| F8 | `-k ~/.ssh/id_rsa` 备份 | `assh set myserver -k ~/.ssh/id_rsa` | 复制到 data/keys/{fingerprint}.pem |
| F9 | 备份优先连接 | 删除原始 key 文件后 `assh login myserver` | 自动使用 data/keys/ 备份，连接成功 |
| F10 | 密钥被篡改保护 | 修改原始 key 文件内容后连接 | 使用备份（未篡改），连接成功 |
| F11 | `-k` + 空字符串 + 已存在配置 | `assh set myserver -k ""` (已有密码认证) | 密码保留，新增 key 认证 |
| F12 | 文件路径不存在 | `assh set myserver -k /nonexistent/key` | 报错提示路径不存在 |
| F13 | keygen 到不存在的服务器 | `assh keygen nonexist` | 报错 server not found |
| F14 | 直连登录记录 known_servers | `assh root@host -P pass` | known_servers 新增记录，connect_count=1 |
| F15 | 同参数第二次直连 | 再次 `assh root@host -P pass` | connect_count=2，updated_at 更新 |
| F16 | 直连 keygen 后复用密钥 | `assh keygen -H host -u root -P pass` 后 `assh root@host` | 自动使用 known_servers 关联的密钥连接 |
| **验证** | `go build ./... && go test ./... -v` | | 编译+测试全部通过 |

---

## 依赖关系

```
Phase 3 (SSH) ──→ Phase 6 (Key Management) ──→ Phase 4 (CLI)
     ↑                       ↓
Phase 1 (crypto)        新增 keymgr/ 模块
                        修改 SSH client
                        新增 known_servers 表 (ADR-018)
```

- Phase 6 不依赖 Phase 5 (SFTP) — 可并行开发
- Phase 6 不依赖 Phase 7/8/... — 后续阶段无需等待
- ADR-018（known-servers 表）是 Phase 6 的架构决策

---

## 开放问题

1. ~~**Phase 编号**~~ ✅ 已确认：插入为 Phase 6
2. ~~**`--gen-key`**~~ ✅ 已确认：使用 `-K` / `--keygen` BoolFlag
3. ~~**keygen 直连模式**~~ ✅ 已确认：保存到 data/keys/ + known_servers 表记录
4. ~~**密钥密码保护**~~ ✅ 已确认：使用 account password 加密私钥（`ssh.MarshalPrivateKey`），非空密码。未设 account password 时使用空密码兼容。
