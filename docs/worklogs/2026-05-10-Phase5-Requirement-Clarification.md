# Phase 5 需求澄清记录

> 日期: 2026-05-10
> 分支: dev/v2.0.0-phase-5
> 状态: ✅ 需求已澄清，待开发

---

## 背景

Phase 5 (SFTP 文件传输) 已完成初步设计（ADR-017 + modules/sftp.md + TODO.md），但部分细分设计未明确。本轮对 7 个主题进行了逐一澄清。

---

## 澄清结果

### ① 进度显示格式

**确认格式**：
```
( 1/5) [project.tar.gz] .............. 58%  2.1MB/s  ETA 4s
( 2/5) [config.json] ................. 100%  5.3MB/s  ETA 0s
```

**回调接口**：
```go
type ProgressInfo struct {
    Index    int     // 当前文件序号（1-based）
    Total    int     // 总文件数
    FileName string  // 当前文件名
    Progress float64 // 0.0 ~ 1.0
    Rate     string  // 格式化速率
    ETA      string  // 格式化剩余时间
}
type TransferProgress func(info ProgressInfo)
```

### ② 批量传输行为

| 决策 | 确认 |
|------|------|
| 单文件失败 | **继续传后续文件**，不中断整个操作 |
| ya/na 作用域 | **本次会话有效**，退出命令后重置 |
| 默认询问 | 每个文件弹确认，ya=全部覆盖，na=全部跳过 |

### ③ 远程路径不存在

| 决策 | 确认 |
|------|------|
| push 时远程目录不存在 | **询问是否创建** |
| pull 时本地目录不存在 | **询问是否创建**（行为一致） |
| 不自动创建 | 需要用户确认 |

### ④ Glob/递归目录结构

| 策略 | 行为 |
|------|------|
| Glob 匹配 | **平铺**到目标目录（`"*.log" → /target/*.log`） |
| 递归目录 `-r` | **保持目录结构**（`./dir/sub/ → /target/sub/`） |
| pull 侧 Glob | **支持**，平铺到本地目录 |

> ⚠️ 纠正记录：Glob 平铺 vs 递归保持目录结构，两者策略不同

### ⑤ 交互式 SFTP 命令集

完整命令（与 OpenSSH sftp 对齐）：

| 分类 | 命令 |
|------|------|
| 导航 | `ls`, `cd`, `pwd` |
| 传输 | `get`, `put` |
| 远程操作 | `rm`, `mkdir`, `rmdir`, `chmod`, `rename`, `ln` |
| 本地操作 | `lcd`, `lpwd`, `lls`, `!<cmd>` |
| 系统 | `df`, `umask` |
| 帮助 | `help`, `quit`, `exit` |

### ⑥ 断点续传边界

| 场景 | 行为 |
|------|------|
| 远程 > 本地 | 报错"远程文件比本地大，续传不可用"，提示用 `-f` 覆盖 |
| 远程 == 本地 | 跳过（信任大小比对） |
| 远程 == 本地但内容不同 | 用户使用 `-S/--checksum` 启用 SHA256 验证 |

### ⑦ 并发模型

| 层级 | 控制方式 | 默认值 |
|------|----------|--------|
| 单文件内部 | pkg/sftp `MaxConcurrentRequestsPerFile` | 64 |
| 多文件间 | `TransferOptions.Concurrency` goroutine 池 | 3 |

---

## 文档变更

| 文档 | 变更内容 |
|------|----------|
| `docs/modules/sftp.md` | 新增 ProgressInfo 接口定义、完整交互式命令表、更新关键设计（覆盖策略/批量行为/并发模型/断点续传边界） |
| `docs/TODO.md` | 更新进度显示格式示例、TransferOptions 增加 `-S` 短参、批量行为说明、验收标准新增 F18-F21 |

## 设计完整度

所有 Phase 5 需求均已澄清，可以进入编码实现阶段。
