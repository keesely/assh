# Proxy Module

**文件**: `asshv2/asshc/infra/proxy/socks5.go` + `asshv2/asshc/infra/proxy/port.go` + `asshv2/asshc/infra/tunnel/tunnel.go`

## 功能

支持多种代理模式：

- **SOCKS5 代理**：`proxy` 命令
- **端口转发**：`hostproxy` 命令（远程端口 → 本地端口）
- **SSH 隧道**：`localproxy` 命令