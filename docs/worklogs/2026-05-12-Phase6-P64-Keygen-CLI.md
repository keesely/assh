# Phase 6 P6.4 keygen CLI 命令 + P6.7 DI 注入

**日期**: 2026-05-12
**阶段**: Phase 6 (Key Management) — P6.4 CLI 命令 + P6.7 DI 注入
**版本**: v2.0.0-phase-6 (开发中)

---

## 1. 完成内容

### P6.4-a keygen 命令 (`cmd/keygen.go`)

`assh keygen` 命令支持两种运行模式：

**已命名服务器模式：**
```
assh keygen myserver                   RSA 4096 + 部署 + 更新配置
assh keygen myserver --type ed25519    Ed25519 密钥
assh keygen myserver --type ecdsa --bits 521  ECDSA P-521
```

**直连模式（无需服务器配置）：**
```
assh keygen -H 192.168.1.1 -u root -P password
```

**命令参数：**
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--type, -t` | 密钥类型：rsa/ed25519/ecdsa | rsa |
| `--bits, -b` | 密钥位数 | 4096 |
| `-H, --host` | 直连主机地址 | — |
| `-u, --user` | 直连用户名 | — |
| `-l, --login` | 用户名（同 --user） | — |
| `-p, --port` | SSH 端口 | 22 |
| `-P, --password` | 直连密码 | — |

### P6.4-b 命令注册 (`cmd/app.go`)
- `registerKeygenCommands()` 方法注册 keygen 到 CLI 命令列表
- `registerCommands()` 中调用
- `registerCompletionHints()` 中增加 "keygen" 的服务器名补全

### P6.7-d App 签名变更 (`cmd/app.go`)
- `NewApp` 签名增加 `keySvc *service.KeyService` 和 `keymgr port.KeyManager`
- `App` 结构体新增 `keySvc` 和 `keymgr` 字段
- `NewFSApp` 签名不变（assh-fs 暂无需 keygen）

### P6.7-b DI 注入 (`main.go`)
- 导入 `assh/asshc/infra/keymgr` 包
- 创建 KeyManager：`keymgr.New(config.KeysDir, nil)`（nil passphrase，账号密码待 Phase 8）
- 创建 KeyService：`service.NewKeyService(km, repo, connector)`
- 注入到 `cmd.NewApp()`

## 2. 设计决策

### 直连模式部署分离
- 直连模式下密钥生成直接使用 `a.keymgr`（port.KeyManager），而非 `a.keySvc`（KeyService）
- 原因：KeyService.GenerateAndDeploy 依赖 repo 中的服务器配置，直连模式没有配置
- 直连模式流程：keymgr.Generate → keymgr.DeployPublicKey → known_servers 记录

### 桩错误容错
- 直连模式和命名服务器模式都对 DeployPublicKey 的桩错误进行容错
- 使用 `service.DeployStubError` 常量进行比较（导出版本，避免字符串重复）

## 3. 文档对齐

- [x] `docs/TODO.md` — P6.4 ✅, P6.7-b/d ✅
- [x] `docs/modules/key-management.md` — P6.4 ✅, P6.7 ✅
- [x] `docs/worklogs/2026-05-12-Phase6-P64-Keygen-CLI.md` — 本文件

## 4. 下一步待办

| 优先级 | 任务 | 说明 |
|:------:|------|------|
| ⭐⭐ | **P6.6-a** SSH 备份优先 | `client.go` tryKeyFile 优先查 data/keys/ |
| ⭐⭐ | **P6.6-b** known-servers 查找 | 直连时查 key_backup_path |
| ⏳ | **P6.4-c** add/set -k 增强 | 空参/stdin/备份逻辑（依赖 P6.6） |
| ⏳ | **P6.5-e** keygen 关联 known-servers | key_backup_path 关联 |
| ⏸️ | **P6.2-b** 公钥部署 | 暂缓 |

## 5. 验证记录

```bash
$ cd asshv2 && go build ./...       # ✅ 编译通过
$ cd asshv2 && go vet ./...         # ✅ 静态检查通过
$ cd asshv2 && go test ./... -count=1  # ✅ 全量测试通过
$ cd asshv2 && go run . help        # ✅ keygen 命令可见
$ cd asshv2 && go run . keygen --help  # ✅ help 正确显示
```

## 6. 代码变更摘要

```
asshv2/
├── cmd/
│   ├── keygen.go        # 新增 — keygen 命令（命名服务器 + 直连两种模式）
│   └── app.go           # 修改 — NewApp 签名变更 + App 新增字段 + 注册 keygen
├── main.go              # 修改 — DI 注入 KeyManager + KeyService
└── asshc/service/
    └── key.go           # 修改 — 导出 DeployStubError 常量
```
