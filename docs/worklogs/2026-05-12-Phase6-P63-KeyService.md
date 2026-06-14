# Phase 6 P6.3 KeyService 服务层实现

**日期**: 2026-05-12
**阶段**: Phase 6 (Key Management) — P6.3 KeyService 服务层
**版本**: v2.0.0-phase-6 (开发中)

---

## 1. 完成内容

### P6.3-a KeyService (`asshc/service/key.go`)

实现了 `KeyService` 结构体，包含两个核心方法：

**`GenerateAndDeploy(name, keyType, bits)`** — 完整密钥生成部署流程：
1. 从 repo 获取服务器配置（确保服务器存在）
2. 调用 `keymgr.Generate()` 生成密钥对
3. 调用 `keymgr.DeployPublicKey()` 部署公钥
4. 更新服务器 `Auth.KeyFile` 配置

**部署容错策略**：
- `DeployPublicKey` 当前为桩实现（P6.2-b 暂缓），桩错误 `"deploy public key is not yet implemented (P6.2-b deferred)"` 被容错
- 密钥仍会成功生成并配置，仅部署步骤跳过
- 真正的部署错误会导致操作回退（清理已生成的私钥文件）

**`HandleKeyFlag(name, keyValue) (keyFilePath, err)`** — 处理 -k 参数三种场景：

| keyValue | 行为 | 说明 |
|----------|------|------|
| `""` | `GenerateAndDeploy(name, "rsa", 4096)` | 生成默认 RSA 4096 密钥 |
| `"-"` | `ReadKeyFromStdin()` + 更新配置 | 从 stdin 读取密钥 |
| 文件路径 | `BackupKey(src, backupPath)` + 更新配置 | 备份已有密钥文件 |

**方法签名变更**（vs 初始设计）：
- `HandleKeyFlag` 返回 `(string, error)` 而非仅 `error` — 返回密钥文件路径供调用方显示/使用

### P6.3-b 单元测试 (`asshc/service/key_test.go`)

**共 16 个测试，全部通过**：

**GenerateAndDeploy 测试（7 个）**：
| 测试 | 场景 | 验证 |
|------|------|------|
| `TestGenerateAndDeploy_Success` | 正常流程 | 密钥生成、部署、配置更新全部成功 |
| `TestGenerateAndDeploy_EmptyName` | 空名称 | 返回 `ErrInvalidName` |
| `TestGenerateAndDeploy_ServerNotFound` | 不存在 | 返回错误 |
| `TestGenerateAndDeploy_GenerateFails` | 生成失败 | 错误传播 |
| `TestGenerateAndDeploy_DeployStub` | 桩实现 | 被容错，密钥仍配置成功 |
| `TestGenerateAndDeploy_DeployRealError` | 真正部署错误 | 操作回退，错误返回 |
| `TestGenerateAndDeploy_UpdateConfigFails` | 配置更新失败 | `mockRepoFailSet` 模拟 Set 错误 |

**HandleKeyFlag 测试（8 个）**：
| 测试 | 场景 | 验证 |
|------|------|------|
| `TestHandleKeyFlag_EmptyName` | 空名称 | 返回 `ErrInvalidName` |
| `TestHandleKeyFlag_ServerNotFound` | 不存在 | 错误返回 |
| `TestHandleKeyFlag_EmptyString` | `keyValue=""` | 触发 GenerateAndDeploy（rsa+4096） |
| `TestHandleKeyFlag_Dash` | `keyValue="-"` | 从 stdin 读取并配置 |
| `TestHandleKeyFlag_StdinFails` | stdin 为空 | 错误返回 |
| `TestHandleKeyFlag_FilePath` | 文件路径 | Backup + 配置更新 |
| `TestHandleKeyFlag_FileNotFound` | 文件不存在 | 错误返回 |
| `TestHandleKeyFlag_BackupFails` | 备份失败 | 错误返回 |
| `TestHandleKeyFlag_UpdateConfigFails` | Set 失败 | 错误返回 |

## 2. 设计决策

### GenerateAndDeploy 部署容错
- 使用 `deployStubError` 常量匹配桩错误信息（精确字符串匹配）
- 非桩错误导致操作回退：清理已生成私钥 + 返回错误
- 权衡：精确字符串匹配不如类型判断优雅，但桩实现仅为临时方案（P6.2-b 后会替换）
- 替代方案：使用 sentinel error 进行比较 → 但 infra 桩在当前包外，引入耦合

### HandleKeyFlag 路径返回
- 返回 `(string, error)` 而非仅 `error`
- 原因：CLI 层需要显示更新后的密钥路径，避免二次查询

### SSHConnector 预留
- `KeyService` 包含 `ssh port.SSHConnector` 字段
- 当前部署由 `KeyManager.DeployPublicKey()` 内部处理 SSH 连接
- 预留字段为后续精细部署控制（如连接复用、代理隧道等）

## 3. 文档对齐

- [x] `docs/TODO.md` — P6.3 标记 ✅，新增 P6.3-b 行
- [x] `docs/modules/key-management.md` — P6.3 标记 ✅，方法签名同步
- [x] `docs/architecture.md` — 新增 keymgr/ 模块条目
- [x] `docs/adrs.md` — 无需变更（无新增 ADR）

## 4. 下一步待办

后续 Phase 6 任务按优先级：

| 优先级 | 任务 | 说明 |
|:------:|------|------|
| ⭐⭐ | **P6.4** keygen 命令 | `cmd/keygen.go` — CLI 入口 |
| ⭐⭐ | **P6.6-a** SSH 备份优先 | `client.go` tryKeyFile 优先查 data/keys/ |
| ⭐⭐ | **P6.6-b** known-servers 查找 | 直连时查 key_backup_path |
| ⭐⭐ | **P6.7-b~d** DI 注入 | `main.go` + `cmd/app.go` 注入 KeyService |
| ⏳ | **P6.5-e** keygen 关联 known-servers | deploy 成功后更新 key_backup_path |
| ⏸️ | **P6.2-b** 公钥部署 | 暂缓 |

## 5. 验证记录

```bash
$ cd asshv2 && go build ./...      # ✅ 编译通过
$ cd asshv2 && go test ./... -count=1  # ✅ 全量测试通过

service package: 48 tests, 0 failures
```

## 6. 代码变更摘要

```
asshv2/asshc/service/
  ├── key.go         # 新增 — KeyService 实现（GenerateAndDeploy/HandleKeyFlag）
  └── key_test.go    # 新增 — 16 个单元测试
```
