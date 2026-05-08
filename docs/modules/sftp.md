# SFTP Module

**文件**: `asshv2/asshc/infra/sftp/handler.go`

## 功能

基于 `github.com/pkg/sftp` 实现：

- **推送**：`push` 命令，支持文件和目录
- **拉取**：`pull` 命令
- **断点续传**：大文件分块上传，显示进度和速率