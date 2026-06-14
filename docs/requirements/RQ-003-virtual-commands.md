# RQ-003: 虚拟命令投放与执行 (X2)

> 状态：✅ 已确认
> 创建日期：2026-06-14
> 优先级：实验性
> MVP 范围：@push/@pull + 配置文件

---

## 1. 概述

### 1.1 核心概念

**虚拟命令**是 SSH 客户端的本地命令扩展机制。用户在 SSH 会话中输入以 `@` 为前缀的命令，assh 客户端拦截并在本地执行对应操作，而非发送到远程服务器。

### 1.2 使用场景

- 快速上传/下载文件，无需新开终端
- 执行本地脚本到远程服务器
- 标准化运维操作
- 提升 SSH 工作效率

### 1.3 设计原则

- **非侵入性**：前缀 `@` 明确区分虚拟命令和远程命令
- **可扩展性**：支持内置命令 + 自定义命令
- **复用性**：复用现有 SFTP 等基础设施
- **简洁性**：MVP 聚焦核心功能

---

## 2. 功能规格

### 2.1 命令触发机制

| 特性 | 规格 |
|------|------|
| 前缀符号 | `@` |
| 触发方式 | 用户在 SSH shell 中输入 `@command [args...]` |
| 命令识别 | assh 客户端拦截以 `@` 开头的输入 |
| 冲突处理 | `@command` 始终执行本地虚拟命令，`command` 发送到远程 |

### 2.2 内置命令列表

#### 文件传输命令

| 命令 | 语法 | 说明 |
|------|------|------|
| `@push` | `@push [-r] <local> [remote]` | 上传本地文件到远程服务器 |
| `@pull` | `@pull [-r] <remote> [local]` | 从远程服务器下载文件 |

**参数说明**：
- `-r`：递归上传/下载目录
- `<local>`：本地文件路径
- `[remote]`：远程路径（可选，默认为远程当前目录）

**路径处理**：
- 支持相对路径和绝对路径
- 相对路径基于当前工作目录
- `~` 展开为用户主目录

#### 执行命令

| 命令 | 语法 | 说明 |
|------|------|------|
| `@run` | `@run <command>` | 在本地执行 shell 命令 |
| `@deploy` | `@deploy <script>` | 上传脚本到远程并执行 |

**@deploy 流程**：
1. 检查本地脚本文件是否存在
2. 通过 SFTP 上传到远程 `/tmp/` 目录
3. 设置执行权限
4. 执行脚本
5. 返回执行结果

#### 系统命令

| 命令 | 语法 | 说明 |
|------|------|------|
| `@help` | `@help [command]` | 显示帮助信息 |
| `@version` | `@version` | 显示 assh 版本 |
| `@status` | `@status` | 显示当前连接状态 |
| `@reconnect` | `@reconnect` | 重新建立 SSH 连接 |

### 2.3 命令参数传递

支持混合模式：

```bash
# 位置参数
@push ./local-file.txt /remote/path/

# 命名参数
@push --source=./local-file.txt --target=/remote/path/

# 混合模式
@push ./local-file.txt --target=/remote/path/ -r
```

**参数映射**：
- `$1`, `$2`, ...：位置参数
- `$@`：所有参数
- `$*`：所有参数（单字符串）

### 2.4 帮助系统

```bash
# 显示所有虚拟命令列表
@help

# 显示指定命令的详细帮助
@help push
@help deploy
```

**帮助输出格式**：
```
Available virtual commands:
  @push    Upload file to remote server
  @pull    Download file from remote server
  @run     Execute local command
  @deploy  Upload and execute script
  @help    Show this help
  @version Show version
  @status  Show connection status
  @reconnect Reconnect to server

Use '@help <command>' for more information.
```

---

## 3. 配置规格

### 3.1 配置文件

| 属性 | 值 |
|------|-----|
| 位置 | `~/.assh/v2/virtual.yml` |
| 格式 | YAML |
| 编码 | UTF-8 |
| 作用域 | 全局生效 |

### 3.2 配置结构

```yaml
# ~/.assh/v2/virtual.yml

# 虚拟命令配置
virtual-commands:
  # 内置命令配置
  builtin:
    push:
      enabled: true
      timeout: 30s
    pull:
      enabled: true
      timeout: 30s
    deploy:
      enabled: true
      timeout: 300s
    run:
      enabled: true
      timeout: 60s

  # 自定义命令
  custom:
    # 示例：快速部署
    quick-deploy:
      description: "Quick deploy to server"
      script: |
        echo "Deploying..."
        @push ./dist/ /var/www/html/
        @run "systemctl restart nginx"
      timeout: 120s

    # 示例：备份命令
    backup:
      description: "Backup remote database"
      script: |
        @run "mysqldump -u root mydb > /tmp/backup.sql"
        @pull /tmp/backup.sql ./backups/
      timeout: 600s

    # 示例：健康检查
    healthcheck:
      description: "Check server health"
      script: |
        @run "uptime"
        @run "df -h"
        @run "free -m"
      timeout: 30s

  # 全局设置
  settings:
    # 错误处理：continue | exit
    on-error: continue
    # 输出格式：color | plain | json
    output: color
    # 命令历史：false
    history: false
```

### 3.3 自定义命令语法

自定义命令使用 Shell 脚本语法，支持：

- 变量替换：`$1`, `$2`, `$@`
- 虚拟命令调用：`@push`, `@pull`, `@run`
- 条件判断：`if [ ... ]; then ... fi`
- 循环：`for i in ...; do ... done`

**示例**：

```yaml
custom:
  deploy-env:
    description: "Deploy with environment"
    script: |
      ENV=$1
      if [ "$ENV" = "prod" ]; then
        @push ./dist/ /var/www/prod/
      else
        @push ./dist/ /var/www/staging/
      fi
      @run "systemctl restart app"
    timeout: 180s
```

---

## 4. 行为规格

### 4.1 错误处理

| 场景 | 行为 |
|------|------|
| 命令不存在 | 显示错误信息，保持 SSH 会话 |
| 参数错误 | 显示用法提示，保持 SSH 会话 |
| 文件不存在 | 显示错误信息，保持 SSH 会话 |
| 连接断开 | 显示错误信息，尝试重连 |
| 执行超时 | 显示超时信息，保持 SSH 会话 |

**错误输出格式**（带颜色）：
```
[ERROR] @push: local file not found: ./nonexistent.txt
```

### 4.2 输出格式

默认使用带颜色的输出：

| 类型 | 颜色 |
|------|------|
| 成功信息 | 绿色 |
| 错误信息 | 红色 |
| 警告信息 | 黄色 |
| 信息提示 | 蓝色 |
| 命令回显 | 灰色 |

**示例**：
```
[INFO] @push: uploading ./file.txt → /remote/path/file.txt
[INFO] @push: 1024 bytes transferred (1.2 MB/s)
[OK] @push: completed successfully
```

### 4.3 命令优先级

当虚拟命令与远程命令同名时：

| 输入 | 行为 |
|------|------|
| `@push file` | 执行本地虚拟命令 |
| `push file` | 发送到远程服务器执行 |

前缀 `@` 是明确的区分标识，不存在歧义。

### 4.4 超时处理

| 命令 | 默认超时 | 可配置 |
|------|----------|--------|
| @push | 30s | ✅ |
| @pull | 30s | ✅ |
| @run | 60s | ✅ |
| @deploy | 300s | ✅ |
| 自定义命令 | 60s | ✅ |

超时后：
1. 发送 SIGTERM 给正在执行的操作
2. 等待 5s
3. 如果仍未结束，发送 SIGKILL
4. 显示超时错误信息

---

## 5. 技术规格

### 5.1 架构设计

```
SSH Session (tty)
    │
    ▼
Input Handler
    │
    ├── starts with '@' → Virtual Command Handler
    │                         │
    │                         ├── @push → SFTP Upload
    │                         ├── @pull → SFTP Download
    │                         ├── @run → Local Exec
    │                         ├── @deploy → Upload + Exec
    │                         └── @help/@version/@status → ...
    │
    └── otherwise → Send to Remote Shell
```

### 5.2 模块划分

```
asshc/
├── domain/
│   └── virtual.go        # VirtualCommand 实体
├── port/
│   └── virtual.go        # VirtualCommandHandler 接口
├── service/
│   └── virtual.go        # VirtualCommandService 实现
└── infra/
    └── virtual/
        ├── handler.go    # 命令处理器
        ├── push.go       # @push 实现
        ├── pull.go       # @pull 实现
        ├── run.go        # @run 实现
        ├── deploy.go     # @deploy 实现
        └── config.go     # 配置加载
```

### 5.3 复用现有组件

| 组件 | 复用方式 |
|------|----------|
| `asshc/infra/sftp/` | @push/@pull 的文件传输 |
| `asshc/infra/ssh/` | SSH 连接管理 |
| `config/` | 配置文件加载 |
| `log/` | 日志输出 |

### 5.4 接口定义

```go
// VirtualCommand 虚拟命令实体
type VirtualCommand struct {
    Name        string
    Description string
    Usage       string
    Handler     VirtualCommandHandler
    Timeout     time.Duration
}

// VirtualCommandHandler 命令处理器
type VirtualCommandHandler interface {
    Execute(ctx context.Context, args []string, session SSHSession) error
}

// VirtualCommandService 虚拟命令服务
type VirtualCommandService interface {
    // Register 注册命令
    Register(cmd *VirtualCommand) error
    // Execute 执行命令
    Execute(ctx context.Context, input string, session SSHSession) (bool, error)
    // List 列出所有命令
    List() []*VirtualCommand
    // GetCommand 获取命令
    GetCommand(name string) (*VirtualCommand, error)
}
```

---

## 6. MVP 范围

### 6.1 Phase 1: 核心框架

- [ ] 虚拟命令识别和拦截机制
- [ ] 命令注册和路由
- [ ] 配置文件加载
- [ ] 帮助系统 (@help)

### 6.2 Phase 2: 文件传输

- [ ] @push 命令实现
- [ ] @pull 命令实现
- [ ] 递归传输 (-r 参数)
- [ ] 进度显示

### 6.3 Phase 3: 执行命令

- [ ] @run 命令实现
- [ ] @deploy 命令实现
- [ ] 超时处理

### 6.4 Phase 4: 系统命令

- [ ] @version 命令
- [ ] @status 命令
- [ ] @reconnect 命令

### 6.5 Phase 5: 自定义命令

- [ ] 自定义命令解析
- [ ] 变量替换
- [ ] 虚拟命令嵌套调用

---

## 7. 验收标准

### 7.1 功能验收

```bash
# 1. 启动 SSH 会话
assh login server1

# 2. 测试 @push
@push ./local-file.txt /remote/path/
# 预期：文件上传成功，显示进度

# 3. 测试 @pull
@pull /remote/file.txt ./local-path/
# 预期：文件下载成功

# 4. 测试 @run
@run ls -la
# 预期：本地执行 ls -la

# 5. 测试 @deploy
@deploy ./deploy.sh
# 预期：脚本上传并执行

# 6. 测试 @help
@help
# 预期：显示命令列表

@help push
# 预期：显示 @push 详细帮助

# 7. 测试 @version
@version
# 预期：显示 assh 版本

# 8. 测试 @status
@status
# 预期：显示连接状态

# 9. 测试命令冲突
@push file  # 本地执行
push file   # 远程执行
# 预期：@push 本地，push 远程
```

### 7.2 配置验收

```bash
# 1. 创建配置文件
cat > ~/.assh/v2/virtual.yml << EOF
virtual-commands:
  builtin:
    push:
      enabled: true
      timeout: 30s
  custom:
    hello:
      description: "Say hello"
      script: echo "Hello from virtual command!"
      timeout: 10s
EOF

# 2. 测试内置命令
@push ./file.txt /tmp/
# 预期：正常工作

# 3. 测试自定义命令
@hello
# 预期：输出 "Hello from virtual command!"
```

### 7.3 错误处理验收

```bash
# 1. 测试文件不存在
@push ./nonexistent.txt /tmp/
# 预期：红色错误信息，保持会话

# 2. 测试参数错误
@push
# 预期：显示用法提示

# 3. 测试超时
@run "sleep 100"  # 假设超时 60s
# 预期：60s 后超时错误
```

---

## 8. 未来扩展

### 8.1 可能的增强

- 命令历史记录（SQLite 存储）
- 命令别名系统
- 命令参数自动补全
- 命令执行结果缓存
- 批量服务器执行
- 命令录制和回放

### 8.2 与其他功能集成

- 与 Phase 12（挂载）集成：@mount, @umount
- 与 Phase 13（命令执行增强）集成：超时、输出格式化
- 与 NEXT-001（代理/隧道）集成：@proxy, @tunnel

---

## 9. 参考资料

- Git alias 配置格式
- Slack/Telegram 斜杠命令
- Ansible 模块设计
- OpenSSH 扩展机制

---

## 10. 变更记录

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2026-06-14 | 1.0 | 初始需求确认 |
