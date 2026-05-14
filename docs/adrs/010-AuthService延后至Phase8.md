# ADR-010: AuthService 延后至 Phase 8，与云同步整合

## Status

accepted

## Date

2026-05-09

## Context

原始 Phase 2 计划中包含 `P2.6 AuthService`，用于：

- 设置/修改安全密码（account password）
- 使用 RSA 加密存储密码到 `~/.assh/.account`
- 在数据读写时验证密码

但 Phase 2 的设计已发生根本性变化：

1. **SQLite 替代加密 JSON**（ADR-008）：不再需要密码来解密数据文件
2. **静态密钥加密**（ADR-009）：服务器密码使用自动密钥，无需用户干预
3. **account 密码的唯一用途**：仅用于 Phase 8 的七牛云同步加密

## Decision

**将 AuthService 从 Phase 2 移除，延后至 Phase 8 实现：**

- Phase 2 完全聚焦于服务器数据的 SQLite 持久化和基础 CRUD
- Phase 8 实现云同步时，AuthService 作为同步的辅助功能加入
- `config` 表中预留 `password_hash` 配置项（但不强制使用）

### 当前处理方式

```go
// Phase 2: 无 AuthService，密码管理仅涉及
// 1. 首次运行自动生成 AES 静态密钥 → 存入 config 表
// 2. 服务器密码使用静态密钥 AES-GCM 加密 → 存入 password_encrypted 列
// 3. 用户无需输入任何密码即可操作

// Phase 8: 恢复 AuthService
// 1. account 命令设置同步密码
// 2. 密码用 RSA 加密存入 ~/.assh/v2/.account
// 3. 同步时用 account 密码加密传输数据
```

## Consequences

1. **优点**：
   - Phase 2 聚焦单一职责，降低复杂度
   - 用户可立即使用 assh 而无需设置密码
   - 将密码相关特性与同步场景绑定，职责清晰

2. **缺点**：
   - Phase 8 需要额外实现密码变更时的数据重加密逻辑
   - 老用户可能期待立即有密码保护功能

## Related

- Related ADRs: ADR-008 (SQLite), ADR-009 (静态密钥)
