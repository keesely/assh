# Config Module

**文件**: `asshv2/config/config.go` + `asshv2/config/path.go`

## 功能

初始化和数据加密：

- **初始化流程**：创建 `~/.assh` 目录，自动生成 RSA 密钥对 (`~/.assh/.rsa`)
- **密码管理**：`account` 命令设置安全密码，用于加密服务器数据
- **数据存储位置**：`~/.assh/data/servers.db`（AES 加密）
- **配置文件**：`~/.assh/assh.yml`（RSA 加密）