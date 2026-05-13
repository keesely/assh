# ADR-025: assh keygen 命令 ssh-keygen 参数对齐

## Status

accepted

## Date

2026-05-12

## Context

Phase 6 (密钥管理) 的 `assh keygen` 命令需要与 OpenSSH 标准工具 `ssh-keygen` 的参数命名对齐，降低用户学习成本，提供更自然的 CLI 体验。

### 决策驱动因素

1. **零学习成本**：`ssh-keygen` 是 SSH 用户最熟悉的密钥生成工具，参数对齐让 assh keygen 可以"直觉使用"
2. **参数语义清晰**：`ssh-keygen` 的参数（如 `-f` 输出路径、`-C` 注释、`-N` 新密码）语义经过社区验证
3. **冲突约束**：`assh` 全局和直连模式已占用部分短参（`-P` = 登录密码），需要明确如何处理 `-P` 与 ssh-keygen 旧密码语义冲突

### 已占用参数现状

| 短参 | ssh-keygen 语义 | assh 现状 | 冲突 |
|------|----------------|-----------|------|
| `-f` | 输出文件路径 | 未使用 | ✅ 可用 |
| `-P` | 旧密码短语 | 直连登录密码 | ❌ **冲突** |
| `-C` | 密钥注释 | 未使用 | ✅ 可用 |
| `-N` | 新密码短语 | 未使用 | ✅ 可用 |
| `-q` | 静默模式 | 全局 -q 已覆盖 | ✅ 已覆盖 |

### 用户调研结论

通过问卷调研确认：

| 议题 | 决策 |
|------|------|
| `-f` 输出路径别名 | ✅ 添加（与 `-O`/`--output` 并存） |
| 无参行为 | ✅ 触发交互式向导（仿 ssh-keygen） |
| `-C` 密钥注释 | ✅ 添加 `--comment`/`-C` |
| `-N` 新密码短语 | ✅ 添加，留空时回退 account password |
| `-P` 冲突 | ✅ 保持 `-P` = 登录密码；旧密码走交互式输入 |
| RSA 默认位数 | ✅ 保持 4096/2048（不随 ssh-keygen 3072/1024） |
| `-q` 静默模式 | ✅ 用全局 `-q` 即可，无需单独处理 |

## Decision

### 1. 参数对齐表

| 参数 | assh keygen | ssh-keygen | 语义 |
|------|-------------|-----------|------|
| `-f` | `--output`/`-O`/`-f` 三别名 | `-f output_keyfile` | 输出文件路径 |
| `-C` | `--comment`/`-C` | `-C comment` | 密钥注释（默认 "user@host"） |
| `-N` | `--new-passphrase`/`-N` | `-N new_passphrase` | 私钥密码短语（优先级最高） |
| `-t` | `--type`/`-t`（已有） | `-t` | 密钥类型 |
| `-b` | `--bits`/`-b`（已有） | `-b` | 密钥位数 |
| `-q` | 全局已覆盖 | `-q` | 静默模式 |

### 2. `-P` 冲突解决方案

保持 `-P` 作为 assh 直连模式登录密码（已有行为），**不添加 `-P` 作为旧密码短语参数**。旧密码场景（如 ssh-keygen 的 `-p` 改密码操作）采用交互式输入，不暴露命令行参数。

### 3. 无参交互式向导

`assh keygen` 无参时触发交互式向导，流程与 ssh-keygen 一致：

```
$ assh keygen
Generating public/private SSH key pair.
Key type (rsa/ed25519/ecdsa) [rsa]:          ← 询问类型（默认 rsa）
Key bits [4096]:                              ← 询问位数（RSA/ECDSA）
Enter file in which to save the key (~/.ssh/id_rsa):   ← 询问路径
Enter passphrase (empty for no passphrase):            ← 询问密码
Enter same passphrase again:                            ← 确认密码
```

### 4. Passphrase 优先级

```
-N 指定 → 使用 -N 值
-N 未指定 → 回退 account password（Phase 8 账号密码）
未设 account password → 无加密（空 passphrase）
```

### 5. Comment 默认值

ssh-keygen 默认 comment 为 `user@host`。assh 采用相同默认值，在生成时填充。

## Consequences

### Positive

- **零学习成本**：熟悉 ssh-keygen 的用户无需查看帮助即可使用 assh keygen
- **交互体验一致**：无参时触发交互式向导，提供与 ssh-keygen 相似的引导体验
- **参数语义清晰**：`-f`/`-C`/`-N` 语义与 ssh-keygen 完全一致

### Negative

- **参数签名变更**：port.KeyManager 接口的 `Generate`/`GenerateToPath` 增加 `comment string` 参数，需更新所有调用处（已更新）
- **`-P` 语义差异**：assh `-P` = 登录密码，与 ssh-keygen `-P` = 旧密码短语 完全不同，可能造成混淆
- **无参行为变更**：原来无参显示 help，现在进入交互式向导（需用户知晓）

### Affected Modules

| 范围 | 文件 | 变更 |
|------|------|------|
| **Port 接口** | `asshc/port/keymanager.go` | `Generate`/`GenerateToPath` 新增 `comment string` 参数 |
| **Infra 实现** | `asshc/infra/keymgr/generate.go` | `marshalPrivateKey` 接收 comment → `ssh.MarshalPrivateKey(key, comment)` |
| **Service 层** | `asshc/service/key.go` | `GenerateAndDeploy` 新增 `comment string` 参数 |
| **CLI 层** | `cmd/keygen.go` | 新增 `-f`/`-C`/`-N` flags、`keygenInteractive()` 无参向导 |
| **测试** | `keymgr_test.go`/`key_test.go` | 所有调用处补充 `""` 参数（37 处） |
| **Mock** | `key_test.go` mockKeyManager | 签名更新 |

## Implementation Notes

### 签名变更（向后兼容破坏）

```go
// 旧签名（v2.0.0 已废弃）
Generate(keyType string, bits int, passphrase []byte) (privPath, pubKey, fingerprint, error)
GenerateToPath(keyType string, bits int, passphrase []byte, outputPath string) (pubKey, fingerprint, error)
GenerateAndDeploy(name, keyType string, bits int) error

// 新签名（v2.0.0 当前）
Generate(keyType string, bits int, passphrase []byte, comment string) (privPath, pubKey, fingerprint, error)
GenerateToPath(keyType string, bits int, passphrase []byte, comment string, outputPath string) (pubKey, fingerprint, error)
GenerateAndDeploy(name, keyType string, bits int, comment string) error
```

### CLI 参数注册（urfave/cli v1）

```go
cli.StringFlag{Name: "output, O, f", ...}       // -f 是 --output 别名
cli.StringFlag{Name: "comment, C", ...}          // -C
cli.StringFlag{Name: "new-passphrase, N", ...}   // -N
```

## Related

- ADR-012: CLI 参数对齐与命令架构
- ADR-018: known-servers 隐性表
- Phase 6: 密钥管理模块
- docs/modules/key-management.md
