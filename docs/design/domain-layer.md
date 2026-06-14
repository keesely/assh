# 领域层设计 (Domain Layer)

## 1. 概述

领域层是整个应用的核心，包含业务实体、值对象和领域错误定义。该层不依赖任何外部框架或基础设施，是最纯净的业务逻辑层。

### 1.1 设计原则

- **无外部依赖**：不依赖任何第三方库或基础设施代码
- **自包含**：所有业务规则和约束在领域层内定义
- **稳定**：变更频率最低，是其他层的基础

## 2. 实体 (Entities)

### 2.1 Server 实体

**位置**: `asshc/domain/server.go`

Server 是核心聚合根，表示一台 SSH 服务器的完整配置。

```go
type Server struct {
    Name    string                 `json:"name"`             // 服务器名称（组内唯一）
    Group   string                 `json:"group"`            // 服务器分组名
    Host    string                 `json:"host"`             // 主机地址（IP 或域名）
    Port    int                    `json:"port"`             // SSH 端口号（默认 22）
    User    string                 `json:"user"`             // 登录用户名（默认 root）
    Auth    *Auth                  `json:"auth,omitempty"`   // 认证信息（可选）
    Remark  string                 `json:"remark,omitempty"` // 备注信息
    Options map[string]interface{} `json:"options,omitempty"` // 扩展选项
    Version int                    `json:"version"`          // 配置版本号
}
```

**身份标识**: `Group + Name` 构成唯一标识，格式为 `"group.name"`

**业务规则**:
- `Name` 和 `Group` 通过 `ParseName()` 从复合名称解析
- `Port` 默认值为 22，范围 1-65535
- `User` 默认值为 "root"
- `Version` 由持久化层自动维护，用于乐观锁

**关键方法**:
- `NewServer()` - 创建实例并设置默认值
- `ParseName()` - 解析 "group.name" 格式
- `JoinName()` - 合并分组和名称
- `ValidateServer()` - 校验必填字段

### 2.2 KnownServer 实体

**位置**: `asshc/domain/known_server.go`

记录未具名服务器的直连历史。

```go
type KnownServer struct {
    ID              string    `json:"id"`               // 唯一标识（基于 host+port+user+auth 指纹）
    Host            string    `json:"host"`             // 主机地址
    Port            int       `json:"port"`             // 端口号
    User            string    `json:"user"`             // 用户名
    AuthFingerprint string    `json:"auth_fingerprint"` // 认证方式指纹
    KeyBackupPath   string    `json:"key_backup_path"`  // 密钥备份路径
    ConnectCount    int       `json:"connect_count"`    // 连接次数
    LastConnectedAt time.Time `json:"last_connected_at"`// 最后连接时间
}
```

**业务规则**:
- ID 基于 `host:port:user:auth_fingerprint` 计算
- 同一 ID 的连接会递增 `connect_count`

## 3. 值对象 (Value Objects)

### 3.1 Auth 值对象

**位置**: `asshc/domain/server.go`

表示服务器的认证信息。

```go
type Auth struct {
    Password string `json:"password,omitempty"` // 密码认证
    KeyFile  string `json:"keyfile,omitempty"`  // 密钥文件路径
}
```

**特点**:
- 不可变（创建后不应修改）
- 可为空（`*Auth` 指针类型）
- 两种认证方式可同时存在

### 3.2 ChangelogEntry 值对象

**位置**: `asshc/domain/changelog.go`

记录服务器配置的变更历史。

```go
type ChangelogEntry struct {
    Version   int       `json:"version"`    // 版本号
    ChangeType string   `json:"change_type"` // 变更类型
    Timestamp time.Time `json:"timestamp"`   // 变更时间
    Snapshot  *Server   `json:"snapshot"`    // 变更前快照
}
```

**变更类型常量**:
- `ChangeTypeAdd` - 新增
- `ChangeTypeUpdate` - 更新
- `ChangeTypeDelete` - 删除
- `ChangeTypeMove` - 移动/重命名

### 3.3 UpdateInfo 值对象

**位置**: `asshc/domain/update.go`

表示系统更新信息。

```go
type UpdateInfo struct {
    Version     string            `json:"version"`     // 最新版本号
    ReleaseDate time.Time         `json:"release_date"`// 发布日期
    ReleaseNotes string           `json:"release_notes"`// 发布说明
    Platforms   map[string]UpdatePlatform `json:"platforms"` // 各平台下载信息
}

type UpdatePlatform struct {
    URL      string `json:"url"`      // 下载地址
    Checksum string `json:"checksum"` // 校验和
    Size     int64  `json:"size"`     // 文件大小
}
```

## 4. 领域错误 (Domain Errors)

**位置**: `asshc/domain/errors.go`

定义领域层的通用错误语义：

```go
var (
    // 服务器相关
    ErrNotFound      = errors.New("server not found")
    ErrExists        = errors.New("server already exists")
    ErrInvalidName   = errors.New("invalid server name")
    ErrInvalidPort   = errors.New("invalid port number")
    ErrEmptyField    = errors.New("empty field not allowed")
    
    // 版本相关
    ErrVersionNotFound = errors.New("version not found in changelog")
    ErrInvalidVersion  = errors.New("invalid version number")
    
    // KnownServer 相关
    ErrKnownServerNotFound = errors.New("known server not found")
    
    // 云同步相关
    ErrAccountNotFound = errors.New("cloud account not found")
    ErrSyncFailed      = errors.New("sync operation failed")
    ErrInvalidAccount  = errors.New("invalid cloud account credentials")
)
```

**错误使用原则**:
- 领域错误应在领域层定义
- 服务层直接返回领域错误
- 命令层负责错误展示和国际化

## 5. 聚合设计

### 5.1 Server 聚合

```
┌─────────────────────────────────────────────────────────────┐
│                    Server 聚合                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              Server (聚合根)                          │  │
│  │  • Name + Group = 唯一标识                            │  │
│  │  • 管理 Auth 生命周期                                 │  │
│  │  • 维护 Version 乐观锁                                │  │
│  └───────────────────────────────────────────────────────┘  │
│         │                                                    │
│         ├── Auth (值对象，可空)                              │
│         │   • Password                                      │
│         │   • KeyFile                                       │
│         │                                                   │
│         └── ChangelogEntry[] (领域事件记录)                 │
│             • Version                                       │
│             • ChangeType                                    │
│             • Snapshot                                      │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 聚合边界

- **Server** 是唯一的聚合根
- **Auth** 作为值对象嵌入 Server
- **ChangelogEntry** 通过版本号关联，不直接嵌入
- **KnownServer** 是独立实体，不属于 Server 聚合

## 6. 领域服务

当前设计中，领域逻辑主要在应用服务层实现，领域层保持轻量。未来可考虑将以下逻辑下沉到领域服务：

- 服务器名称解析和验证
- 版本号管理和冲突检测
- 认证回退策略

## 7. 最佳实践

### 7.1 添加新实体

1. 在 `domain/` 包中定义结构体
2. 添加 JSON tag 用于序列化
3. 实现必要的校验方法
4. 定义相关的领域错误

### 7.2 添加新值对象

1. 确保值对象不可变
2. 使用指针类型允许为空
3. 实现 `String()` 方法便于调试

### 7.3 错误处理

1. 领域错误使用 `errors.New()` 定义
2. 错误信息简洁明了
3. 避免暴露内部实现细节

## 8. 相关文档

- [DDD 架构概览](./overview.md)
- [端口层设计](./port-layer.md)
- [服务层设计](./service-layer.md)
