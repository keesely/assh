# NEXT-001 阶段 1-2 验收报告

**日期**: 2026-05-29
**分支**: dev/v2.1.0-phase-12
**版本**: v2.1.0-dev

## 验收范围

NEXT-001 阶段 1-2：Proxy & Tunnel 参数重构 + 远程监听模式实现

## 验收结果

### ✅ 阶段 1：参数重构 + tunnel 简化

| 任务 | 状态 | 验证结果 |
|------|:----:|----------|
| P0.1: 修复 --http 参数无效 | ✅ | `./assh proxy ali.24 --http :8009 --ports 8009:4096` 正确进入远程监听模式 |
| P0.2: 重构 proxy 参数体系 | ✅ | 新增 `--socks`/`--tcp`/`--listen`，保留 `--socks5` 别名 |
| P0.3: tunnel 命令简化 | ✅ | 去除 `start` 子命令，直接 `assh tunnel server -L/-R` |
| P0.4: 端口参数自动补全 | ✅ | 空 host 自动补全为 `127.0.0.1` |

### ✅ 阶段 2：--listen http 实现

| 任务 | 状态 | 验证结果 |
|------|:----:|----------|
| 远程监听模式实现 | ✅ | `startRemoteListenMode()` 函数正确实现 |
| 端口映射解析 | ✅ | `--ports 8009:4096` 正确解析为 `[8009] -> [4096]` |
| SSH Remote Forward | ✅ | `0.0.0.0:8009 <- (SSH) <- 127.0.0.1:4096` |

## 验证命令

```bash
# 编译验证
cd asshv2 && go build -o assh . && echo "BUILD OK"

# 功能验证（带超时）
timeout 3 ./assh proxy ali.24 --http :8009 --ports 8009:4096 2>&1
```

## 验证输出

```
{"level":"debug","time":"2026-05-29T23:44:14+08:00","message":"startProxyWithClient: Reverse=false, TCPAddr=\"\", HTTPAddr=\":8009\""}
{"level":"debug","time":"2026-05-29T23:44:14+08:00","message":"entering remote listen mode"}
{"level":"debug","time":"2026-05-29T23:44:14+08:00","message":"startRemoteListenMode: TCPAddr=\"\", HTTPAddr=\":8009\", ListenType=\"tcp\", Ports=\"8009:4096\""}
{"level":"debug","time":"2026-05-29T23:44:14+08:00","message":"parsed port rules: [{ServerPorts:[8009] LocalPorts:[4096]}]"}
{"level":"info","time":"2026-05-29T23:44:14+08:00","message":"Starting remote http listen on 0.0.0.0:8009 -> local 127.0.0.1:4096"}
{"level":"debug","time":"2026-05-29T23:44:14+08:00","message":"tunnel forward [d97558221c9325cf] added (type=remote)"}
{"level":"debug","time":"2026-05-29T23:44:14+08:00","message":"remote forward [d97558221c9325cf] started: 0.0.0.0:8009 <- (SSH) <- 127.0.0.1:4096"}
{"level":"debug","time":"2026-05-29T23:44:14+08:00","message":"tunnel forward [d97558221c9325cf] started"}
{"level":"info","time":"2026-05-29T23:44:16+08:00","message":"received signal terminated, shutting down..."}
{"level":"debug","time":"2026-05-29T23:44:16+08:00","message":"remote forward [d97558221c9325cf] stopped"}
```

## 问题修复记录

### 问题 1：urfave/cli Subcommands 导致 flag 解析失败

**现象**: `--http` 参数无法被正确解析，所有参数被当作位置参数

**根因**: urfave/cli v1 的 bug，当 Command 有 Subcommands 时，flag 解析异常

**解决方案**: 
- 将 Subcommands（rule、log）从 proxy 命令移为独立顶级命令
- `assh proxy rule reload` → `assh proxy-rule reload`
- `assh proxy log <id>` → `assh proxy-log <id>`

### 问题 2：端口映射逻辑错误

**现象**: `--http :8009 --ports 8009:4096` 启动了 SOCKS5 代理而非远程监听

**根因**: `startRemoteListenMode` 函数中的端口映射逻辑不正确

**解决方案**: 
- 修复 `localPort` 计算逻辑
- 使用 `ParsePorts` 函数正确解析端口映射规则

## 文档更新

- [x] `docs/TODO.md` - 更新 NEXT-001.4 状态为 ✅
- [x] `docs/worklogs/2026-05-29-NEXT-001-acceptance.md` - 创建验收报告

## 下一步

NEXT-001 阶段 3：服务管理持久化 (SQLite)
- NEXT-001.6: 服务领域实体 + 存储接口
- NEXT-001.7: SQLite 持久化实现
- NEXT-001.8: ServiceManager 接口 + 实现
- NEXT-001.9: ServiceManager 服务编排
- NEXT-001.10: assh service 命令组
- NEXT-001.11: 数据库迁移
- NEXT-001.12: DI 注入
- NEXT-001.13: 自动重连/恢复
