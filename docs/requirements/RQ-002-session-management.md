# RQ-002: Session 管理 — 守护进程生命周期管理

## Status
proposed

## Date
2026-06-04

## Requester
用户反馈 (2026-06-04)

## Background

当前 `--daemon/-d` 模式存在严重问题（见 BUG-014），daemon 子进程无法正常持续运行。用户需要：

1. `--daemon` 模式下服务应自动持久化到数据库
2. 提供统一的 session 管理机制，支持 `session list` 和 `session kill [SID]`

## Requirements

### RQ-002.1: Daemon 模式自动持久化

**优先级**: P0

当使用 `--daemon` 启动 proxy/tunnel 服务时，应自动将服务配置持久化到 SQLite：

```bash
# 自动持久化到数据库
assh tunnel myserver -R 8080:web:80 --daemon
# 等价于
assh tunnel myserver -R 8080:web:80 --save --daemon
```

**行为**:
- daemon 模式默认启用 `--save`
- 服务记录包含：服务器名、端口映射、daemon 标志、PID、启动时间
- 服务状态标记为 `running`

### RQ-002.2: Session 列表

**优先级**: P0

提供 `session list` 命令查看所有活跃的守护进程：

```bash
assh session list
```

**输出格式**:
```
SID   TYPE    SERVER    FORWARDS                  PID    STATUS    UPTIME
1     tunnel  ali.24    -R 12496::4096           12345  running   2h15m
2     proxy   web-srv   SOCKS5 :1080             12346  running   1h30m
3     tunnel  db-srv    -L 3306:localhost:3306   12347  stopped   -
```

**过滤选项**:
```bash
assh session list --type tunnel      # 仅隧道
assh session list --type proxy       # 仅代理
assh session list --status running   # 仅运行中
assh session list --server ali.24    # 按服务器过滤
```

### RQ-002.3: Session 终止

**优先级**: P0

提供 `session kill` 命令终止指定的守护进程：

```bash
assh session kill <SID>              # 终止指定 session
assh session kill --all              # 终止所有 session
assh session kill --type tunnel      # 终止所有隧道 session
assh session kill --server ali.24    # 终止指定服务器的所有 session
```

**行为**:
- 发送 SIGTERM 给目标进程
- 等待进程退出（超时 5s 后 SIGKILL）
- 更新数据库状态为 `stopped`
- 清理 PID 文件

### RQ-002.4: Session 重启

**优先级**: P1

```bash
assh session restart <SID>           # 重启指定 session
```

**行为**:
- 终止当前进程
- 使用保存的配置重新启动
- 更新 PID 和启动时间

### RQ-002.5: Session 日志查看

**优先级**: P2

```bash
assh session logs <SID>              # 查看 session 日志
assh session logs <SID> -f           # 实时跟踪日志
assh session logs <SID> --tail 100   # 查看最后 100 行
```

## Technical Design

### 领域模型

复用已有的 `TunnelService` 和 `ProxyService` 实体，新增 `Session` 抽象：

```go
// domain/session.go
type Session struct {
    ID          string        // UUID
    Type        ServiceType   // "proxy" | "tunnel"
    ServiceID   int64         // 关联的 service 记录 ID
    ServerName  string
    Forwards    string        // 端口映射描述
    PID         int           // 守护进程 PID
    Status      SessionStatus // "running" | "stopped" | "failed"
    StartedAt   time.Time
    StoppedAt   *time.Time
    LogFile     string        // 日志文件路径
}
```

### 进程管理

```go
// port/session_manager.go
type SessionManager interface {
    // Start 启动守护进程并记录 session
    Start(serviceType ServiceType, config interface{}) (*Session, error)
    // Stop 终止指定 session
    Stop(sid string) error
    // Restart 重启 session
    Restart(sid string) error
    // List 列出 session
    List(filter SessionFilter) ([]*Session, error)
    // Get 获取单个 session
    Get(sid string) (*Session, error)
    // KillAll 终止所有匹配的 session
    KillAll(filter SessionFilter) error
}
```

### 进程生命周期

1. **启动**: fork 子进程 → 写 PID 文件 → 写数据库 → 阻塞等待信号
2. **运行**: 信号处理 (SIGTERM → 优雅退出, SIGHUP → 重载配置)
3. **停止**: 发送 SIGTERM → 等待退出 → 更新数据库 → 清理 PID 文件
4. **恢复**: 启动时扫描数据库 → 检查 PID 存活 → 恢复或标记 stopped

### 日志管理

```
~/.assh/v2/
├── sessions/
│   ├── {sid}.pid        # PID 文件
│   ├── {sid}.log        # 日志文件
│   └── {sid}.sock       # Unix socket (可选，用于 IPC)
└── asshv2.db            # SQLite 数据库
```

## Command Summary

```bash
# Session 管理
assh session list                           # 列出所有 session
assh session list --type tunnel --status running  # 过滤
assh session kill <SID>                     # 终止指定 session
assh session kill --all                     # 终止所有
assh session restart <SID>                  # 重启
assh session logs <SID> [-f] [--tail N]     # 查看日志

# Daemon 启动（自动注册 session）
assh tunnel myserver -R 8080:web:80 --daemon
assh proxy myserver --socks :1080 --daemon
```

## Dependencies

- BUG-014 修复（daemon 模式基础功能）
- NEXT-001.6 ~ NEXT-001.8（服务领域实体 + 持久化）

## Related

- [BUG-014: tunnel daemon 模式无法正常工作](../bugs/BUG-014-tunnel-daemon-mode-broken.md)
- [NEXT-001 设计文档](../worklogs/2026-05-29-NEXT-001-Design.md)
- [TODO.md](../TODO.md) — NEXT-001 统一服务管理
