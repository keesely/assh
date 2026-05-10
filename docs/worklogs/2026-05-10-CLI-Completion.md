# CLI Shell 补全功能工作报告

> 日期: 2026-05-10
> 分支: dev/v2.0.0-phase-4

---

## 变更摘要

为 CLI 添加 shell 补全支持，用户在输入服务器名时按 Tab 可自动补全。

### 提交

`632ed8f` — feat: add CLI shell completion for server names

### 修改文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `cmd/app.go` | 修改 | 新增 `registerCompletionHints()` + `serverNameCompletion()` |

---

## 实现方案

### 技术选型

使用 **urfave/cli v1** 内置的 `BashComplete` 机制：

```go
cli.Command{
    Name: "info",
    BashComplete: func(c *cli.Context) {
        // 输出服务器名列表
    },
}
```

### 实现细节

1. **启用补全**: `app.EnableBashCompletion = true` (已有)
2. **注册处理器**: 在 `NewApp` 中调用 `registerCompletionHints()` 为目标命令注册
3. **目标命令**: `info`, `rm`, `mv`, `rollback`, `login`, `run`, `bc`
4. **补全逻辑**: 调用 `serverSvc.ListServers()` 获取所有服务器名并输出

### 代码结构

```go
// registerCompletionHints 为需要服务器名补全的命令注册 BashComplete
func (a *App) registerCompletionHints() {
    serverNameCommands := []string{"info", "rm", "mv", "rollback", "login", "run", "bc"}
    for i := range a.cli.Commands {
        cmd := &a.cli.Commands[i]
        for _, name := range serverNameCommands {
            if cmd.Name == name {
                cmd.BashComplete = a.serverNameCompletion
            }
        }
    }
}

// serverNameCompletion 提供服务器名补全
func (a *App) serverNameCompletion(c *cli.Context) {
    servers, err := a.serverSvc.ListServers()
    // 输出 domain.JoinName(group, name)
}
```

---

## 使用方式

```bash
# 方法 1: 手动 source
source <(assh completion bash)

# 方法 2: 安装到 bash_completion.d
sudo cp autocomplete/bash_autocomplete /etc/bash_completion.d/assh
source /etc/bash_completion.d/assh

# 使用
assh info <Tab>     # 列出所有服务器名
assh rm <Tab>       # 列出所有服务器名
assh login <Tab>    # 列出所有服务器名
```

---

## 验证

```
go build ./...    ✅ 编译通过
go test ./...    ✅ 测试通过
```

---

## ADR 判断

**触发条件检查**:

| 条件 | 本次 |
|------|------|
| 新技术栈引入 | ❌ urfave/cli v1 已内置 |
| 不兼容变更 | ❌ |
| 架构模式调整 | ❌ |
| 安全与合规 | ❌ |
| 核心流程重构 | ❌ |

**结论**: 无需新建 ADR，Shell 补全是 UX 增强，使用框架已有功能实现。