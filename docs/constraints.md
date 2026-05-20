# Constraints

- **Go 版本**: 1.25.0+
- **平台**: 不支持 Windows 编译
- **工作目录**: 所有开发在 `asshv2/` 内进行（git worktree）

## 依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| `modernc.org/sqlite` | latest | SQLite 数据库驱动（纯 Go，无 CGO） |
| `github.com/rs/zerolog` | v1.32.0 | 日志库 |
| `github.com/pkg/sftp` | v1.13.7+ | SFTP 客户端 |
| `github.com/qiniu/api.v7/v7` | v7.4.0 | 七牛云 SDK |
| `github.com/urfave/cli` | v1.22.2 | CLI 框架 |
| `golang.org/x/crypto` | v0.45.0 | SSH 库 |
| `golang.org/x/term` | (indirect) | 终端原始模式设置（通过 x/crypto/ssh 间接依赖） |

> **kiris 库**（`github.com/keesely/kiris`）：Phase 1 移植后已无直接依赖，待后续确认是否从 go.mod 移除。

## 测试文件位置

- `asshv2/log/log_test.go` — 日志模块测试
- `asshv2/asshc/.../*_test.go` — 核心业务测试（逐步添加）
