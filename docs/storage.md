# Storage

- 存储根目录：`~/.assh/v2`
- 日志文件：`/tmp/assh.log`（默认，未配置时）
- 数据库文件：`~/.assh/v2/asshv2.db`（SQLite，含 servers 表 + config 表）
- 配置文件：`~/.assh/v2/assh.yml`（RSA 加密，Phase 1 已实现）
- 密钥文件：`~/.assh/v2/.rsa`, `~/.assh/v2/.rsa.pub`（RSA 密钥对）
- 密码文件：`~/.assh/v2/.account`（Phase 6 云同步时实现）

## 数据库结构

数据库 `asshv2.db` 包含两张表：

| 表 | 用途 | 说明 |
|----|------|------|
| `servers` | 服务器列表 | 含 AES-GCM 加密的密码列 |
| `config` | 应用配置 | 存储 AES 静态密钥等 |

> 静态密钥（AES-256）首次运行时自动生成，base64 编码存入 `config` 表。
> 密钥随数据库一起同步（Phase 6 云同步），支持多机器共享。