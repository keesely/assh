# Server Data Module

**文件**: `asshv2/asshc/domain/server.go` + `asshv2/asshc/infra/store/filestore.go`

## 功能

管理服务器列表的核心数据结构：

- **数据结构**：`map[string]map[string]*Server`（群组 -> 服务器名 -> 服务器信息）
- **命名格式**：`group.server_name`（群组可省略）
- **数据加密**：AES-CBC 模式加密，带 MD5 签名验证
- **密钥管理**：每个服务器可独立关联公钥文件