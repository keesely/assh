# SSH Connection Module

**文件**: `asshv2/asshc/infra/ssh/client.go` + `asshv2/asshc/infra/ssh/session.go`

## 功能

基于 `golang.org/x/crypto/ssh` 实现：

- **认证方式**：密码认证 + 公钥认证
- **交互式终端**：支持 TTY 和终端窗口大小调整
- **心跳保活**：支持 `ServerAliveInterval` 配置
- **远程命令执行**：通过 `-c` 参数