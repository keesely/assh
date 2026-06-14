# server edit 命令实现

> 日期: 2026-05-09
> 分支: dev/v2.0.0-phase-4
> 类型: 功能增强

---

## 1. 需求

用户需要修改已添加的服务器配置，包括：
- 修改主机地址、端口、用户名
- 修改/清除密码、密钥文件
- 添加/替换 Options
- 修改备注

**约束**：不能要求交互式输入密码（与 `add` 区分），仅修改显式指定的字段。

## 2. 设计方案

### 命令签名

```
assh server edit <name> [flags]
```

### 参数对齐（与 `add` 一致）

| Flag | 别名 | 用途 | 说明 |
|------|------|------|------|
| `-H/--host` | — | 更换主机地址 | `c.IsSet` 检测 |
| `-p/--port` | — | 更换端口 | `c.IsSet` 检测 |
| `-u/--user` | `-l/--login` | 更换用户名 | `firstNonEmpty` 处理别名 |
| `-P/--password` | — | 设置/更换密码 | `c.IsSet` 检测 |
| `-i/--identity-file` | `-k/--key` | 设置/更换密钥文件 | `firstNonEmpty` 处理别名 |
| `-o/--option` | — | 添加/替换 Options | `len(c.StringSlice)` 检测（`IsSet` 对 StringSliceFlag 不可靠） |
| `--remark` | — | 更换备注 | `c.IsSet` 检测 |
| `--clear-password` | — | 清除密码 | BoolFlag |
| `--clear-key` | — | 清除密钥文件 | BoolFlag |

### 核心设计

1. **深度拷贝**：从 `serverSvc.GetServer()` 获取现有记录后，深拷贝 Options map 和 Auth struct，防止意外修改
2. **仅修改指定字段**：对每个 flag 使用 `c.IsSet()` 判断是否显式指定（StringSlice 使用 `len()` 替代）
3. **空 Auth 清理**：密码和密钥都为空时，`Auth` 置为 nil
4. **无交互式密码**：不触发 `term.ReadPassword()`，与 `add` 的行为区别明显
5. **无 Flag 提示**：未指定任何修改标志时，显示当前服务器配置详情和使用提示

### 代码重构

提取 `printServerDetail(s *domain.Server)` 方法供 `edit` 和 `info` 共用，消除代码重复。

## 3. 实现细节

### 特殊处理

- `c.IsSet("option")` 在 urfave/cli v1.22 中对 StringSliceFlag 返回 false（已知问题），改用 `len(c.StringSlice("option")) > 0` 检测
- `--clear-password` 和 `--clear-key` 为独立 BoolFlag，与 `-P`/`-i` 互不影响
- Options 采用"增改"而非"替换"语义：已有选项保留，新选项添加/覆盖

### 文件变更

| 文件 | 变更 |
|------|------|
| `asshv2/cmd/server.go` | +125行：edit 子命令注册 + `serverEditAction` + `printServerDetail` 提取 |
| `asshv2/CHANGELOG.md` | Phase 4 条目更新 server edit |
| `docs/TODO.md` | 添加 P4.1-g 任务和验收条目 F11-F14 |
| `docs/worklogs/2026-05-09-Phase4-Server-Edit.md` | 新建 |

## 4. 验证结果

### 编译与静态检查

```bash
go build ./...  # ✅
go vet ./...    # ✅
go test ./... -count=1  # ✅ (7 包全部通过)
```

### 功能验收 (14 场景)

| # | 场景 | 操作 | 预期 | 结果 |
|---|------|------|------|------|
| 1 | 添加测试服务器 | `server add myserver -H 10.0.0.1 -u admin -P oldpass --remark original -o key1=val1` | 成功 | ✅ |
| 2 | 无标志编辑 | `server edit myserver` | 显示详情 + 提示 | ✅ |
| 3 | 修改 Host+Port | `server edit myserver -H 10.0.0.2 -p 2222` | Host/Port 变更 | ✅ |
| 4 | 修改 User(-u) | `server edit myserver -u operator` | User 变更 | ✅ |
| 5 | 修改 User(-l) | `server edit myserver -l root` | User 变更 | ✅ |
| 6 | 修改密码(-P) | `server edit myserver -P newpass` | 密码更新 | ✅ |
| 7 | 修改密钥(-i) | `server edit myserver -i ~/.ssh/id_ed25519` | 密钥更新 | ✅ |
| 8 | 修改密钥(-k) | `server edit myserver -k ~/.ssh/id_rsa` | 密钥更新（别名） | ✅ |
| 9 | 清除密码 | `server edit myserver --clear-password` | 密码清除 | ✅ |
| 10 | 清除密钥 | `server edit myserver --clear-key` | 密钥清除 | ✅ |
| 11 | 修改备注 | `server edit myserver --remark "edited"` | Remark 变更 | ✅ |
| 12 | 添加 Options | `server edit myserver -o newkey=newval -o Compression=yes` | Options 增改 | ✅ |
| 13 | 编辑不存在的服务器 | `server edit nonexistent -H 1.2.3.4` | 错误提示 | ✅ |
| 14 | 无名称编辑 | `server edit` | 错误提示 | ✅ |

## 5. Git 日志

```
3537eef docs: add Phase 4 CLI command assembly to CHANGELOG
6cb05cb chore: merge Phase 3 SSH connection layer into master
```
(当前版本基础上未提交变更)
