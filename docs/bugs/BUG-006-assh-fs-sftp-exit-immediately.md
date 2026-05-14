# BUG-006: assh-fs sftp 连接后立即退出，无法进入交互模式

## Status
fixed

## Date
2026-05-10

## Description

执行 `assh-fs sftp case.bbp2` 时，连接成功后直接退出，无法进入交互式 SFTP 会话。

**复现步骤**：
```bash
$ ./assh-fs sftp case.bbp2
Connected to case.bbp2
Interactive SFTP session (type 'exit' to quit)
# 直接退出，会话结束
```

**预期行为**：连接成功后应进入交互式循环，等待用户输入命令。

## Root Cause

`sftp` 命令的 Action 函数在成功建立 SFTP 连接后，直接返回 nil，没有进入交互循环。

代码问题（`cmd/app.go`）：
```go
Action: func(c *cli.Context) error {
    // ... 连接代码 ...
    defer sftpClient.Close()
    fmt.Printf("Connected to %s\n", serverName)
    fmt.Println("Interactive SFTP session (type 'exit' to quit)")
    return nil  // ← 问题：直接返回，没有交互循环
}
```

## Solution

在 `cmd/app.go` 的 sftp 命令 Action 中添加完整的交互式循环：

1. 使用 `bufio.Scanner` 读取用户输入
2. 解析命令和参数
3. 实现完整的 SFTP 交互命令集：
   - `ls/cd/pwd` — 远程目录操作
   - `get/put` — 文件传输
   - `mkdir/rmdir/rm` — 文件操作
   - `lls/lcd/lpwd` — 本地目录操作
   - `!<cmd>` — 本地命令执行
   - `help/quit` — 帮助和退出

## Affected Files

- `cmd/app.go` — sftp 命令 Action 函数 + 辅助函数

## Verification

```bash
$ ./assh-fs sftp case.bbp2
Connected to case.bbp2
Type 'help' for available commands, 'quit' to exit.
sftp> ls
drwxr-xr-x    12288 etc/
drwxr-xr-x     4096 home/
...
sftp> quit
Goodbye!
```

## Related

- Phase 5 (SFTP 文件传输) 实现
- ADR-017: SFTP 文件传输架构设计