# Changelog

## v2.2.0 (2026-06-14)

### X1: AI 安全检查

#### 领域层 (`asshc/domain/`)
- `vuln.go` — 漏洞检查实体 (VulnCheck/VulnReport/CVEDetail/ScriptValidation/SecurityRule)

#### 接口层 (`asshc/port/`)
- `vuln.go` — AIProvider/VulnerabilityChecker/CVEDetailFetcher/SecurityScanner/VulnStore 接口
- `security.go` — SecurityChecker/SecurityConfigLoader 接口

#### 基础设施层 (`asshc/infra/`)
- `ai/config.go` — AI 配置加载器 (YAML 配置, 默认值合并)
- `ai/openai.go` — OpenAI 兼容 API 客户端 (Generate/GenerateWithSystem/GenerateScript)
- `cve/nvd.go` — NVD NIST API CVE 查询 (Fetch/FetchBatch/ValidateCVEID)
- `security/scanner.go` — 安全审查引擎 (12 条内置规则, 脚本/输入检查)
- `vulnstore/store.go` — 漏洞检查存储 (脚本/报告/索引文件管理)

#### 服务编排层 (`asshc/service/`)
- `vuln.go` — VulnService (Check/CheckBatch/GenerateScript/ValidateScript/SaveScript/SaveReport)

#### CLI 命令 (`cmd/`)
- `security.go` — security 命令组
  - `vuln-check` — 漏洞检查 (CVE/文本输入, 单台/批量/分组)
  - `rules` — 安全规则管理 (list/add/remove)

#### 配置
- `~/.assh/v2/ai.yml` — AI 服务配置 (endpoint/model/api_key)
- `~/.assh/security/scripts/` — 脚本/报告存储目录

#### 依赖
- 新增 `gopkg.in/yaml.v3` — YAML 配置解析

### X2: 虚拟命令投放与执行 (基础框架)

#### 领域层 (`asshc/domain/`)
- `virtual.go` — 虚拟命令实体 (VirtualCommand/VirtualCommandConfig/VirtualCommandResult)

#### 接口层 (`asshc/port/`)
- `virtual.go` — VirtualCommandHandler/VirtualCommandService/VirtualCommandConfigLoader 接口

#### 基础设施层 (`asshc/infra/virtual/`)
- `config.go` — 虚拟命令配置加载器 (YAML 配置, 默认值合并)
- `handler.go` — 输入拦截处理器 (虚拟命令识别和执行)
- `push.go` — @push 命令处理器 (文件上传)
- `pull.go` — @pull 命令处理器 (文件下载)
- `help.go` — @help 命令处理器 (帮助系统)
- `version.go` — @version 命令处理器 (版本信息)
- `status.go` — @status 命令处理器 (连接状态)

#### 服务编排层 (`asshc/service/`)
- `virtual.go` — VirtualCommandService (Register/Execute/GetCommand/ListCommands)

#### 配置
- `~/.assh/v2/virtual.yml` — 虚拟命令配置 (内置命令/自定义命令/全局设置)

### 其他更新

#### 文档
- 新增 `docs/requirements/RQ-003-virtual-commands.md` — X2 需求文档
- 新增 `docs/requirements/RQ-004-ai-security-check.md` — X1 需求文档
- 新增 `docs/v2.2.0-preview.1-plan.md` — 版本规划文档
- 更新 `docs/TODO.md` — 任务状态更新
- 更新 `AGENTS.md` — 版本和模块信息更新

#### 版本管理
- 创建 `v2.2.0` 版本标签
- 创建 `dev/v2.2.0-phase-x1` 和 `dev/v2.2.0-phase-x2` 开发分支

---

## v2.0.1 (2026-05-26)

### Phase 11: 自动更新/升级

**完成时间**: 2026-05-26 | **ADR**: ADR-031 | **设计**: [Phase11-AutoUpdate-Design.md](docs/worklogs/2026-05-26-Phase11-AutoUpdate-Design.md)

#### 领域层 (`asshc/domain/`)
- `update.go` — UpdateInfo + UpdatePlatform 结构体
- `errors.go` — 新增 6 个错误 (ErrUpdateNotFound/ErrUpdateFailed/ErrRollbackFailed/ErrVerificationFailed/ErrNetworkError/ErrAlreadyLatest)

#### 接口层 (`asshc/port/`)
- `updater.go` — VersionChecker + Downloader + Installer 接口定义

#### 基础设施层 (`asshc/infra/updater/`)
- `github.go` — GitHubChecker 实现 VersionChecker
- `downloader.go` — HTTPDownloader 实现 Downloader
- `installer.go` — BinaryInstaller 实现 Installer

#### 服务编排层 (`asshc/service/`)
- `upgrade.go` — UpgradeService

#### CLI 命令 (`cmd/`)
- `upgrade.go` — upgrade 命令

---

## v2.0.0-dev (2026-05-13)

### Phase 7-10: 代理/隧道/密钥管理/跳板机

详见 [USAGE.md](USAGE.md) 完整功能说明。

---

## v2.0.0-dev (2026-05-08)

### Phase 0-6: 基础架构

- 项目骨架搭建
- 基础设施移植 (log/config/crypto)
- 数据持久化层 (SQLite + AES-GCM)
- SSH 连接层
- CLI 命令组装
- SFTP 文件传输
- 密钥管理
