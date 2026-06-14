# ADR-009: 服务器密码采用 AES-256-GCM 静态密钥加密

## Status

accepted

## Date

2026-05-09

## Context

v1 方案中，服务器数据使用用户主密码（account password）进行 AES-CBC 加密。
每次使用 assh 前用户必须先设置/验证密码，流程繁琐。

Phase 2 重构时，需要决定如何保护存储在 SQLite 中的服务器密码字段。

## Decision

采用 **AES-256-GCM 静态密钥** 方案：

### 密钥生命周期

```
首次运行 NewStore(dbPath):
  ├── config 表中有 key 记录？ → 加载已有密钥
  └── config 表中无 key 记录？ → 生成 32 字节随机密钥
                                → base64 编码
                                → 存入 config 表
```

### 密码加解密流程

```
Set(name, server):
  password_encrypted = AES-GCM-Encrypt(server.Auth.Password, staticKey)
  存入 SQLite password_encrypted 列

Get(name):
  password_encrypted = 从 SQLite 读取
  server.Auth.Password = AES-GCM-Decrypt(password_encrypted, staticKey)
```

### 技术选型

| 项目 | 选择 | 理由 |
|------|------|------|
| 加密算法 | AES-256-GCM | 内置认证加密，消除 CBC 的 padding oracle 风险 |
| 密钥长度 | 256 位（32 字节） | 符合 AES-256 标准 |
| 密钥存储 | SQLite config 表 | 随数据库一同同步，支持多机器共享 |
| 密钥编码 | base64 | 文本友好，便于 config 表存储和调试 |

## Consequences

1. **优点**：
   - **无感体验**：用户无需输入密码即可使用 assh
   - **自动管理**：密钥首次自动生成，后续完全无感
   - **可同步**：密钥随 SQLite 一起同步（Phase 8），多机器共享
   - **安全**：AES-GCM 提供认证加密（integrity + confidentiality）

2. **缺点**：
   - 数据库文件本身不加密，依赖文件系统权限保护（`~/.assh/v2/` 目录 0700）
   - 静态密钥的「静态」特性意味着：获取数据库文件即获取密钥

3. **风险缓解**：
   - SQLite 文件存储在 `~/.assh/v2/`（默认 0700 权限）
   - 服务器密码本身是 SSH 凭据，安全性等价于 SSH 私钥文件保护
   - Phase 8 的 account 密码可额外加密数据库

## Related

- Related ADRs: ADR-008 (SQLite), ADR-004 (crypto 模块)
