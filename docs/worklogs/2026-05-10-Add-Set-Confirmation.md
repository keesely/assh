# add/set 确认流程工作报告

> 日期: 2026-05-10
> 分支: dev/v2.0.0-phase-4

---

## 变更摘要

为 `add` 和 `set` 命令增加确认流程：预览 → 确认 → 保存 → 结果

### 提交

`d426747` — feat: add confirmation flow for add/set commands

### 修改文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `cmd/server.go` | 修改 | 新增 previewServer()、askConfirmation()，修改 add/set action |

---

## 实现方案

### 流程设计

1. **Preview** - 显示待保存的服务器配置（简洁格式）
2. **Confirm** - 询问用户确认 (Y/n)
3. **Execute** - 保存并显示结果

### 辅助函数

```go
// previewServer 预览服务器配置信息，用于 add/set 确认前的预览
func (a *App) previewServer(s *domain.Server, name string) {
    fmt.Println("=== Server Preview ===")
    fmt.Printf("Name:    %s\n", fullName)
    fmt.Printf("Host:    %s\n", s.Host)
    fmt.Printf("Port:    %d\n", s.Port)
    fmt.Printf("User:    %s\n", s.User)
    // ... auth, remark, options
}

// askConfirmation 询问用户确认，返回 true 表示确认
func (a *App) askConfirmation(prompt string) bool {
    fmt.Print(prompt, " [Y/n]: ")
    input := strings.TrimSpace(strings.ToLower(reader.ReadString('\n')))
    return input == "" || input == "y" || input == "yes"
}
```

### 交互示例

```
$ assh add test-srv --host 10.0.0.1 --user admin --password pwd
=== Server Preview ===
Name:    test-srv
Host:    10.0.0.1
Port:    22
User:    admin
Auth:    password
Add server "test-srv"? [Y/n]: y
server "test-srv" added (v1)

$ assh set test-srv --remark "updated"
=== Server Preview ===
...
update server "test-srv"? [Y/n]: n
cancelled
```

---

## 验证

```bash
go build ./...    ✅ 编译通过
go test ./...    ✅ 测试通过

# 交互测试
echo "y" | assh add xxx --host ...  ✅ 确认后保存
echo "" | assh add xxx --host ...   ✅ Enter 默认确认
echo "n" | assh add xxx --host ...  ✅ 取消不保存
```

---

## ADR 判断

本项为 UX 增强，使用框架已有功能，未触及 ADR 触发条件，无需新建 ADR。