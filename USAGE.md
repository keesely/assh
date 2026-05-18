# ASSH v2 使用手册

> ASSH (An SSH Client) — 使用 Go 开发的 SSH 客户端，适用于服务器管理、维护和快速使用。
> 基于 SQLite 加密存储，支持多认证方式，双二进制架构（`assh` + `assh-fs`）。

---

## 目录

1. [简介](#1-简介)
2. [安装与构建](#2-安装与构建)
3. [快速入门（5分钟上手）](#3-快速入门5分钟上手)
4. [服务器管理](#4-服务器管理)
5. [SSH 连接](#5-ssh-连接)
6. [文件传输](#6-文件传输)
7. [密钥管理](#7-密钥管理)
8. [代理与隧道](#8-代理与隧道)
9. [云同步](#9-云同步)
10. [健康检查](#10-健康检查)
11. [配置参考](#11-配置参考)
12. [命令速查表](#12-命令速查表)
13. [常见问题与技巧](#13-常见问题与技巧)

---

## 1. 简介

### 1.1 功能总览

| 功能 | 说明 | 对应命令 | 实现阶段 |
|------|------|----------|:--------:|
| 服务器管理 | 增删改查、分组、命名规则 (group.server) | `assh add/set/ls/info/rm/mv/rollback` | Phase 4 |
| SSH 连接 | 交互式登录、远程命令执行、批量并发 | `assh login/run/bc` | Phase 4 |
| 文件传输 | push/pull、递归、断点续传、校验 | `assh-fs push/pull` | Phase 5 |
| 密钥管理 | RSA/Ed25519/ECDSA 生成、部署、备份 | `assh keygen` | Phase 6 |
| 代理 & 隧道 | SOCKS5/HTTP CONNECT/AutoProxy/端口转发 | `assh proxy/tunnel` | Phase 7 |
| 云同步 | 七牛云加密同步服务器配置 | `assh sync` | Phase 8 |
| 健康检查 | SSH 连接延迟 + 系统指标采集 | `assh health` | Phase 9 |
| 加密存储 | AES-256-GCM + SQLite，自动密钥管理 | 内置 | Phase 2 |
| 版本回滚 | 配置变更自动快照，支持任意版本回滚 | `assh rollback` | Phase 4 |

### 1.2 双二进制架构

ASSH v2 构建为两个独立的可执行文件，共享同一核心代码：

```
assh               assh-fs
SSH 连接/管理      文件系统操作
├── login/run/bc   ├── push/pull
├── server 管理     └── <server> (交互式 SFTP)
├── keygen
├── proxy/tunnel
├── sync
└── health
    │
    └── 共用核心层 (asshc/)
```

- **`assh`** — 主命令行工具，处理连接、管理、代理等所有操作
- **`assh-fs`** — 文件操作专用工具，处理 SFTP 传输

### 1.3 数据安全

- 服务器密码使用 **AES-256-GCM** 加密后存入 SQLite
- 加密密钥首次运行自动生成，存储在数据库中
- 密钥通过 Base64 编码持久化
- 静态密钥自动管理，无需用户干预

---

## 2. 安装与构建

### 2.1 从源码构建

```bash
# 克隆仓库
git clone https://github.com/keesely/assh.git
cd assh/asshv2

# 构建所有平台（macOS / Linux / Windows，含 amd64/386/arm）
sh build.sh

# 构建产物在 build/ 目录
ls build/
# assh-macOS-amd64_v2.0.0.zip
# assh-linux-amd64_v2.0.0.zip
# ...
```

### 2.2 单平台构建

```bash
# 语法: sh build.sh <os> <arch> <alias>
# 示例：构建 Linux amd64
sh build.sh linux amd64 linux

# 构建当前系统（直接运行）
sh build.sh
```

每个构建产物包含两个二进制文件：
- **`assh`** — 主程序（连接/管理/代理等）
- **`assh-fs`** — 文件传输工具

### 2.3 快速运行（不安装）

```bash
cd asshv2
go run . login myserver          # 直接运行 assh 命令
go run ./cmd/fs/ pull ...        # 直接运行 assh-fs
```

### 2.4 安装到系统

```bash
# 解压对应平台的构建包
unzip build/assh-linux-amd64_v2.0.0.zip -d ~/bin
# 或复制到 PATH 目录
cp build/assh-linux-amd64_v2.0.0/assh /usr/local/bin/
cp build/assh-linux-amd64_v2.0.0/assh-fs /usr/local/bin/
```

### 2.5 系统要求

- **Go 版本**: 1.22.0+
- **运行环境**: macOS / Linux / Windows（无 CGO 依赖）
- **存储空间**: 约 20MB（二进制）+ 数据库空间

---

## 3. 快速入门（5分钟上手）

### 场景：管理三台服务器

假设有三台服务器需要管理：
- `web-01` (192.168.1.10) — Web 服务器，root 密码登录
- `db-01` (10.0.0.20) — 数据库服务器，密钥登录
- `dev-box` (dev.example.com) — 开发机，SSH Agent 认证

### 第一步：添加服务器

```bash
# 添加 Web 服务器（使用密码）
assh add web-01 -H 192.168.1.10 -u root -P mypassword -p 22

# 添加数据库服务器（使用密钥）
assh add db-01 -H 10.0.0.20 -u admin -i ~/.ssh/id_rsa

# 添加开发机（使用分组名 - 分组.服务器名 格式）
assh add dev.dev-box -H dev.example.com -u deploy

# 查看已添加的服务器
assh ls
```

### 第二步：登录服务器

```bash
# 按名称登录
assh login web-01

# 直接输入服务器名（等价于 login）
assh web-01

# 执行命令
assh run web-01 "uptime"
assh run web-01 "df -h"
```

### 第三步：批量操作

```bash
# 在分组内批量执行
assh bc "systemctl status nginx" --group dev

# 在指定服务器上并发执行
assh bc "free -m" --servers web-01,db-01

# 三通道输出（stdout/stderr 分离 + 汇总）
```

### 第四步：文件传输

```bash
# 推送配置文件
assh-fs push web-01 ./nginx.conf /etc/nginx/nginx.conf

# 拉取日志
assh-fs pull web-01 /var/log/nginx/access.log ./logs/
```

### 第五步：健康检查

```bash
# 检查单台服务器
assh health check web-01

# 检查所有服务器
assh health list
```

---

## 4. 服务器管理

### 4.1 命名规则

服务器名支持 **分组.主机** 格式：
- `prod.web-01` — 属于 prod 分组的 web-01 服务器
- `staging.api` — 属于 staging 分组的 api 服务器
- `myserver` — 无分组，独立服务器

> 分组通过点号 `.` 分隔，第一段为分组名，第二段为主机名。

### 4.2 添加服务器

```bash
# 基本添加
assh add myserver -H 192.168.1.100

# 指定所有参数
assh add myserver -H 192.168.1.100 -u root -P mypass -p 22

# 使用密钥认证
assh add myserver -H example.com -u deploy -i ~/.ssh/id_rsa

# 使用默认端口（22），不指定用户名（登录时交互输入）
assh add myserver -H 192.168.1.100

# 添加到分组
assh add prod.web -H 10.0.0.1 -u admin -P secret

# 带备注
assh add dev.node -H 10.0.0.2 -u dev -P devpass --remark "开发环境测试节点"

# 设置 Keepalive 心跳（秒）
assh set myserver -o keepalive=30

# 设置连接超时（秒）
assh set myserver -o timeout=10
```

### 4.3 查看服务器

```bash
# 列出所有服务器
assh ls

# 按分组筛选
assh ls --group prod

# 关键词搜索
assh ls --search web

# 查看服务器详情
assh info myserver

# 模糊匹配（当名称不完全匹配时给出建议）
assh info web
# 输出: 未找到 'web'，您是不是要找：web-01, web-02.prod ?
```

### 4.4 修改服务器

```bash
# 修改单个字段
assh set myserver -H new-host.example.com
assh set myserver -u newuser
assh set myserver -p 2222
assh set myserver --remark "更新后的备注"

# 修改密码
assh set myserver -P newpassword

# 修改密钥
assh set myserver -i ~/.ssh/new_key

# 清除密码（改为密钥认证）
assh set myserver --clear-password

# 清除密钥（改为密码认证）
assh set myserver --clear-key

# 设置 SSH 选项
assh set myserver -o keepalive=60 -o timeout=15
```

### 4.5 删除与移动

```bash
# 删除服务器
assh rm myserver

# 重命名
assh mv myserver newname

# 移动到分组
assh mv myserver prod.myserver

# 跨分组移动
assh mv dev.web-01 prod.web-01
```

### 4.6 版本回滚

每次服务器配置变更自动生成快照，支持回滚：

```bash
# 查看变更历史
assh rollback myserver --list
# 输出示例：
# Version | Date        | Change
# 3       | 2 min ago   | password changed
# 2       | 1 hour ago  | host changed
# 1       | 2 hours ago | server created

# 回滚到指定版本
assh rollback myserver 2

# 回滚到上一个版本
assh rollback myserver
```

### 4.7 服务器管理示例集合

```bash
# === 日常管理场景 ===

# 场景：添加一批新服务器
assh add web-01 -H 10.0.0.1 -u root -P pass1
assh add web-02 -H 10.0.0.2 -u root -P pass2
assh add db-01 -H 10.0.1.1 -u admin -P dbpass
assh set web-01 -o keepalive=30
assh set web-02 -o keepalive=30

# 场景：服务器迁移（更换 IP）
assh set web-01 -H 10.0.2.1

# 场景：密码轮换
assh set web-01 -P newpassword
# 记得记录变更
assh info web-01

# 场景：清理废弃服务器
assh ls --group old
assh rm old.server1
assh rm old.server2

# 场景：误操作回滚
assh rollback web-01 --list
assh rollback web-01 2
```

---

## 5. SSH 连接

### 5.1 交互式登录

```bash
# 通过已保存的服务器名登录
assh login myserver

# 快捷方式：直接输入服务器名即可
assh myserver

# 直接连接（不依赖已保存配置）
assh login root@192.168.1.100
assh login root@192.168.1.100:2222  # 指定端口
assh login user@host

# 使用 -H 参数分离登录
assh login -H 192.168.1.100 -u root -P password

# 使用密钥直接连接
assh login -H 192.168.1.100 -u root -i ~/.ssh/id_rsa

# 连接时指定端口
assh login -H 192.168.1.100 -u root -p 2222
```

### 5.2 执行远程命令

```bash
# 在单台服务器上执行命令
assh run myserver "uptime"
assh run myserver "df -h | grep sda"
assh run myserver "systemctl status nginx"

# 使用管道组合
assh run myserver "ps aux | grep nginx | wc -l"

# 多命令执行
assh run myserver "cd /var/www && git pull && npm install"
```

### 5.3 批量并发执行（bc）

`bc` 命令支持在多台服务器上并发执行命令，输出通过三通道（stdout/stderr/汇总）展示：

```bash
# 按服务器列表执行
assh bc "uptime" --servers web-01,web-02,db-01

# 按分组执行
assh bc "systemctl status nginx" --group prod

# 输出到文件
assh bc "date" --servers s1,s2,s3 --log ./result.json

# 组合使用
assh bc "free -m" --servers web-01,db-01 --log ./memory.json
```

**bc 三通道输出示例**：

```
=== Batch Command: uptime ===
Targets: web-01, web-02, db-01
Concurrency: 3

─── [web-01] stdout ──────────────────────
 10:23:45 up 30 days,  2:15,  0 users,  load average: 0.08, 0.03, 0.01

─── [web-02] stdout ──────────────────────
 10:23:46 up 15 days,  4:30,  0 users,  load average: 0.15, 0.10, 0.05

─── [db-01] stdout ───────────────────────
 10:23:46 up 60 days,  1:00,  0 users,  load average: 0.50, 0.45, 0.40

─── Summary ──────────────────────────────
  Executed: 3/3  Success: 3  Failed: 0
```

### 5.4 认证机制

SSH 连接时按以下顺序尝试认证：

1. **备份密钥优先**（如果 `known_servers` 表有记录，优先使用 `data/keys/` 的备份）
2. **SSH Agent**（通过 `SSH_AUTH_SOCK` 环境变量）
3. **密钥文件**（`-i` / `--key` 指定）
4. **密码**（`-P` / `--password` 或交互式输入）
5. **认证回退**（密钥认证失败后自动回退到密码认证）

```bash
# 使用 SSH Agent（无需额外参数，自动使用 agent 中的密钥）
assh login myserver

# 指定密钥文件
assh login myserver -i ~/.ssh/id_ed25519

# 交互式输入密码
assh login myserver
# 提示: Password for myserver:
```

### 5.5 连接示例集合

```bash
# === 日常操作场景 ===

# 快速登录查看状态
assh web-01
# 进入后执行: uptime, free -m, df -h, exit

# 一键重启服务
assh run web-01 "systemctl restart nginx"

# 批量检查磁盘
assh bc "df -h / | tail -1" --group prod

# 批量更新
assh bc "apt update && apt upgrade -y" --servers web-01,web-02

# 查看多台服务器的内存使用
assh bc "free -h | grep Mem" --servers web-01,db-01,redis-01

# 批量重启服务（带检查）
assh run web-01 "systemctl restart nginx && systemctl status nginx"
```

---

## 6. 文件传输

ASSH v2 使用独立的 `assh-fs` 二进制处理文件传输，支持 SFTP 协议的 push/pull 操作。

### 6.1 推送文件到远程（push）

```bash
# 推送单文件
assh-fs push myserver ./local/file.txt /remote/path/

# 递归推送目录
assh-fs push -r myserver ./local/dir/ /remote/dir/

# 断点续传（大文件传输中断后继续）
assh-fs push --resume myserver ./large.tar.gz /remote/
assh-fs push -e myserver ./large.iso /remote/       # 短参形式

# 强制覆盖已存在的文件
assh-fs push -f myserver ./file.txt /remote/

# 跳过已存在的文件
assh-fs push --skip myserver ./file.txt /remote/

# Glob 匹配推送（将匹配的文件平铺到目标目录）
assh-fs push myserver "*.log" /tmp/logs/
assh-fs push myserver "build/*.zip" /tmp/releases/

# 传输后 SHA256 校验
assh-fs push --checksum myserver ./important.iso /remote/

# 直连模式（不依赖已保存配置）
assh-fs push -H 192.168.1.1 -u root -P mypass ./file /remote/
```

### 6.2 从远程拉取文件（pull）

```bash
# 拉取单文件
assh-fs pull myserver /remote/file.txt ./local/

# 递归拉取目录
assh-fs pull -r myserver /remote/dir/ ./local/

# 断点续传拉取
assh-fs pull --resume myserver /remote/large.tar.gz ./

# Glob 匹配拉取（将匹配的文件平铺到本地目录）
assh-fs pull myserver "/var/log/*.log" ./logs/
assh-fs pull myserver "/etc/nginx/conf.d/*.conf" ./backup/

# 直连模式拉取
assh-fs pull -H 192.168.1.1 -u root -P mypass /remote/file ./

# 带校验拉取
assh-fs pull --checksum myserver /backup/db.sql.gz ./
```

### 6.3 交互式 SFTP 会话

```bash
# 直接进入交互式 SFTP 管理
assh-fs myserver

# 直连模式交互式
assh-fs -H 192.168.1.1 -u root
```

交互式 SFTP 支持的命令：

| 命令 | 说明 | 示例 |
|------|------|------|
| `ls` | 列出远程目录 | `ls`, `ls -la /var/log` |
| `cd` | 切换远程目录 | `cd /var/www` |
| `pwd` | 显示远程当前目录 | `pwd` |
| `get` | 下载文件 | `get remote.txt`, `get -r /remote/dir/` |
| `put` | 上传文件 | `put local.txt`, `put -r ./local/dir/` |
| `chmod` | 修改远程文件权限 | `chmod 755 script.sh` |
| `rename` | 重命名远程文件 | `rename old.txt new.txt` |
| `rm` | 删除远程文件 | `rm file.txt` |
| `rmdir` | 删除远程目录 | `rmdir emptydir` |
| `mkdir` | 创建远程目录 | `mkdir newdir` |
| `df` | 查看远程磁盘空间 | `df -h` |
| `symlink` | 创建远程符号链接 | `symlink target link` |
| `ln` | 创建远程硬链接 | `ln target link` |
| `lls` | 列出本地目录 | `lls`, `lls -la` |
| `lcd` | 切换本地目录 | `lcd ./downloads` |
| `lpwd` | 显示本地当前目录 | `lpwd` |
| `!` | 执行本地 shell 命令 | `!mkdir -p backups` |
| `help` | 显示帮助 | `help` |
| `exit` / `quit` | 退出 | `exit` |

### 6.4 进度显示

文件传输时的进度条格式：

```
( 1/5) [project.tar.gz] .............. 58%  2.1MB/s  ETA 4s
( 2/5) [config.json] ................. 100%  5.3MB/s  ETA 0s
( 3/5) [data.sql.gz] ................. 100%  12MB/s   ETA 0s

verifying: project.tar.gz
  ✓ size match: 12.3MB / 12.3MB
  ✓ sha256: a3f2c8d1... (--checksum 开启时)
```

### 6.5 文件传输示例集合

```bash
# === 日常操作场景 ===

# 场景：部署应用
assh-fs push -r myserver ./dist/ /var/www/app/
assh run myserver "systemctl restart nginx"

# 场景：备份日志
assh-fs pull myserver "/var/log/nginx/*.log" ./backups/$(date +%Y%m%d)/

# 场景：同步配置文件到多台服务器
assh-fs push web-01 ./nginx.conf /etc/nginx/nginx.conf
assh-fs push web-02 ./nginx.conf /etc/nginx/nginx.conf
assh bc "nginx -t && systemctl reload nginx" --servers web-01,web-02

# 场景：数据库备份拉取
assh run db-01 "mysqldump -u root mydb > /tmp/mydb.sql"
assh-fs pull db-01 /tmp/mydb.sql ./backups/
assh run db-01 "rm /tmp/mydb.sql"

# 场景：大文件传输（断点续传）
assh-fs push --resume myserver ./ubuntu-24.04.iso /var/iso/

# 场景：批量收集日志
for srv in web-01 web-02 db-01; do
  assh-fs pull $srv "/var/log/syslog" "./logs/$srv-syslog"
done
```

---

## 7. 密钥管理

ASSH v2 支持三种 SSH 密钥算法：**RSA**、**Ed25519**、**ECDSA**（P256/P384/P521），兼容 `ssh-keygen`。

### 7.1 生成密钥并部署到服务器

```bash
# 一键生成密钥并部署到服务器（自动备份）
assh keygen myserver

# 指定密钥类型（推荐 Ed25519）
assh keygen myserver --type ed25519
assh keygen myserver -t ed25519

# 指定 RSA 位数
assh keygen myserver --type rsa --bits 4096
assh keygen myserver -t rsa -b 4096

# 指定 ECDSA 曲线
assh keygen myserver -t ecdsa -b 256      # P256
assh keygen myserver -t ecdsa -b 384      # P384
assh keygen myserver -t ecdsa -b 521      # P521

# 指定密钥注释（默认 user@host）
assh keygen myserver -C "web-server-key-2026"
```

### 7.2 独立生成模式（不部署）

```bash
# 生成密钥到指定路径
assh keygen -f ~/.ssh/mykey --type ed25519

# 加密码保护私钥
assh keygen -f ~/.ssh/mykey -N "mypassphrase"

# 指定全部参数
assh keygen -f ~/.ssh/mykey -t rsa -b 4096 -C "my-key" -N "secret"

# 指定输出目录（自动生成文件名）
assh keygen -f ~/.ssh/ --type ed25519
# 生成: ~/.ssh/id_ed25519 和 ~/.ssh/id_ed25519.pub
```

### 7.3 交互式向导

```bash
# 无参运行，进入 ssh-keygen 风格交互式向导
assh keygen
```

交互式向导会逐步询问：
1. 生成路径（默认 `~/.ssh/id_算法`）
2. 密钥类型（RSA/Ed25519/ECDSA）
3. 密钥位数
4. 注释
5. 密码短语
6. 是否部署到服务器

### 7.4 直连模式部署

```bash
# 在远程服务器上生成并部署密钥
assh keygen -H 192.168.1.1 -u root -P password -t ed25519
```

### 7.5 密钥备份策略

生成的密钥自动管理，无需手动备份：

```
~/.assh/v2/data/keys/{fingerprint}/
├── id_ed25519          # 私钥备份
├── id_ed25519.pub      # 公钥
└── metadata.json       # 元数据（服务器名、指纹等）
```

### 7.6 密钥相关系统行为

- **备份密钥优先** — SSH 连接时，如果 `known_servers` 表有记录，优先使用 `data/keys/` 的备份密钥，不依赖 `~/.ssh/`
- **直连自动记录** — 直连登录的服务器自动记录到 known-servers 表，便于后续使用
- **多备份支持** — 同一密钥部署到多台服务器，会生成对应的备份引用

### 7.7 密钥管理示例集合

```bash
# === 常见场景 ===

# 场景：为新服务器生成 ed25519 密钥并部署
assh keygen new-server -t ed25519
# 1. 在本地生成 ed25519 密钥对
# 2. 自动备份到 ~/.assh/v2/data/keys/
# 3. 通过 SSH 将公钥部署到服务器的 ~/.ssh/authorized_keys
# 4. 更新服务器配置，使用新密钥

# 场景：为现有服务器更换密钥
assh keygen myserver -t ed25519 -C "2026-rotation"

# 场景：生成一个通用密钥，部署到多台服务器
assh keygen -f ~/.ssh/deploy-key -t ed25519 -C "deploy-key"
assh keygen -H server1 -u deploy -i ~/.ssh/deploy-key -P pass
assh keygen -H server2 -u deploy -i ~/.ssh/deploy-key -P pass

# 场景：使用密钥登录服务器
assh login myserver            # 自动使用备份密钥
assh login -H 10.0.0.1 -u root -i ~/.ssh/mykey    # 指定密钥
```

---

## 8. 代理与隧道

支持 SOCKS5 代理、HTTP CONNECT 代理、本地/远程端口转发，以及 Smart Proxy 智能代理系统。

### 8.1 SOCKS5 正向代理

通过 SSH 隧道建立 SOCKS5 代理，用于浏览器或应用的网络代理：

```bash
# 最简启动（默认监听 127.0.0.1:1080）
assh proxy myserver

# 指定监听地址和端口
assh proxy myserver --socks5 :1080
assh proxy myserver --socks5 0.0.0.0:1080

# SOCKS5 用户名密码认证
assh proxy myserver --auth user:pass

# 后台守护运行（不阻塞终端）
assh proxy myserver --daemon

# 自动重连（最多 3 次，间隔 5 秒）
assh proxy myserver --auto-reconnect=3/5s

# 直连模式
assh proxy -H 192.168.1.1 -u root -P password --socks5 :1080
```

### 8.2 HTTP CONNECT 代理

```bash
# 启动 HTTP CONNECT 代理
assh proxy myserver --http :8080

# 同时启动 SOCKS5 + HTTP（双协议监听）
assh proxy myserver --socks5 :1080 --http :8080

# HTTP 代理 + Basic Auth
assh proxy myserver --http :8080 --auth user:pass
```

### 8.3 Smart Proxy 智能代理

结合 AutoProxy 规则引擎和透明日志的智能路由系统。根据规则自动决定流量走代理还是直连：

```bash
# 加载 GFWList 规则文件（匹配规则的域名走代理，其余直连）
assh proxy myserver --autoproxy ./gfwlist.txt

# 启用透明日志（JSONL 格式，按日轮转）
assh proxy myserver --log-dir ./proxy-logs

# 规则引擎 + 日志 + 后台运行
assh proxy myserver --autoproxy ./list.txt --log-dir ./logs --daemon
```

**规则文件格式**（`gfwlist.txt`）：
```
# 注释行
# 域名后缀匹配
.google.com
.youtube.com
.twitter.com
.facebook.com
# CIDR 匹配
10.0.0.0/8
# 精确域名
example.com
# 正则匹配
^.*\.gov\.cn$
# 白名单前缀（始终直连）
!localhost
!192.168.
```

规则引擎运行时命令：

```bash
# 热重载规则（不重启代理）
assh proxy rule reload

# 查看规则状态
assh proxy rule status

# 查看指定会话的日志
assh proxy log <session-id>
```

### 8.4 反向代理模式

将远程服务器的端口映射到本地（类似 `ssh -R`）：

```bash
# 反向代理模式（默认 TCP）
assh proxy myserver --reverse

# 指定端口映射（server:80 → local:3000）
assh proxy myserver --reverse --ports 80:3000

# 多端口映射（server:80,443 → local:80）
assh proxy myserver --reverse --ports 80,443:80

# 端口范围展开 1:1（80-85 → 81-86）
assh proxy myserver --reverse --ports 80-85:81-86

# 反向代理 + HTTP 协议 + Basic Auth
assh proxy myserver --reverse --http --ports 8080:3000 --auth user:pass

# 反向代理 + 规则 + 日志（全功能）
assh proxy myserver --reverse --tcp --ports 80,443:80 \
  --auth user:pass --autoproxy ./list.txt --log-dir ./logs
```

### 8.5 端口转发隧道

```bash
# 本地端口转发（相当于 ssh -L）
assh tunnel start myserver -L 8080:localhost:80

# 远程端口转发（相当于 ssh -R）
assh tunnel start myserver -R 8080:localhost:3000

# 指定绑定地址
assh tunnel start myserver -L 0.0.0.0:8080:web:80

# 多端口转发
assh tunnel start myserver -L 8080:svc1:80 -L 9090:svc2:90

# 后台守护运行 + 自动重连
assh tunnel start myserver -L 8080:web:80 --daemon --auto-reconnect=3/5s

# 隧道管理
assh tunnel list                # 列出所有活跃隧道
assh tunnel stop <id>           # 停止指定隧道
```

### 8.6 端口映射规则语法

`--ports` 参数支持多种映射模式：

| 语法 | 含义 | 示例 |
|------|------|------|
| `L:R` | 本地端口 L 映射到远程端口 R | `80:3000` |
| `L1,L2:R` | 多个本地端口映射到同一远程端口 | `80,443:80` |
| `L1-L2:R1-R2` | 端口范围 1:1 映射 | `80-85:81-86` |
| `L` | 等号映射（L → L） | `8080` |

### 8.7 代理与隧道示例集合

```bash
# === 常见场景 ===

# 场景：浏览器安全上网
assh proxy myserver --socks5 :1080 --daemon
# 浏览器设置 SOCKS5 代理 127.0.0.1:1080

# 场景：访问内网服务（端口转发）
assh tunnel start myserver -L 8080:internal-web:80
# 浏览器访问 http://localhost:8080

# 场景：数据库远程管理
assh tunnel start myserver -L 3306:db.internal:3306
# 本地 MySQL 客户端连接 localhost:3306

# 场景：穿透内网（反向代理）
assh proxy myserver --reverse --ports 3000:3000
# 远程服务器的 localhost:3000 映射到本地的 3000 端口

# 场景：科学上网（AutoProxy + SOCKS5）
assh proxy myserver --socks5 :1080 --autoproxy ./gfwlist.txt --daemon

# 场景：多会话复用（后台代理 + 多命令执行）
assh proxy myserver --socks5 :1080 --daemon
# 在另一个终端执行命令，复用同一个 SSH 连接
assh run myserver "tail -f /var/log/nginx/access.log"

# 场景：远程开发（端口转发 + 代码同步）
assh tunnel start dev-box -L 9229:localhost:9229  # Node.js debug
assh tunnel start dev-box -L 3000:localhost:3000  # Web app
assh-fs push dev-box ./src/ /home/dev/app/ -r     # 同步代码
```

---

## 9. 云同步

ASSH 支持将服务器配置通过七牛云（Qiniu Kodo）进行加密同步，实现跨设备配置共享。

### 9.1 配置云账户

```bash
# 配置七牛云账号（首次使用）
assh sync account set
# 交互式输入：
#   Access Key: <your-qiniu-access-key>
#   Secret Key: <your-qiniu-secret-key>
#   Bucket:     <your-bucket-name>

# 一行式配置
assh sync account set --access-key AKxxx --secret-key SKxxx --bucket my-assh-backup

# 查看当前账号配置
assh sync account show

# 测试账号连通性
assh sync account test

# 删除账号配置
assh sync account delete
```

### 9.2 同步数据

```bash
# 推送本地配置到云端
assh sync push

# 从云端拉取配置到本地
assh sync pull

# 查看同步历史
assh sync history
```

### 9.3 同步策略

- **Push** — 将本地 SQLite 数据库（含所有服务器配置）加密后上传到七牛云
- **Pull** — 从七牛云下载最新配置，替换本地数据库
- **数据加密** — 传输全程使用加密，云端存储的是加密后的数据
- **冲突处理** — Pull 操作会完整替换本地数据，建议先 Push 备份本地数据

### 9.4 云同步示例集合

```bash
# === 常见场景 ===

# 场景：首次配置同步
assh sync account set --access-key AKxxx --secret-key SKxxx --bucket assh-backup
assh sync push
# 云端的 my-assh-backup 现在包含加密后的服务器配置

# 场景：在新电脑上恢复配置
assh sync account set   # 配置相同的云账号
assh sync pull          # 下载所有服务器配置
assh ls          # 确认配置已恢复

# 场景：修改后推送更新
assh add new-server -H 10.0.0.5 -u root
assh sync push          # 将新增的服务器同步到云端

# 场景：查看同步记录
assh sync history
```

---

## 10. 健康检查

ASSH 支持通过 SSH 连接对服务器进行健康检查，采集关键系统指标。

### 10.1 检查指标

健康检查采集以下系统信息：

| 指标 | 命令 | 说明 |
|------|------|------|
| 连接延迟 | SSH 连接时间 | 从发起连接到建立的时间 |
| 系统运行时长 | `uptime -p` | 系统已运行时间 |
| CPU 负载 | `cat /proc/loadavg` | 1/5/15 分钟平均负载 |
| 内存使用 | `free -h` | 总内存/已用/可用 |
| 磁盘使用 | `df -h /` | 根分区使用情况 |
| **健康状态** | 综合判定 | healthy / unhealthy / timeout / error |

> 单个指标采集失败**不会**阻断整体检查，静默跳过后继续采集其他指标。
> 所有指标均失败时标记为 unhealthy。

### 10.2 检查单台服务器

```bash
# 按名称检查
assh health check myserver

# 直接连接检查
assh health check root@192.168.1.100

# 多台服务器（逗号分隔）
assh health check web-01,web-02,db-01
```

**单机详细输出示例**：

```
Server:     web-01 (192.168.1.10:22)
Status:     ✅ healthy
Latency:    45.2ms
Uptime:     up 30 days, 2 hours, 15 minutes
Load Avg:   0.08 0.03 0.01
Memory:     total 7.6Gi, used 3.2Gi, free 4.4Gi
Disk:       total 98G, used 45G, free 53G (48% used)
```

### 10.3 批量检查

```bash
# 检查所有服务器
assh health list

# 按分组检查
assh health list --group prod

# 指定并发数（默认 5）
assh health list --concurrency 10

# 查看详细信息
assh health list --detail

# 组合使用
assh health list --group web --detail --concurrency 5
```

**批量表格输出示例**：

```
Server      | Status     | Latency | Uptime         | Load        | Memory         | Disk
------------|------------|---------|----------------|-------------|----------------|---------
web-01      | ✅ healthy | 45.2ms  | up 30 days     | 0.08 0.03   | 3.2G/7.6G      | 45G/98G
web-02      | ✅ healthy | 52.1ms  | up 15 days     | 0.15 0.10   | 2.1G/7.6G      | 30G/98G
db-01       | ⚠️ unhealthy| -      | -              | 30.5 28.2   | 6.8G/7.6G      | 85G/98G
old-server  | ❌ error   | -       | -              | -           | -              | -
office-proxy| ⏱ timeout | -       | -              | -           | -              | -

Summary: 2 healthy, 1 unhealthy, 1 error, 1 timeout
```

### 10.4 健康检查示例集合

```bash
# === 常见场景 ===

# 场景：日常巡检
assh health list --detail

# 场景：检查特定分组
assh health list --group production --concurrency 10

# 场景：快速检查单台服务器
assh health check web-01

# 场景：多台关键服务器检查
assh health check web-01,web-02,db-01,redis-01

# 场景：结合分组检查（运维值班快速巡检）
assh health list --group prod --detail | grep -E "unhealthy|error|timeout"

# 场景：远程服务器健康检查（直连）
assh health check root@10.0.0.1:2222
```

---

## 11. 配置参考

### 11.1 存储路径

ASSH v2 使用独立的 v2 路径，与 v1.x 隔离：

| 路径 | 说明 |
|------|------|
| `~/.assh/v2/` | 配置根目录 |
| `~/.assh/v2/asshv2.db` | SQLite 数据库（服务器配置 + AES-256 加密密钥） |
| `~/.assh/v2/assh.yml` | YAML 配置文件 |
| `~/.assh/v2/.rsa` | RSA 私钥（AES-256-GCM 加密的静态密钥） |
| `~/.assh/v2/.rsa.pub` | RSA 公钥 |
| `~/.assh/v2/.account` | 加密的密码文件 |
| `~/.assh/v2/data/keys/` | 密钥备份目录（按指纹命名子目录） |

### 11.2 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ASSH_CONFIG_DIR` | 配置目录路径 | `~/.assh/v2` |

### 11.3 全局命令行标志

| 标志 | 简写 | 说明 |
|------|:----:|------|
| `--verbose` | `-v` | 详细日志输出（DEBUG 级别） |
| `--quiet` | `-q` | 关闭日志输出 |
| `--config` | `-F` | 指定配置文件路径 |
| `--version` | `-V` | 打印版本信息 |

> **组合短参数**：支持组合使用，例如 `-qv` 等价于 `-q -v`。

### 11.4 日志级别

| 级别 | 值 | 说明 |
|------|:---:|------|
| OFF | 0 | 关闭日志 |
| FATAL | 100 | 致命错误 |
| PANIC | 150 | 严重错误 |
| ERROR | 200 | 错误 |
| WARN | 300 | 警告 |
| INFO | 400 | 信息 |
| DEBUG | 500 | 调试 |

```bash
# 调试模式（查看详细连接过程）
assh -v login myserver

# 静默模式（仅显示命令输出）
assh -q run myserver "uptime"

# 指定配置文件
assh -F ~/.assh/custom.yml login myserver
```

### 11.5 认证配置

服务器支持的认证方式及优先级：

| 优先级 | 认证方式 | 配置方式 |
|:------:|----------|----------|
| 1 | 备份密钥 | 自动（known_servers 表记录） |
| 2 | SSH Agent | 环境变量 `SSH_AUTH_SOCK` |
| 3 | 密钥文件 | `-i <path>` 或 `--key <path>` |
| 4 | 密码 | `-P <password>` 或交互输入 |
| 5 | 认证回退 | 密钥失败 → 自动尝试密码 |

### 11.6 SSH 选项

通过 `-o` 参数可以设置额外的 SSH 选项：

```bash
assh set myserver -o keepalive=30    # 心跳间隔（秒）
assh set myserver -o timeout=10      # 连接超时（秒）

# 启动代理时也支持
assh proxy myserver -o keepalive=15
```

### 11.7 直连模式

所有命令都支持直连模式，不依赖已保存的配置：

```bash
assh login -H <host> -u <user> [-p port] [-P password] [-i keyfile]
assh run -H <host> -u <user> "command"
assh-fs push -H <host> -u <user> ./local /remote/
assh proxy -H <host> -u <user> --socks5 :1080
assh health check <user>@<host>:<port>
assh keygen -H <host> -u <user> -P password -t ed25519
```

直连模式下，服务器信息不会保存到配置数据库，但认证信息会记录到 known_servers 表以便后续使用。

---

## 12. 命令速查表

### 12.1 assh 命令

| 命令 | 功能 | 常用参数 | 示例 |
|------|------|----------|------|
| `add` | 添加服务器 | `-H -u -P -p -i --remark` | `assh add myserver -H 10.0.0.1 -u root` |
| `set` | 修改服务器 | `-H -u -P -i -o --remark --clear-password --clear-key` | `assh set myserver -H new-host.com` |
| `ls` | 列出服务器 | `--group --search` | `assh ls --group prod` |
| `info` | 查看详情 | `<name>` | `assh info myserver` |
| `rm` | 删除服务器 | `<name>` | `assh rm myserver` |
| `mv` | 移动/重命名 | `<old> <new>` | `assh mv dev.web prod.web` |
| `rollback` | 回滚配置 | `--list` `<version>` | `assh rollback myserver 2` |
| `login` | SSH 登录 | `<name>` / `user@host` / `-H -u` | `assh login myserver` |
| `run` | 远程命令 | `<name> <command>` | `assh run myserver "uptime"` |
| `bc` | 批量执行 | `--servers --group --log` | `assh bc "df -h" --group prod` |
| `keygen` | 密钥管理 | `-t -b -f -C -N -H -u -P` | `assh keygen myserver -t ed25519` |
| `proxy` | 启动代理 | `--socks5 --http --reverse --autoproxy --daemon` | `assh proxy myserver --socks5 :1080` |
| `tunnel start` | 启动隧道 | `-L -R` | `assh tunnel start myserver -L 8080:web:80` |
| `tunnel stop` | 停止隧道 | `<id>` | `assh tunnel stop 1` |
| `tunnel list` | 列出隧道 | — | `assh tunnel list` |
| `proxy rule` | 规则管理 | `reload / status` | `assh proxy rule reload` |
| `proxy log` | 查看日志 | `<session-id>` | `assh proxy log abc123` |
| `sync account` | 云账号管理 | `set / show / delete / test` | `assh sync account set` |
| `sync push` | 推送到云端 | — | `assh sync push` |
| `sync pull` | 从云端拉取 | — | `assh sync pull` |
| `sync history` | 同步历史 | — | `assh sync history` |
| `health check` | 健康检查 | `<server>[,server2,...]` | `assh health check web-01` |
| `health list` | 批量检查 | `--group --concurrency --detail` | `assh health list --detail` |
| `version` | 版本信息 | — | `assh version` |

### 12.2 assh-fs 命令

| 命令 | 功能 | 常用参数 | 示例 |
|------|------|----------|------|
| `push` | 推送文件 | `-r -f --skip --resume --checksum -H -u -P` | `assh-fs push myserver ./file /remote/` |
| `pull` | 拉取文件 | `-r --resume --checksum -H -u -P` | `assh-fs pull myserver /remote/file ./` |
| `<server>` | 交互式 SFTP | 交互式命令 | `assh-fs myserver` |

### 12.3 快捷键一览

| 命令行模式 | 操作 |
|-----------|------|
| `assh <name>` | 直接登录（等价于 `assh login <name>`） |
| `-qv` | 组合短参数（等价的 `-q -v`） |

---

## 13. 常见问题与技巧

### 13.1 首次运行

**Q: 第一次运行 assh 需要做什么？**

A: 下载二进制后直接运行即可。首次运行会自动：
1. 创建 `~/.assh/v2/` 目录
2. 生成 AES-256-GCM 加密密钥
3. 初始化 SQLite 数据库
4. 生成初始 RSA 密钥对

不需要手动配置，添加服务器即可开始使用。

### 13.2 密码存储安全

**Q: 密码存储在本地安全吗？**

A: 密码使用 AES-256-GCM 加密存储在 SQLite 中。加密密钥在首次运行时自动生成，单独存储在同一数据库的 `config` 表中，并通过 Base64 编码持久化。理论上需要同时获取数据库文件和破解 AES-256 才能解密。

### 13.3 跨设备同步配置

**Q: 如何在多台电脑之间同步服务器配置？**

A: 使用云同步功能：
```bash
# 电脑 A：配置云账号并推送
assh sync account set --access-key AKxxx --secret-key SKxxx --bucket my-bucket
assh sync push

# 电脑 B：配置相同云账号并拉取
assh sync account set
assh sync pull
```

### 13.4 代理保持后台运行

**Q: 关闭终端后代理会停止吗？**

A: 使用 `--daemon` 参数可以让代理在后台持续运行：
```bash
assh proxy myserver --socks5 :1080 --daemon
```
关闭终端后代理继续运行。使用 `ps aux | grep assh` 查看进程，`kill` 停止。

### 13.5 端口转发场景

**Q: 如何访问内网数据库？**

A: 使用端口转发隧道：
```bash
# 将远程 3306 端口映射到本地
assh tunnel start jump-server -L 3306:rds.internal:3306
# 本地连接
mysql -h 127.0.0.1 -P 3306 -u admin -p
```

### 13.6 文件传输效率

**Q: 大文件传输中断了怎么办？**

A: 使用 `--resume`（或 `-e`）启用断点续传：
```bash
assh-fs push --resume myserver ./large-file.iso /remote/
```
传输中断后重新执行相同命令，会自动从断点处继续。

### 13.7 批量服务器管理

**Q: 如何对多台服务器执行相同命令？**

A: 使用 `bc` 命令：
```bash
# 指定服务器列表
assh bc "apt update && apt upgrade -y" --servers web-01,web-02,web-03

# 按分组
assh bc "systemctl restart nginx" --group prod

# 保存结果到文件
assh bc "date" --servers s1,s2 --log ./result.json
```

### 13.8 常用组合技巧

```bash
# 快速查看所有服务器的磁盘使用
assh bc "df -h / | tail -1" --group prod

# 健康检查 + 问题定位
assh health check web-01
assh run web-01 "free -m && df -h && uptime"

# 部署 + 重启 + 检查
assh-fs push web-01 ./app.jar /opt/app/
assh run web-01 "systemctl restart myapp && systemctl status myapp"

# 备份配置到云端
assh sync push

# 多服务器并发健康检查
assh health list --concurrency 20

# 代理调试（详细日志）
assh -v proxy myserver --socks5 :1080 --log-dir ./logs
```

### 13.9 故障排查

**Q: 连接失败怎么办？**

```bash
# 1. 使用 -v 开启调试日志，查看连接过程
assh -v login myserver

# 2. 测试网络连通性
assh run myserver "echo ok" 

# 3. 检查服务器配置
assh info myserver

# 4. 尝试直连
assh login -H <host> -u <user> -P <password>

# 5. 如果是密钥问题，重新生成并部署
assh keygen myserver -t ed25519
```

**Q: `assh-fs: command not found`？**

A: `assh-fs` 是独立的二进制文件，需要确保：
1. 构建时同时生成了 `assh-fs`（`build.sh` 自动构建两个二进制）
2. 两个二进制都在 `PATH` 中

### 13.10 最佳实践总结

| 实践 | 说明 |
|------|------|
| 使用 Ed25519 密钥 | 推荐 `assh keygen myserver -t ed25519`，比 RSA 更快更安全 |
| 定期健康检查 | 使用 `assh health list --concurrency 10` 快速巡检 |
| 配置云端备份 | 配置七牛云后定期 `assh sync push` |
| 使用分组管理 | 按用途分组：`prod.web`、`staging.api`、`dev.db` |
| 善用直连模式 | 临时连接不需要保存到配置 |
| 批量操作用 bc | 多服务器并发执行，效率远高于逐个登录 |
| 大文件用断点续传 | `--resume` 参数避免重传 |
| 代理加规则引擎 | `--autoproxy` 智能分流，不必全局走代理 |
| 启动时用 -v 调试 | 第一次连接遇到问题时加 `-v` 查看详情 |
| 回滚是安全网 | 修改配置后如果出错，`rollback` 一键恢复 |



