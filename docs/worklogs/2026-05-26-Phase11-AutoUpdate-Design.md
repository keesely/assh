# Phase 11: 自动更新/升级 — 详细设计

**日期**: 2026-05-26  
**状态**: 设计中  
**分支**: `dev/v2.0.1-phase-11`

---

## 1. 目标

实现 ASSH 的自动更新/升级机制，保障用户始终运行最新安全版本。

### 1.1 核心功能

| 功能 | 说明 |
|------|------|
| 版本检查 | 从 GitHub Release 检查最新版本 |
| 手动升级 | `assh upgrade` 下载并安装新版本 |
| 强制更新 | `assh upgrade --force` 强制更新到指定版本 |
| 更新源配置 | 支持自定义更新源（URL/GitHub Release） |
| 回滚支持 | 升级失败时回滚到上一版本 |

---

## 2. 架构设计

### 2.1 分层结构

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

### 2.2 接口设计

```go
// port/updater.go

// VersionChecker 定义版本检查接口。
type VersionChecker interface {
    // CheckUpdate 检查是否有新版本可用。
    // 返回：
    //   - *UpdateInfo: 更新信息（如果有新版本）
    //   - error: 检查失败时返回错误
    CheckUpdate() (*UpdateInfo, error)
    
    // GetCurrentVersion 获取当前版本号。
    GetCurrentVersion() string
}

// Updater 定义更新操作接口。
type Updater interface {
    // Download 下载指定版本的更新包。
    // 参数：
    //   - ctx: 上下文，用于控制超时和取消
    //   - info: 更新信息
    //   - progress: 进度回调函数
    // 返回：
    //   - string: 下载文件的本地路径
    //   - error: 下载失败时返回错误
    Download(ctx context.Context, info *UpdateInfo, progress DownloadProgress) (string, error)
    
    // Verify 验证下载文件的完整性。
    // 参数：
    //   - filePath: 下载文件路径
    //   - expectedSHA256: 期望的 SHA256 哈希值
    // 返回：
    //   - error: 验证失败时返回错误
    Verify(filePath string, expectedSHA256 string) error
    
    // Install 安装更新。
    // 参数：
    //   - ctx: 上下文
    //   - filePath: 下载文件路径
    // 返回：
    //   - error: 安装失败时返回错误
    Install(ctx context.Context, filePath string) error
    
    // Rollback 回滚到上一版本。
    // 返回：
    //   - error: 回滚失败时返回错误
    Rollback() error
}

// UpdateInfo 表示更新信息。
type UpdateInfo struct {
    Version     string    `json:"version"`      // 新版本号
    ReleaseDate time.Time `json:"release_date"` // 发布时间
    ReleaseNotes string   `json:"release_notes"` // 发布说明
    DownloadURL string    `json:"download_url"`  // 下载链接
    SHA256      string    `json:"sha256"`        // 文件哈希值
    FileSize    int64     `json:"file_size"`     // 文件大小（字节）
    Mandatory   bool      `json:"mandatory"`     // 是否强制更新
}

// DownloadProgress 定义下载进度回调函数。
type DownloadProgress func(downloaded, total int64, percent float64)
```

---

## 3. 实现细节

### 3.1 GitHub Release 检查

```go
// infra/updater/github.go

type GitHubChecker struct {
    owner   string // 仓库所有者
    repo    string // 仓库名称
    current string // 当前版本
    client  *http.Client
}

// API 端点: https://api.github.com/repos/{owner}/{repo}/releases/latest
```

**版本比较逻辑**：
1. 解析语义化版本（v2.0.1 → 2.0.1）
2. 比较 Major.Minor.Patch
3. 忽略预发布标签（-alpha, -beta）

### 3.2 下载器

```go
// infra/updater/downloader.go

type HTTPDownloader struct {
    client *http.Client
    tempDir string // 临时下载目录
}
```

**下载流程**：
1. 创建临时目录 `~/.assh/v2/tmp/`
2. 下载文件到临时目录
3. 支持断点续传（HTTP Range）
4. 进度回调通知

### 3.3 安装器

```go
// infra/updater/installer.go

type BinaryInstaller struct {
    currentPath string // 当前二进制路径
    backupDir   string // 备份目录
}
```

**安装流程**：
1. 备份当前二进制到 `~/.assh/v2/backups/assh-{version}`
2. 验证下载文件的可执行权限
3. 原子替换：`os.Rename` 替换当前二进制
4. 设置可执行权限 `0755`

**回滚流程**：
1. 从 `~/.assh/v2/backups/` 查找最近备份
2. 恢复备份文件到原位置

---

## 4. CLI 命令设计

### 4.1 `assh upgrade` 命令

```bash
# 检查并升级到最新版本
assh upgrade

# 强制升级到指定版本
assh upgrade --version v2.0.2

# 强制升级（忽略版本检查）
assh upgrade --force

# 仅检查，不升级
assh upgrade --check

# 回滚到上一版本
assh upgrade --rollback

# 指定更新源
assh upgrade --source https://custom.example.com/releases
```

**参数规格**：

| 参数 | 短参 | 说明 |
|------|------|------|
| `--check` | `-c` | 仅检查更新，不执行升级 |
| `--version` | `-v` | 指定升级到的版本号 |
| `--force` | `-f` | 强制升级（跳过版本检查） |
| `--rollback` | `-r` | 回滚到上一版本 |
| `--source` | `-s` | 自定义更新源 URL |
| `--yes` | `-y` | 自动确认，不提示 |

### 4.2 `assh version --check` 命令

```bash
# 显示当前版本
assh version

# 检查是否有新版本
assh version --check
```

---

## 5. 数据存储

### 5.1 更新配置

存储位置：`~/.assh/v2/config` 表

```sql
-- 新增配置项
INSERT INTO config (key, value) VALUES ('update_source', 'github');
INSERT INTO config (key, value) VALUES ('update_auto_check', 'true');
INSERT INTO config (key, value) VALUES ('last_update_check', '2026-05-26T10:00:00Z');
```

### 5.2 备份目录结构

```
~/.assh/v2/
├── backups/
│   ├── assh-v2.0.0.backup
│   ├── assh-v2.0.1.backup
│   └── ...
└── tmp/
    └── assh-v2.0.2-download
```

---

## 6. 安全考虑

### 6.1 下载验证

- **HTTPS 强制**：所有下载必须使用 HTTPS
- **SHA256 校验**：下载后验证文件哈希
- **签名验证**（可选）：GPG 签名验证

### 6.2 权限管理

- **最小权限**：只替换当前二进制文件
- **备份保留**：保留最近 3 个版本的备份
- **临时文件清理**：安装成功后清理临时文件

---

## 7. 错误处理

### 7.1 错误类型

```go
var (
    ErrUpdateNotFound    = errors.New("update not found")
    ErrUpdateFailed      = errors.New("update failed")
    ErrRollbackFailed    = errors.New("rollback failed")
    ErrVerificationFailed = errors.New("verification failed")
    ErrNetworkError      = errors.New("network error")
)
```

### 7.2 失败恢复

1. **下载失败**：自动重试 3 次
2. **验证失败**：删除下载文件，提示用户
3. **安装失败**：自动回滚到备份版本
4. **回滚失败**：提示用户手动恢复

---

## 8. 测试策略

### 8.1 单元测试

| 测试文件 | 覆盖范围 |
|----------|----------|
| `service/upgrade_test.go` | 服务层逻辑（mock 接口） |
| `infra/updater/github_test.go` | GitHub API 解析 |
| `infra/updater/downloader_test.go` | 下载逻辑（mock HTTP） |
| `infra/updater/installer_test.go` | 安装/回滚逻辑 |

### 8.2 集成测试

- 使用本地 HTTP 服务器模拟 GitHub Release
- 测试完整升级流程
- 测试回滚机制

---

## 9. 实现计划

### 9.1 Phase 11 任务分解

| 编号 | 任务 | 文件 | 优先级 |
|------|------|------|--------|
| P11.1 | 更新领域实体 | `asshc/domain/update.go` | P0 |
| P11.2 | VersionChecker 接口 | `asshc/port/updater.go` | P0 |
| P11.3 | GitHub Release 检查实现 | `asshc/infra/updater/github.go` | P0 |
| P11.4 | 下载器实现 | `asshc/infra/updater/downloader.go` | P0 |
| P11.5 | 安装器实现 | `asshc/infra/updater/installer.go` | P0 |
| P11.6 | UpgradeService 服务层 | `asshc/service/upgrade.go` | P0 |
| P11.7 | upgrade CLI 命令 | `cmd/upgrade.go` | P0 |
| P11.8 | version --check 命令 | `cmd/version.go` (修改) | P0 |
| P11.9 | DI 注入 | `main.go` + `cmd/app.go` | P0 |
| P11.10 | 配置存储 | `asshc/infra/store/config.go` (扩展) | P1 |
| P11.11 | 自动检查机制 | `asshc/service/upgrade.go` | P1 |
| P11.12 | 单元测试 | `*_test.go` | P0 |

### 9.2 依赖关系

```
P11.1 (领域实体)
  ↓
P11.2 (接口定义)
  ↓
P11.3 (GitHub 实现) + P11.4 (下载器) + P11.5 (安装器)
  ↓
P11.6 (服务层)
  ↓
P11.7 (CLI 命令) + P11.8 (version 命令)
  ↓
P11.9 (DI 注入)
```

---

## 10. 验收标准

### 10.1 功能验收

| # | 场景 | 操作 | 预期 |
|---|------|------|------|
| F1 | 检查更新 | `assh upgrade --check` | 显示当前版本和最新版本 |
| F2 | 升级到最新 | `assh upgrade` | 下载并安装最新版本 |
| F3 | 强制升级 | `assh upgrade --force` | 跳过版本检查，强制升级 |
| F4 | 指定版本 | `assh upgrade --version v2.0.2` | 升级到指定版本 |
| F5 | 回滚 | `assh upgrade --rollback` | 回滚到上一版本 |
| F6 | 自定义源 | `assh upgrade --source URL` | 从自定义源下载 |
| F7 | 自动确认 | `assh upgrade -y` | 不提示，直接升级 |
| F8 | 版本检查 | `assh version --check` | 显示版本信息和更新状态 |

### 10.2 验证命令

```bash
cd asshv2 && go build ./...     # 编译
cd asshv2 && go vet ./...       # 静态检查
cd asshv2 && go test ./... -v   # 测试
```

---

## 11. 后续扩展

### 11.1 自动检查

- 启动时自动检查更新（可配置）
- 定时检查（每天/每周）
- 静默下载，提示安装

### 11.2 增量更新

- 仅下载变更部分
- 二进制差分更新

### 11.3 多源支持

- 镜像源切换
- CDN 加速

---

**设计完成时间**: 2026-05-26  
**设计者**: PM Agent
