# Proxy Module（已迁移）

> ⚠️ 本文档已废弃，请移步 [proxy-tunnel.md](proxy-tunnel.md) 查看完整的 Phase 7 代理 & 隧道设计文档。

## 迁移说明

- 旧文件 `infra/tunnel/tunnel.go` → 合并至 `infra/proxy/`
- 旧 `hostproxy` 命令 → `tunnel` 命令组
- 旧 `localproxy` 命令 → `tunnel` 命令组

详细信息见：
- [代理 & 隧道设计文档](proxy-tunnel.md)
- [ADR-028: 代理 & 隧道架构设计](../adrs/028-proxy-tunnel-architecture.md)
