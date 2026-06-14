# Config Module

**文件**: `asshv2/config/config.go` + `asshv2/config/path.go`

## 功能

配置路径管理和全局变量：

- **初始化流程**：创建 `~/.assh/v2` 目录
- **路径管理**：`ExpandPath`、`EnsureDir`、`FileExists` 工具函数
- **全局变量**：`DbPath`、`PasswordHash`、`LogLevel`、七牛云配置等
- **数据库文件**：`~/.assh/v2/asshv2.db`（SQLite 数据库，由 `infra/store` 管理）
- **配置文件**：`~/.assh/v2/assh.yml`（RSA 加密，Phase 1 已实现）

> **注意**：密码管理（account）已从 Phase 2 移除，延后至 Phase 8（云同步 Qiniu）时实现。
> 当前服务器密码使用自动生成的 AES-256 静态密钥加密，用户无需输入主密码。