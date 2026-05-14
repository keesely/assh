---
id: RQ-001
title: CLI 组合短参数支持（-qnt 风格）
status: accepted
priority: medium
created: 2026-05-10
updated: 2026-05-10
source: user
phase: 5
related_adr: []
related_rq: []
tags: [cli, ux]
---

## 描述

`assh` CLI 应支持像 SSH 那样使用组合短参数，例如：

```bash
assh root@host -qnt
```

等价于：

```bash
assh root@host -q -n -t
```

即，多个无值的短参数可以合并为一个 token，`-abc` 等效于 `-a -b -c`。

## 动机

- SSH、git、tar 等标准 Unix 工具均支持组合短参数，用户习惯已养成
- 减少击键次数，提升日常使用效率
- 与 v1 版本的 CLI 体验保持一致（如果 v1 支持）

## 当前状态

assh 基于 urfave/cli v1.22.2，该框架**不原生支持**组合短参数。当前 `-qnt` 会被解析为一个未知 flag，导致错误。

### 当前短参数清单

| 短参 | 类型 | 含义 | 备注 |
|------|------|------|------|
| `-v` | bool | 详细输出 | 可组合 |
| `-q` | bool | 安静模式 | 可组合 |
| `-V` | bool | 版本号 | 可组合 |
| `-F` | string | 配置路径 | 带值，不可组合 |
| `-c` | string | 远程命令 | 带值，不可组合 |
| `-p` | int | 端口 | 带值，不可组合 |
| `-u` | string | 用户名 | 带值，不可组合 |
| `-l` | string | 登录用户名 | 带值，不可组合 |
| `-P` | string | 密码 | 带值，不可组合 |
| `-i` | string | 密钥文件 | 带值，不可组合 |
| `-k` | string | 密钥文件 | 带值，不可组合 |
| `-H` | string | 主机地址 | 带值，不可组合 |

**可组合** = bool 类型，无值参数
**不可组合** = string/int 类型，需要值参数

## 实现方案（草案）

### 方案：Arg 预处理器

在 `main.go` 中，`app.Run(os.Args)` 之前对 `os.Args` 做预处理：

```go
func expandCombinedFlags(args []string, knownBoolFlags map[string]bool) []string {
    var result []string
    for _, arg := range args {
        if strings.HasPrefix(arg, "--") || arg == "-" || !strings.HasPrefix(arg, "-") {
            // 长参数、单个"-"、非参数 → 保持原样
            result = append(result, arg)
            continue
        }
        if len(arg) == 2 {
            // 已经是单短参数如 "-q" → 保持原样
            result = append(result, arg)
            continue
        }
        // 检查组合参数如 "-qnt" 
        allBool := true
        for _, ch := range arg[1:] {
            if !knownBoolFlags[string(ch)] {
                allBool = false
                break
            }
        }
        if allBool {
            // 全部是 bool flag → 展开
            for _, ch := range arg[1:] {
                result = append(result, "-"+string(ch))
            }
        } else {
            // 包含带值参数 → 保持原样（让 urfave/cli 报错）
            result = append(result, arg)
        }
    }
    return result
}
```

**已知 bool flag 白名单**：`{"v": true, "q": true, "V": true}`

**边界处理**：
- `-p22`（port 值紧接）→ 当前不会展开（urflags/cli 会通过 `-p22` 取到值 "22"，事实上 urfave/cli 的 IntFlag 支持 `-p22` 语法）
- `-abc` 中包含未知 flag → 保持原样，让 urfave/cli 报未知 flag 错误
- `--` 分隔符之后 → 保持原样（`--` 后的内容不是 flags）
- 组合中混入带值 flag（如 `-qpH`）→ 不展开，让 urfave/cli 报错

### 注意事项

- 预处理只需要处理 `os.Args` 的全局 flag 层。子命令的 flag（如 `--list`、`--search`）一般使用长参数，不受影响
- 未来新增 bool flag 时需同步更新白名单
- `-n`、`-t` 等新 flag 若未来加入，需要先定义为 bool 类型才能参与组合

## 验收标准

| # | 场景 | 操作 | 预期 |
|---|------|------|------|
| F1 | 两个 bool 组合 | `assh -qv root@host` | 等价于 `-q -v`，安静+详细 |
| F2 | 三个 bool 组合 | `assh -qVv root@host` | 等价于 `-q -V -v` |
| F3 | 单个 bool 不变 | `assh -q root@host` | 行为不变 |
| F4 | 混入带值参数 | `assh -qp root@host` | 报错未知 flag（不做静默处理） |
| F5 | 混合使用 | `assh -qv root@host -c "uptime"` | bool 组合 + 带值参数正常 |
| F6 | 长参数不受影响 | `assh --quiet --verbose root@host` | 行为不变 |
| F7 | 组合中的未知 flag | `assh -qx root@host` | 报错未知 flag `-x` |

## 关联

- [urfave/cli v1 issue about combined flags](https://github.com/urfave/cli/issues)
- 实现位置：`main.go` 中 `expandCombinedFlags()` + `app.Run()`
- 依赖：无（纯 CLI 层改动）
