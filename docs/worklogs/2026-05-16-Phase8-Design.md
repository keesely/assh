# Phase 8: 云同步 (Qiniu) 设计文档

## 需求确认

基于用户确认的需求：

| 项目 | 选择 | 说明 |
|------|------|------|
| **数据范围** | 仅服务器配置 | 只同步servers表中的服务器配置数据 |
| **账户管理** | 单账户 | 只支持一个七牛云账户，简单易用 |
| **同步策略** | 增量同步为主 | 默认增量同步，首次或冲突时全量同步 |
| **冲突处理** | 保留双方 | 冲突时保留两个版本，添加后缀区分 |

## 架构设计

### 1. Syncer 接口设计 (`asshc/port/syncer.go`)

```go
package port

import (
    "context"
    "time"
)

// Syncer 定义了云同步的抽象接口。
type Syncer interface {
    // Login 验证云账户凭据。
    Login(ctx context.Context, account SyncAccount) error
    
    // Push 将本地数据推送到云端。
    Push(ctx context.Context, data *SyncData) (*SyncResult, error)
    
    // Pull 从云端拉取数据到本地。
    Pull(ctx context.Context) (*SyncData, error)
    
    // List 列出云端所有同步对象。
    List(ctx context.Context) ([]*SyncObject, error)
    
    // Delete 删除云端的同步对象。
    Delete(ctx context.Context, key string) error
}

// SyncAccount 表示云账户信息。
type SyncAccount struct {
    Type       string `json:"type"`        // 云服务类型：qiniu
    AccessKey  string `json:"access_key"`  // 访问密钥
    SecretKey  string `json:"secret_key"`  // 秘密密钥（加密存储）
    Bucket     string `json:"bucket"`      // 存储空间
    Zone       string `json:"zone"`        // 区域：huadong/huabei/huanan/beimei/xinjiapo
}

// SyncData 表示同步的数据内容。
type SyncData struct {
    Version   string                 `json:"version"`   // 数据格式版本
    Timestamp time.Time              `json:"timestamp"` // 同步时间戳
    Servers   map[string]*ServerData `json:"servers"`   // 服务器配置数据
    Checksum  string                 `json:"checksum"`  // 数据校验和
}

// ServerData 表示单个服务器的同步数据。
type ServerData struct {
    Name      string                 `json:"name"`
    Group     string                 `json:"group"`
    Host      string                 `json:"host"`
    Port      int                    `json:"port"`
    User      string                 `json:"user"`
    Password  string                 `json:"password,omitempty"` // 加密后的密码
    KeyFile   string                 `json:"key_file,omitempty"`
    Remark    string                 `json:"remark,omitempty"`
    Options   map[string]interface{} `json:"options,omitempty"`
    Version   int                    `json:"version"`
    UpdatedAt time.Time              `json:"updated_at"`
}

// SyncResult 表示同步操作的结果。
type SyncResult struct {
    Success   bool      `json:"success"`
    Message   string    `json:"message"`
    Pushed    int       `json:"pushed"`    // 推送的服务器数量
    Updated   int       `json:"updated"`   // 更新的服务器数量
    Conflicts int       `json:"conflicts"` // 冲突数量
    Timestamp time.Time `json:"timestamp"` // 同步完成时间
}

// SyncObject 表示云端的同步对象。
type SyncObject struct {
    Key       string    `json:"key"`
    Size      int64     `json:"size"`
    ETag      string    `json:"etag"`
    Modified  time.Time `json:"modified"`
    Version   string    `json:"version"`
}
```

### 2. 数据结构设计

#### 2.1 云账户表 (`cloud_accounts`)

```sql
CREATE TABLE IF NOT EXISTS cloud_accounts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL DEFAULT 'default',  -- 账户名称
    type        TEXT NOT NULL DEFAULT 'qiniu',    -- 云服务类型
    access_key  TEXT NOT NULL,                    -- 访问密钥
    secret_key  BLOB NOT NULL,                    -- 加密的秘密密钥
    bucket      TEXT NOT NULL,                    -- 存储空间
    zone        TEXT NOT NULL DEFAULT 'huadong',  -- 区域
    enabled     INTEGER NOT NULL DEFAULT 1,       -- 是否启用
    created_at  TEXT DEFAULT (datetime('now')),
    updated_at  TEXT DEFAULT (datetime('now')),
    UNIQUE(name)
);
```

#### 2.2 同步历史表 (`sync_history`)

```sql
CREATE TABLE IF NOT EXISTS sync_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    direction   TEXT NOT NULL,        -- push/pull
    status      TEXT NOT NULL,        -- success/failed/partial
    message     TEXT,                 -- 结果消息
    pushed      INTEGER DEFAULT 0,    -- 推送数量
    updated     INTEGER DEFAULT 0,    -- 更新数量
    conflicts   INTEGER DEFAULT 0,    -- 冲突数量
    timestamp   TEXT DEFAULT (datetime('now'))
);
```

### 3. 冲突处理策略

根据用户选择"保留双方"策略，实现如下：

```go
// handleConflict 处理本地和云端的数据冲突
func (s *SyncService) handleConflict(local, remote *ServerData) (*ServerData, *ServerData, error) {
    // 比较时间戳
    if local.UpdatedAt.After(remote.UpdatedAt) {
        // 本地更新，保留本地版本，云端版本添加后缀
        remoteCopy := *remote
        remoteCopy.Name = remote.Name + "_cloud_" + remote.UpdatedAt.Format("20060102-150405")
        return local, &remoteCopy, nil
    } else if remote.UpdatedAt.After(local.UpdatedAt) {
        // 云端更新，保留云端版本，本地版本添加后缀
        localCopy := *local
        localCopy.Name = local.Name + "_local_" + local.UpdatedAt.Format("20060102-150405")
        return &localCopy, remote, nil
    }
    
    // 时间戳相同，保留双方，添加不同后缀
    localCopy := *local
    localCopy.Name = local.Name + "_local"
    remoteCopy := *remote
    remoteCopy.Name = remote.Name + "_cloud"
    return &localCopy, &remoteCopy, nil
}
```

### 4. 增量同步策略

#### 4.1 首次同步
1. 检测云端是否有数据
2. 如果没有，执行全量上传
3. 如果有，比较版本号，执行增量同步

#### 4.2 后续同步
1. 获取本地最后同步时间戳
2. 获取云端最后同步时间戳
3. 比较双方时间戳，确定同步方向
4. 仅同步变更的数据

#### 4.3 同步版本管理
- 每次同步更新`sync_version`字段
- 使用时间戳+序列号作为版本标识
- 支持版本回滚（通过sync_history）

### 5. 七牛云实现重构

#### 5.1 现有代码分析 (`lib/qiniu/`)

现有功能：
- ✅ 基本上传/下载
- ✅ 存储空间管理
- ✅ 区域配置

需要重构：
- 🔄 适配新的Syncer接口
- 🔄 增强错误处理和重试机制
- 🔄 支持增量同步
- 🔄 支持冲突检测

#### 5.2 新实现结构

```
asshc/infra/sync/qiniu/
├── qiniu.go      # 七牛云Syncer实现
├── bucket.go     # 存储空间管理
├── uploader.go   # 上传逻辑
├── downloader.go # 下载逻辑
└── utils.go      # 工具函数
```

### 6. SyncService 设计

```go
package service

type SyncService struct {
    syncer port.Syncer
    repo   port.ServerRepository
    store  *store.Store
}

func NewSyncService(syncer port.Syncer, repo port.ServerRepository, store *store.Store) *SyncService

// Push 将本地服务器配置推送到云端
func (s *SyncService) Push(ctx context.Context) (*SyncResult, error)

// Pull 从云端拉取服务器配置到本地
func (s *SyncService) Pull(ctx context.Context) (*SyncResult, error)

// GetAccount 获取云账户信息
func (s *SyncService) GetAccount() (*port.SyncAccount, error)

// SetAccount 设置云账户信息
func (s *SyncService) SetAccount(account *port.SyncAccount) error

// GetHistory 获取同步历史
func (s *SyncService) GetHistory(limit int) ([]*SyncHistory, error)
```

### 7. CLI命令设计

```bash
# 账户管理
assh sync account add --type qiniu --ak <access_key> --sk <secret_key> --bucket <bucket> --zone <zone>
assh sync account list
assh sync account test  # 测试账户连接

# 数据同步
assh sync push          # 推送数据到云端
assh sync pull          # 从云端拉取数据
assh sync status        # 查看同步状态

# 高级选项
assh sync push --force    # 强制全量同步
assh sync pull --force    # 强制覆盖本地
assh sync history         # 查看同步历史
```

## 实现计划

### Phase 1: 基础架构 (1-2天)

1. **创建数据库迁移**
   - 添加`cloud_accounts`表
   - 添加`sync_history`表

2. **实现Syncer接口**
   - 定义核心数据结构
   - 实现基础接口框架

3. **实现SyncService**
   - 编排同步逻辑
   - 实现冲突处理

### Phase 2: 七牛云实现 (2-3天)

1. **重构现有代码**
   - 适配Syncer接口
   - 增强错误处理

2. **实现增量同步**
   - 版本管理
   - 变更检测

3. **实现冲突处理**
   - "保留双方"策略
   - 冲突记录

### Phase 3: CLI命令 (1-2天)

1. **实现sync命令**
   - 账户管理子命令
   - 同步子命令

2. **集成测试**
   - 端到端测试
   - 错误场景测试

### Phase 4: 验证和优化 (1天)

1. **单元测试**
2. **集成测试**
3. **性能优化**
4. **文档更新**

## 技术决策

### ADR-030: 云同步数据范围选择

**决策**：仅同步服务器配置数据

**理由**：
1. 服务器配置是最核心的业务数据
2. 减少同步数据量，提高同步效率
3. 避免敏感配置信息泄露风险
4. 用户明确选择"仅服务器配置"

**影响**：
- 不同步config表中的应用配置
- 不同步changelog历史记录
- 不同步known_servers表

### ADR-031: 冲突处理策略

**决策**：保留双方，添加后缀区分

**理由**：
1. 用户明确选择"保留双方"策略
2. 避免数据丢失风险
3. 用户可以手动选择保留哪个版本
4. 提供最大的数据安全性

**实现**：
- 冲突时重命名其中一个版本
- 添加时间戳或"_local"/"_cloud"后缀
- 记录冲突日志供用户查看

### ADR-032: 单账户设计

**决策**：只支持一个云账户

**理由**：
1. 用户明确选择"单账户"设计
2. 简化实现复杂度
3. 降低用户理解成本
4. 大多数用户只需要一个云账户

**影响**：
- 数据库设计简化
- CLI命令简化
- 未来如需多账户支持，需重新设计

## 风险评估

### 风险1: 七牛云API变更

**影响**：高
**概率**：低
**缓解**：
- 使用稳定版本的SDK
- 封装适配层，隔离API变更
- 监控SDK更新

### 风险2: 数据一致性

**影响**：高
**概率**：中
**缓解**：
- 实现校验和验证
- 支持全量同步作为恢复手段
- 详细的错误日志

### 风险3: 网络不稳定

**影响**：中
**概率**：中
**缓解**：
- 实现重试机制
- 支持断点续传
- 详细的进度反馈

## 验证计划

### 单元测试

1. **Syncer接口测试** - mock实现
2. **SyncService测试** - 业务逻辑
3. **冲突处理测试** - 各种冲突场景
4. **数据库测试** - 账户CRUD操作

### 集成测试

1. **端到端同步测试** - push→pull验证
2. **冲突场景测试** - 模拟各种冲突
3. **错误恢复测试** - 网络中断恢复
4. **性能测试** - 大量服务器同步

### 验收标准

1. **功能完整性** - 所有sync命令正常工作
2. **数据一致性** - 同步前后数据完全一致
3. **错误处理** - 网络错误、API错误有合理处理
4. **性能** - 1000个服务器同步时间<30秒
5. **用户体验** - 清晰的进度反馈和错误提示

## 时间估算

| 阶段 | 任务 | 预计时间 |
|------|------|----------|
| Phase 1 | 基础架构 | 1-2天 |
| Phase 2 | 七牛云实现 | 2-3天 |
| Phase 3 | CLI命令 | 1-2天 |
| Phase 4 | 验证优化 | 1天 |
| **总计** | | **5-8天** |

---

**下一步行动**：
1. 创建数据库迁移脚本
2. 实现Syncer接口定义
3. 实现SyncService基础框架
4. 重构七牛云实现