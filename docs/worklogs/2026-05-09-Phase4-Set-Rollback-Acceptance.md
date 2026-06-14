# Phase 4 set/rollback 验收报告

> 日期: 2026-05-09
> 分支: dev/v2.0.0-phase-4
> 测试类型: E2E 全场景验收

---

## 验收摘要

| 项目 | 结果 |
|------|------|
| 测试场景 | 21 项 |
| 通过 | 21 ✅ |
| 失败 | 0 ❌ |
| 编译 | `go build ./...` ✅ |
| 静态检查 | `go vet ./...` ✅ |
| 单元测试 | `go test ./... -count=1` ✅ (7 包) |

---

## 验收清单

| # | 场景 | 操作 | 预期 | 结果 |
|---|------|------|------|------|
| S1 | 标准添加 | `server add server-a -H 10.0.0.1 -u admin -P pass1 --remark "original"` | v1 成功 | ✅ |
| S2 | 缺 host 报错 | `server add nohost` | 错误提示 | ✅ |
| S3 | 重复添加 | 再次 `server add server-a -H 10.0.0.1` | 重复错误 | ✅ |
| S4 | set 新建 | `server set server-b -H 10.0.0.2 -u root -P pass2` | v1 成功 | ✅ |
| S5 | set 更新 | `server set server-a -p 2222 --remark "updated via set"` | v2 成功 | ✅ |
| S6 | set 无 host | `server set server-c -p 22` | 错误提示 | ✅ |
| S7 | set 非法端口 | `server set server-c -H 10.0.0.3 -p 99999` | 错误提示 | ✅ |
| S8 | set 无参数 | `server set server-a` | 显示详情+hint | ✅ |
| S9 | set 密码+密钥 | `server set server-a -P newpass -i ~/.ssh/id_rsa` | v3 | ✅ |
| S10 | set 清除认证 | `server set server-a --clear-password --clear-key` | v4 Auth=none | ✅ |
| S11 | set Options | `server set server-a -o Compression=yes -o LogLevel=DEBUG` | v5 | ✅ |
| S12 | info 版本 | `server info server-a` | 显示 Version:5 | ✅ |
| S13 | ls 版本列 | `server ls` | 显示 Ver 列 | ✅ |
| S14 | rollback --list | `server rollback server-a --list` | 显示 5 条记录 | ✅ |
| S15 | rollback 版本 | `server rollback server-a 1` | 回滚到 v1 | ✅ |
| S16 | 回滚验证 | `server info server-a` | 配置恢复 + v6 | ✅ |
| S17 | rollback 条目 | `server rollback server-a --list` | 含类型=rollback | ✅ |
| S18 | rm 删除 | `server rm server-a` | 成功 | ✅ |
| S19 | mv 移动 | `server mv server-b server-c` | 成功 | ✅ |
| S20 | 版本不存在 | `server rollback server-c 99` | 错误提示 | ✅ |
| S21 | 服务器不存在 | `server rollback nonexistent 1` | 错误提示 | ✅ |

---

## 版本递增验证

| 操作 | 前后版本 |
|------|---------|
| server add | → v1 |
| server set (update port+remark) | v1 → v2 |
| server set (password+key) | v2 → v3 |
| server set (clear password+key) | v3 → v4 |
| server set (options) | v4 → v5 |
| server rollback to v1 | max(5)+1 → v6 (rollback type) |

---

## 回滚正确性验证

回滚到 v1 后 (`server rollback server-a 1`)：
- Host=10.0.0.1 ✓（原始值）
- Port=22 ✓（原始值，非 2222）
- Auth=password ✓（原始值，非 none）
- Remark="original" ✓（原始值）
- Options=空 ✓（非 Compression=yes）
- Version=6 ✓（rollback 追加新版本）
