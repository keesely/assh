# ADR-031: 自动更新/升级架构设计

## Status

accepted

## Date

2026-05-26

## Context

ASSH 需要实现自动更新/升级机制，保障用户始终运行最新安全版本。这是 Phase 11 的核心功能，从原 Phase 12 提升至 Phase 11，因为自动更新是保障用户安全的关键基础设施。

### 问题背景

1. **安全需求**：安全漏洞修复后，用户需要及时更新
2. **版本碎片化**：用户可能运行旧版本，无法享受新功能和修复
3. **手动更新繁琐**：用户需要手动下载、替换二进制文件

### 可选方案

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A. GitHub Release** | 简单、免费、社区支持 | 依赖 GitHub、国内访问可能慢 |
| B. 自建更新服务器 | 完全控制、可定制 | 需要维护成本、服务器费用 |
| C. 包管理器集成 | 系统级集成、自动更新 | 仅支持特定平台、更新延迟 |

## Decision

选择 **方案 A: GitHub Release** 作为默认更新源，同时支持自定义更新源。

### 架构设计

```
cmd/upgrade.go          # CLI 命令层
  ↓
service/upgrade.go      # 服务编排层
  ↓
port/updater.go         # 接口定义
  ↓
infra/updater/          # 基础设施实现
  ├── github.go         # GitHub Release 检查
  ├── downloader.go     # 下载器
  └── installer.go      # 安装器（替换二进制）
```

### 接口定义

```go
// VersionChecker 版本检查接口
type VersionChecker interface {
    CheckUpdate() (*UpdateInfo, error)
    GetCurrentVersion() string
}

// Updater 更新操作接口
type Updater interface {
    Download(ctx context.Context, info *UpdateInfo, progress DownloadProgress) (string, error)
    Verify(filePath string, expectedSHA256 string) error
    Install(ctx context.Context, filePath string) error
    Rollback() error
}
```

### 关键设计决策

1. **原子更新**：下载到临时目录，验证后替换
2. **备份机制**：保留最近 3 个版本的备份
3. **回滚支持**：升级失败时自动回滚
4. **SHA256 验证**：下载后验证文件完整性
5. **进度显示**：下载进度条（百分比、速率、ETA）

## Consequences

### 正面影响

1. **安全性提升**：用户可以快速获取安全更新
2. **用户体验**：一键升级，无需手动操作
3. **版本一致性**：减少版本碎片化
4. **可扩展性**：支持自定义更新源

### 负面影响

1. **依赖 GitHub**：国内访问可能受限
2. **二进制替换权限**：需要写入权限
3. **回滚复杂性**：需要管理备份文件

### 影响范围

- **新增模块**：`asshc/infra/updater/`
- **修改模块**：`cmd/`, `service/`, `port/`, `config/`
- **新增配置**：update_source, update_auto_check, last_update_check

## Related

- **Phase 11**: 自动更新/升级
- **设计文档**: `docs/worklogs/2026-05-26-Phase11-AutoUpdate-Design.md`
- **任务清单**: `docs/TODO.md` Phase 11 部分
