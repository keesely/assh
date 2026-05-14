# ADR-024: v1→v2 配置导入指令

## Status

proposed

## Date

2026-05-10

## Context

v1 和 v2 的配置完全不兼容（破坏性更新）：

- **v1 存储**：使用 RSA + AES-CBC 加密的 JSON 文件（`~/.assh/data/servers.db`），密钥文件存储在 `~/.assh/data/{server-name}/` 目录下，密码通过 RSA 加密的 `.account` 文件管理
- **v2 存储**：使用 SQLite（`~/.assh/v2/asshv2.db`），AES-GCM 加密密码，纯 Go 无 CGO

用户从 v1 升级到 v2 时需要手动重建所有服务器配置，体验差、易出错。需要提供自动导入机制，让用户平滑过渡。

## Decision

实现 `assh import v1` 命令，自动扫描 v1 配置并转入 v2。流程如下：

1. **读取 v1 RSA 私钥**：从 `~/.assh/.rsa` 读取 RSA 私钥（PKCS#1 PEM 格式）
2. **解密 .account**：使用 RSA PKCS1v15 解密 `~/.assh/.account`，获取密码的 MD5 哈希值
3. **解码 servers.db**：使用 MD5 哈希值作为 AES-CBC 密钥，解密 `~/.assh/data/servers.db` 获取 JSON 数据
4. **解析 v1 Server JSON**：解析为 `map[string]map[string]*Server` 格式（分组 → 服务器名 → 配置）
5. **收集密钥文件**：扫描 `~/.assh/data/{group}/{server}/` 目录，收集 `id_rsa` 密钥文件路径
6. **写入 v2 SQLite**：将转换后的 Server 数据通过 v2 的 Store API 写入 SQLite 数据库

技术细节：
- 解密使用 v2 crypto 包中的 `RSADecryptPKCS1v15()` 和 `AESDecryptCBC()`
- v1 Server 结构体定义在 `backup/asshc/server.go` 中
- ~~导入命令作为 `assh` 的子命令（非 `assh-fs`）~~ 为了不影响主体架构设计，导入命令使用独立二进制+独立导入包进行扩展，不参与主框架设计
- ~~导入成功后提示用户可删除 v1 数据~~， 导入成功后不处理旧数据，防止新版本问题可以直接是有 v1 接管

## Consequences

### 正面
- 用户可以从 v1 无缝迁移到 v2，保存全部历史配置
- 支持密码和密钥两种认证方式的自动迁移
- 保留 v1 的分组结构

### 负面
- 需要维护 v1 兼容性代码（数据解密逻辑），增加 v2 的二进制体积
- v1 的 `kiris` 库中的 `AESEncrypt`/`AESDecrypt` 行为需要精确复现（IV 生成、PKCS7 填充等）

### 影响范围
- 新增文件：`asshc/service/importer.go` — v1 导入逻辑
- 新增文件：`cmd/import.go` — import 子命令定义
- 新增测试：`asshc/service/importer_test.go`
- v1 兼容层（见 ADR-016）与本导入命令形成互补，导入后 v1 数据不再依赖

## Related

- ADR-016: v1 兼容层设计
- docs/TODO.md: Phase 6 密钥管理 — 导入后密钥文件管理
