# Phase 6 — ssh-keygen 参数对齐计划 + 执行记录

**计划日期**: 2026-05-12
**执行日期**: 2026-05-12
**阶段**: Phase 6 (Key Management) — 子任务：参数对齐
**版本**: v2.0.0-phase-6 (开发中)
**状态**: ✅ 完成

---

## 1. 变更概览

对 `assh keygen` 命令进行参数对齐，参考 `ssh-keygen` 标准，新增/调整以下参数：

| 变更 | 参数 | 说明 | 影响层级 |
|------|------|------|----------|
| **新增别名** | `-f` | `--output` 的别名，对齐 ssh-keygen | CLI 层 |
| **新增参数** | `-C`/`--comment` | 密钥注释 | CLI + Service + Port + Infra |
| **新增参数** | `-N`/`--new-passphrase` | 私钥密码（覆盖 account password） | CLI 层 |
| **行为变更** | 无参运行 | 触发交互式向导（替代当前显示 help） | CLI 层 |
| **不变** | `-P` | 保持为直连登录密码 | — |
| **不变** | RSA 默认 4096/最小 2048 | 不按 ssh-keygen 的 3072/1024 调整 | — |
| **不变** | 全局 `-q` | 已覆盖静默需求 | — |

---

## 2. 任务分解

### A. `-f` 别名（简单，单文件修改）

| 任务 | 文件 | 操作 |
|------|------|------|
| `-f` 注册 | `cmd/keygen.go:31-41` | `Name: "output, O, f"` — 在 urfave/cli v1 的 flag Name 中追加 `f` 别名 |
| 验证 | `go build ./...` | 编译通过 |

**说明**：urfave/cli v1 的 `Name: "output, O, f"` 解析为主名 `output`（`--output`），别名 `O`（`-O`）、`f`（`-f`）。`-f` 是 StringFlag，不受 main.go 的 `expandCombinedFlags` 影响（该函数仅展开已知 bool 短参）。

---

### B. `-C`/`--comment` 支持（涉及全栈）

#### B1. 接口层 — `asshc/port/keymanager.go`

| 任务 | 操作 |
|------|------|
| `Generate` | 新增 `comment string` 参数 |
| `GenerateToPath` | 新增 `comment string` 参数 |

```go
Generate(keyType string, bits int, passphrase []byte, comment string) (privPath string, pubKey []byte, fingerprint string, err error)
GenerateToPath(keyType string, bits int, passphrase []byte, comment string, outputPath string) (pubKey []byte, fingerprint string, err error)
```

#### B2. 基础设施层 — `asshc/infra/keymgr/generate.go`

| 任务 | 操作 |
|------|------|
| `marshalPrivateKey` | 签名改 `(key interface{}, comment string, passphrase []byte)`，comment 传入 `ssh.MarshalPrivateKey(key, comment)`  |
| `generateRSA` | 接收 `comment`，透传到 `marshalPrivateKey` |
| `generateEd25519` | 接收 `comment`，透传到 `marshalPrivateKey` |
| `generateECDSA` | 接收 `comment`，透传到 `marshalPrivateKey` |
| `Generate` | 接收 `comment`，透传到 generate 函数 |
| `GenerateToPath` | 接收 `comment`，透传到 generate 函数 |

#### B3. 服务层 — `asshc/service/key.go`

| 任务 | 操作 |
|------|------|
| `GenerateAndDeploy` | 新增 `comment string` 参数，透传到 `keymgr.Generate` |

#### B4. CLI 层 — `cmd/keygen.go`

| 任务 | 操作 |
|------|------|
| `-C` flag 注册 | `cli.StringFlag{Name: "comment, C", Usage: "key comment (default: user@host)"}` |
| `keygenAction` | 读取 `-C` 值，透传到 `keygenOutput`/`keygenDirect`/`keySvc.GenerateAndDeploy` |
| `keygenOutput` | 接收 `comment` 透传 |
| `keygenDirect` | 接收 `comment` 透传 |

#### B5. Mock 更新 — `asshc/service/key_test.go`

| 任务 | 操作 |
|------|------|
| `mockKeyManager` | `generateFunc` / `generateToPathFunc` 签名增加 `comment string` |
| 默认 mock | 适配新签名 |
| 相关测试 | 更新调用处 |

#### B6. 测试更新 — `asshc/infra/keymgr/keymgr_test.go`

- 所有 `Generate` / `GenerateToPath` 调用处补充 `""` comment 参数
- 新增 `TestGenerate_WithComment` / `TestGenerateToPath_WithComment` 验证 comment 写入

#### B7. `main.go` / 其他调用处

- `main.go`: `keymgr.New()` 和 `keySvc.NewKeyService()` 不受影响（comment 是方法参数非构造参数）
- 所有 `GenerateAndDeploy` 调用处补充 `""` 参数

---

### C. `-N`/`--new-passphrase` 支持（CLI 层仅需）

| 任务 | 文件 | 操作 |
|------|------|------|
| `-N` flag 注册 | `cmd/keygen.go` | `cli.StringFlag{Name: "new-passphrase, N", Usage: "passphrase for private key (default: account password)"}` |
| `keygenAction` | `cmd/keygen.go:60-114` | 读取 `-N`：`passphrase := resolvePassphrase(c.String("new-passphrase"), a.keymgr.GetAccountPassphrase())` |
| `keygenOutput` | `cmd/keygen.go:116-138` | 接收 passphrase 参数 |
| `keygenDirect` | `cmd/keygen.go:201-259` | 接收 passphrase 参数 |
| 辅助函数 | `cmd/keygen.go` 新增 | `resolvePassphrase(cliPass string, accountPass []byte) []byte` |

**无需修改 infra/port/service**：passphrase 从 CLI 层直接传入 keymgr 方法，底层已有 `passphrase []byte` 参数支持。

---

### D. 无参交互式向导

| 任务 | 文件 | 操作 |
|------|------|------|
| 检测无参状态 | `cmd/keygen.go:keygenAction` | 当 `outputPath == "" && host == "" && c.NArg() < 1` 时触发向导 |
| 交互式流程 | `cmd/keygen.go` 新增 `keygenInteractive()` | 见下方流程设计 |

**交互式流程**（复刻 ssh-keygen 体验）：

```
$ assh keygen
Generating public/private SSH key pair.
Enter file in which to save the key (~/.ssh/id_rsa):
Enter passphrase (empty for no passphrase):
Enter same passphrase again:
Your identification has been saved in ~/.ssh/id_rsa
Your public key has been saved in ~/.ssh/id_rsa.pub
Key fingerprint: SHA256:...
The key's randomart image is:       ← 可选，暂不实现
+---[RSA 4096]----+
|   ...           |
+----[SHA256]-----+
```

**设计要点**：
- 默认路径：`~/.ssh/id_<type>`（与 ssh-keygen 一致）
- 默认类型：`rsa`（保持与 `-t` 默认一致）
- 路径是目录时自动追加默认文件名（复用 `GenerateToPath` 的目录检测）
- 密码输入使用 `terminal.ReadPassword`（不回显）

---

## 3. 变更传导链

```
cmd/keygen.go
  ├── -f alias           → 仅 flag 注册文本变更
  ├── -C comment          → keygenAction → keygenOutput/keygenDirect/keySvc.GenerateAndDeploy
  │                         → service/key.go GenerateAndDeploy
  │                           → port/keymanager.go Generate / GenerateToPath
  │                             → infra/keymgr/generate.go Generate / GenerateToPath
  │                               → marshalPrivateKey(key, comment, passphrase)
  │
  ├── -N passphrase       → keygenAction → resolvePassphrase(-N, accountPass) → keygenOutput/keygenDirect
  │                         → infra/keymgr Generate / GenerateToPath (已有参数，无需修改)
  │
  └── 无参向导             → keygenInteractive() → 交互式输入 → 调用 keygenOutput
```

---

## 4. 文件变更清单

| 文件 | 变更类型 | 变更内容 |
|------|----------|----------|
| `assh/asshc/port/keymanager.go` | **修改** | `Generate`、`GenerateToPath` 新增 `comment string` 参数 |
| `assh/asshc/infra/keymgr/generate.go` | **修改** | `marshalPrivateKey` 接收 comment；所有生成函数透传 comment |
| `assh/asshc/infra/keymgr/keymgr_test.go` | **修改** | 更新调用处签名；新增 comment 测试 |
| `assh/asshc/service/key.go` | **修改** | `GenerateAndDeploy` 新增 `comment string` 参数 |
| `assh/asshc/service/key_test.go` | **修改** | mock 签名更新；调用处更新 |
| `assh/cmd/keygen.go` | **修改** | 新增 `-f`/`-C`/`-N` flags；无参交互式向导；passphrase 解析 |
| `assh/main.go` | **无变更** | comment/passphrase 从 CLI 传入，DI 不变 |
| `assh/cmd/app.go` | **无变更** | 命令注册不变 |

---

## 5. 执行顺序

```
Step 1: -f 别名           (单文件, 1 min)
Step 2: -C comment 全栈    (port → infra → service → cli → tests, ~20 min)
Step 3: -N passphrase      (CLI 层, 5 min)
Step 4: 无参交互式向导      (CLI 层, ~15 min)
Step 5: 验证 + 文档对齐     (5 min)
```

---

## 6. 验证标准

```bash
# 编译
cd asshv2 && go build ./...          # ✅

# 全量测试
cd asshv2 && go test ./... -count=1  # ✅

# 手动验证 -f 别名
cd asshv2 && go run . keygen -f /tmp/testkey     # 生成到 /tmp/testkey
cd asshv2 && go run . keygen -f /tmp/ --type ed25519  # 生成到 /tmp/id_ed25519
cd asshv2 && go run . keygen -O /tmp/testkey -O  # -O 仍可用

# 手动验证 -C comment
cd asshv2 && go run . keygen -f /tmp/ckey -C "my-key-for-work"
# 验证 /tmp/ckey.pub 末尾有 "my-key-for-work"

# 手动验证 -N passphrase
cd asshv2 && go run . keygen -f /tmp/nkey -N "secret123"
# 验证私钥需密码才能解析

# 手动验证无参交互式
cd asshv2 && go run . keygen
# 应进入交互式向导
```

---

## 7. 执行记录

### 7.1 实现完成情况

| 任务 | 状态 | 说明 |
|------|:----:|------|
| `-f` 别名 | ✅ | `Name: "output, O, f"` 注册完成 |
| `-C`/`--comment` | ✅ | 全栈透传（port→infra→service→CLI），默认 "user@host" |
| `-N`/`--new-passphrase` | ✅ | `resolvePassphrase()` 实现，CLI 优先于 account password |
| 无参交互式向导 | ✅ | `keygenInteractive()` 实现完整 ssh-keygen 风格流程 |

### 7.2 接口签名变更

| 文件 | 变更 |
|------|------|
| `asshc/port/keymanager.go` | `Generate` +4参数 `comment` / `GenerateToPath` +4参数 `comment` |
| `asshc/infra/keymgr/generate.go` | `marshalPrivateKey(key, comment, passphrase)` / 所有 generate 函数透传 comment |
| `asshc/service/key.go` | `GenerateAndDeploy` +4参数 `comment` |
| `asshc/service/key_test.go` | mock 签名更新（`generateFunc(keyType, bits, passphrase, comment)`） |
| `asshc/infra/keymgr/keymgr_test.go` | 56 tests（35原测试 + 21 GenerateToPath 测试），37处签名更新 |
| `cmd/keygen.go` | 新增 `-C`/`-N`/`-f` flags、`keygenInteractive()`、`resolvePassphrase()` |

### 7.3 关键决策（ADR-025）

- **新建 ADR-025**：ADR-012（CLI参数对齐）和 ADR-018（known-servers）均不涉及 keygen 命令参数
- **`-f` 作为 `--output` 别名**：urfave/cli v1 `Name: "output, O, f"` 解析为 `-f`/`-O`/`--output`
- **comment 默认 "user@host"**：未提供 `-C` 时，`marshalPrivateKey` 传入 "user@host"
- **`-N` 优先级**：CLI passphrase > account password > nil
- **`-P` 冲突解决**：保持 `-P` = 登录密码，不添加 old-passphrase 参数
- **无参行为变更**：`c.NArg() < 1` 从显示 help 改为触发 `keygenInteractive()` 交互式向导

### 7.4 文档对齐

| 文档 | 变更 |
|------|------|
| `docs/TODO.md` | P6.4-d 条目扩展为详细记录表（-f 别名/-C/-N/无参向导/comment 默认值） |
| `docs/modules/key-management.md` | P6.1 接口签名更新 / P6.2-a 生成设计修订 / P6.4 命令参数+Passphrase优先级+四种模式 |
| `docs/adrs/025-ssh-keygen-参数对齐.md` | 已创建（决策记录） |
| `docs/adrs.md` | 新增 ADR-025 条目（accepted）、accepted 20→21 |
| `AGENTS.md` | Phase 6 状态更新为 "✅ P6.4 keygen + P6.7 DI 完成 (2026-05-12)" |

### 7.5 验证

```bash
cd asshv2 && go build ./...      # ✅
cd asshv2 && go vet ./...        # ✅
cd asshv2 && go test ./... -count=1  # ✅ 56 tests pass
```

### 7.6 git 记录

```
docs/adrs/025-ssh-keygen-参数对齐.md    [新建] ADR 决策记录
docs/adrs.md                           [更新] 新增 ADR-025 条目
AGENTS.md                              [更新] Phase 6 状态
asshv2/cmd/keygen.go                   [更新] -C/-N/-f flags, keygenInteractive, resolvePassphrase
asshv2/asshc/port/keymanager.go       [更新] Generate/GenerateToPath + comment 参数
asshv2/asshc/infra/keymgr/generate.go  [更新] marshalPrivateKey + comment，透传全栈
asshv2/asshc/service/key.go            [更新] GenerateAndDeploy + comment 参数
asshv2/asshc/service/key_test.go       [更新] mock 签名更新
asshv2/asshc/infra/keymgr/keymgr_test.go [更新] 37处签名更新 + 21 GenerateToPath tests
```
