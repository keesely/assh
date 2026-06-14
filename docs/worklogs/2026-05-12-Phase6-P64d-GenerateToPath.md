# Phase 6 P6.4-d GenerateToPath 实现 + --output 独立模式

**日期**: 2026-05-12
**阶段**: Phase 6 (Key Management) — P6.4-d GenerateToPath + `--output` CLI
**版本**: v2.0.0-phase-6 (开发中)

---

## 1. 完成内容

### P6.4-d GenerateToPath 接口 + infra 实现

| 层级 | 文件 | 变更 |
|------|------|------|
| **port 接口** | `asshc/port/keymanager.go` | 新增 `GenerateToPath(keyType, bits, passphrase, outputPath)` 方法 |
| **infra 实现** | `asshc/infra/keymgr/generate.go` | 实现 `GenerateToPath` + `resolveKeyOutputPath` + `defaultKeyName` 辅助函数 |
| **infra 测试** | `asshc/infra/keymgr/keymgr_test.go` | 21 个新增测试覆盖 GenerateToPath 全场景 |
| **CLI 命令** | `cmd/keygen.go` | 新增 `--output`/`-O` 标志 + `keygenOutput` 处理函数 + `resolveOutputFiles` |

### `GenerateToPath` 行为

与 `Generate()` 不同，`GenerateToPath` 不保存到 `data/keys/{fingerprint}.pem`，而是直接写入调用者指定的路径：

- 私钥 → `outputPath`（权限 0600）
- 公钥 → `outputPath + ".pub"`（权限 0644，ssh-keygen 兼容格式，带尾部换行符）
- **目录自动命名**：`outputPath` 以 `/` 结尾或指向已存在目录 → 追加 `id_rsa`/`id_ed25519`/`id_ecdsa`
- **父目录自动创建**：目录不存在时自动 `mkdir -p`
- **直接覆盖**：文件已存在时直接覆盖，不询问

### CLI `--output` 独立生成模式

keygen 命令现在支持三种运行模式（优先级从高到低）：

1. **独立生成模式**（新增）— `assh keygen --output ~/.ssh/mykey`：
   - 调用 `keymgr.GenerateToPath()`，不涉及服务器操作
   - 输出私钥路径、公钥路径、指纹、公钥内容
   - 支持目录路径自动命名：`--output ~/.ssh/` → `~/.ssh/id_rsa`
2. **已命名服务器模式** — `assh keygen myserver`（现有）
3. **直连模式** — `assh keygen -H host -u root -P pass`（现有）

### 新增测试覆盖（21 个）

| 测试 | 场景 |
|------|------|
| `TestGenerateToPath_RSA` | RSA 4096 → 文件路径，验证私钥/公钥文件正确 |
| `TestGenerateToPath_RSA_DefaultBits` | RSA 0 → 4096 默认值 |
| `TestGenerateToPath_RSA_2048` | RSA 2048 最小安全位数 |
| `TestGenerateToPath_RSA_BitsTooSmall` | RSA 1024 应报错 |
| `TestGenerateToPath_Ed25519` | Ed25519 → 文件路径 |
| `TestGenerateToPath_Ed25519_IgnoredBits` | Ed25519 bits=999 被忽略 |
| `TestGenerateToPath_ECDSA` | ECDSA P-384 → 文件路径 |
| `TestGenerateToPath_ECDSA_P256` | ECDSA P-256 |
| `TestGenerateToPath_ECDSA_P521` | ECDSA P-521 |
| `TestGenerateToPath_ECDSA_DefaultBits` | ECDSA 0 → 384 默认值 |
| `TestGenerateToPath_ECDSA_InvalidBits` | ECDSA 192 应报错 |
| `TestGenerateToPath_InvalidKeyType` | dsa 类型应报错 |
| `TestGenerateToPath_DirectoryOutput` | 目录路径 → 自动 id_ed25519 命名 |
| `TestGenerateToPath_TrailingSlashDirectory` | `dir/` 结尾 → 自动命名 |
| `TestGenerateToPath_WithPassphrase` | 密码加密私钥验证 |
| `TestGenerateToPath_PublicKeyPermissions` | 公钥 0644，私钥 0600 |
| `TestGenerateToPath_AutoCreateParentDir` | 父目录自动创建 |
| `TestGenerateToPath_OverwriteExisting` | 覆盖已存在文件 |
| `TestGenerateToPath_RSA_DirectoryAutoNames` | RSA → id_rsa |
| `TestGenerateToPath_ECDSA_DirectoryAutoNames` | ECDSA → id_ecdsa |
| `TestGenerateToPath_Ed25519_DirectoryAutoNames` | Ed25519 → id_ed25519 |

---

## 2. 设计决策

### `GenerateToPath` 放在 `port.KeyManager` 接口

最初考虑在 CLI 层内置路径解析逻辑直接调用底层生成函数，但为了架构清晰和可测试性：
- **决定**：在 `port.KeyManager` 接口新增 `GenerateToPath`，infra 层提供实现
- **理由**：保持三层架构的完整依赖倒置，CLI 层不直接依赖 infra，易于 mock 测试

### `resolveKeyOutputPath` vs `resolveOutputFiles` 双实现

- infra 层的 `resolveKeyOutputPath` 是实际解析逻辑（决定写入路径）
- CLI 层的 `resolveOutputFiles` 是镜像实现（仅用于显示，需保持逻辑一致）
- **决定**：接受重复，因为两者在不同包且职责不同。后续可考虑导出 infra 的路径辅助函数

### 目录检测策略

```go
// 以 / 结尾 → 显式目录标记
if strings.HasSuffix(expanded, "/") {
    return filepath.Join(expanded, defaultKeyName(keyType)), nil
}

// os.Stat 检查 → 已存在目录
if info, statErr := os.Stat(expanded); statErr == nil && info.IsDir() {
    return filepath.Join(expanded, defaultKeyName(keyType)), nil
}

// 否则作为文件路径
return expanded, nil
```

---

## 3. 文档对齐

- [x] `docs/TODO.md` — 新增 P6.4-d 条目 ✅，更新 P6.2-d 测试数量 (35→56)
- [x] `docs/modules/key-management.md` — P6.1 接口新增 GenerateToPath，P6.2-d 测试数更新，P6.4 CLI 新增 --output + 独立模式说明
- [x] `docs/worklogs/2026-05-12-Phase6-P64d-GenerateToPath.md` — 本文件

---

## 4. 下一步待办

| 优先级 | 任务 | 说明 |
|:------:|------|------|
| ⭐⭐ | **P6.6-a** SSH 备份优先 | `client.go` tryKeyFile 优先查 data/keys/ |
| ⭐⭐ | **P6.6-b** known-servers 查找 | 直连时查 key_backup_path |
| ⏳ | **P6.4-c** add/set -k 增强 | 空参/stdin/备份逻辑（依赖 P6.6） |
| ⏳ | **P6.5-e** keygen 关联 known-servers | key_backup_path 关联 |
| ⏸️ | **P6.2-b** 公钥部署 | 暂缓 |

---

## 5. 验证记录

```bash
$ cd asshv2 && go build ./...       # ✅ 编译通过
$ cd asshv2 && go vet ./...         # ✅ 静态检查通过
$ cd asshv2 && go test ./asshc/infra/keymgr/... -v -count=1 -run 'TestGenerateToPath'
# ✅ 21/21 GenerateToPath 测试通过
$ cd asshv2 && go test ./... -count=1
# ✅ 全部包测试通过（keymgr 56 个测试）
```

---

## 6. 代码变更摘要

```
asshv2/
├── asshc/
│   ├── port/
│   │   └── keymanager.go           # 修改 — 新增 GenerateToPath 接口方法
│   └── infra/keymgr/
│       ├── generate.go             # 修改 — 新增 GenerateToPath + resolveKeyOutputPath + defaultKeyName
│       └── keymgr_test.go          # 修改 — 新增 21 个 GenerateToPath 测试
└── cmd/
    └── keygen.go                   # 修改 — 新增 --output/-O 标志 + keygenOutput + resolveOutputFiles
```
