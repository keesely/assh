# 认证回退 & v1 导入设计

> 2026-05-11 | PM Agent

## 任务 1: SSH 认证回退 (Bugfix)

### 现象

```
$ ./assh ali.24
error: ssh dial failed: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain
```

当服务器同时配置了 password 和 key 时，key 认证失败后 password 未被尝试。

### 根因分析

当前 `authMethods()` 将 KeyFile 和 Password 都加入 `[]ssh.AuthMethod`，Go 的 `crypto/ssh` 库顺序尝试。但以下情况会导致 password 不被尝试：

1. **服务器端限制**：SSH 服务器在 key 认证失败后返回 `allowed methods: []`（空列表），即服务器不再允许 password 方式
2. **Go SSH 行为**：Go 库仅尝试服务器允许的方法，若服务器不返回 password，则 password 方法被跳过

用户期望的行为：**客户端主动尝试 key → 失败 → 主动用 password 重试**，即使服务器不推荐 password。

### 已实施修复

**文件**: `asshc/infra/ssh/client.go`

修改 `Connect()` 方法，将认证拆分为两轮：

```
┌─ 第一轮（主认证）───┐
│ agent + key + pwd  │──成功──→ 返回连接
└───────┬──失败──────┘
        │ 如果服务器同时配置了 key 和 pwd
        ▼
┌─ 第二轮（回退）────┐
│ password only      │──成功──→ 返回连接
└───────┬──失败──────┘
        ▼
  返回原始错误
```

关键改动：
- `Connect()` 首轮使用全部方法；若失败且存在 key+pwd 同时配置，发起第二轮 Password-only 连接
- 提取 `setupKeepalive()` 公共方法
- 添加 `hasKeyAndPassword()` 辅助判断
- 包注释更新认证策略描述

### 影响范围

- 仅修改 `asshc/infra/ssh/client.go`
- 新增连接延迟：当首轮 key 认证失败时，多一次 TCP 握手 + SSH 握手
- 对仅有 key 或仅有 pwd 的服务器无影响（不回退）

---

## 任务 2: v1 → v2 配置导入

### v1 数据格式全分析

#### 目录结构

```
~/.assh/
├── .rsa                        # RSA 私钥 (PKCS#1 PEM, 2048-bit)
├── .rsa.pub                    # RSA 公钥
├── .account                    # RSA PKCS1v15 加密的密码 MD5 (256 bytes)
├── assh.yml                    # RSA 加密的配置文件 (256 bytes)
└── data/
    ├── servers.db              # AES-CBC 加密的 JSON 数据 (6576 bytes)
    ├── ali.24/                 # 服务器目录（分组.名称）
    │   ├── id_rsa              # SSH 私钥
    │   └── id_rsa.pub          # SSH 公钥
    ├── home/
    │   ├── id_rsa
    │   ├── _assh_pemkey_home   # 自定义密钥（非标准名称）
    │   └── ...
    └── ...
```

#### 加密链路

```
[明文 JSON] 
    ↓ kiris.AESEncrypt(data, passwd, "cbc") 
    ↓ key=[]byte(passwd_md5_hex), IV=key[:16], AES-256-CBC+PKCS7
[AES 密文]
    ↓ 拼接: MD5(passwd+data) + MD5(data) + data
[servers.db 文件]

[密码明文]
    ↓ rsa.DecryptPKCS1v15(.account, .rsa_privkey)
[MD5 hex string] ← 用作 AES key
```

#### JSON 数据结构

```json
{
  "group_name": {
    "server_name": {
      "name":     "group.server_name",
      "host":     "hostname or IP",
      "port":     22,
      "user":     "root",
      "password": "plaintext_password",
      "key":      "/path/to/key_file",
      "remark":   "备注",
      "options":  null
    }
  }
}
```

顶级 key 为分组名，二级 key 为服务器名。value 的 `name` 字段为完整名（group.server）。

#### 密码存储

- `.account` 解密后获取 MD5 hex string（32 字符）
- servers.db 内密码为 **明文存储**（通过 AES-CBC 保护静止数据）
- `assh.yml` 使用 RSA PKCS1v15 加密，存储全局配置（logPath, dbPath, qiniu 配置）

#### 密钥文件

存储在 `~/.assh/data/{group}/{server}/id_rsa` 对应 v1 Server 的 `key` 字段。
文件名：
- 标准名称：`id_rsa` / `id_rsa.pub`
- 自定义名称：`_assh_pemkey_{server}` / 文件哈希值

### 导入设计

#### 架构

```
cmd/import.go                   # CLI 入口（assh import v1）
    │
    ▼
asshc/service/importer.go       # V1Importer 业务逻辑
    │
    ├── 1. 读取 V1 RSA 私钥     (~/.assh/.rsa)
    ├── 2. 解密 .account         (RSA PKCS1v15)
    ├── 3. 校验+解密 servers.db  (AES-CBC)
    ├── 4. 解析 JSON → []domain.Server
    ├── 5. 扫描密钥目录收集密钥文件路径
    ├── 6. 显示待导入列表 + 确认
    └── 7. 通过 Store API 写入 v2 SQLite
```

#### 命令行设计

```
assh import v1                    # 交互式：显示发现列表 → 确认导入
assh import v1 --dry-run          # 仅预览，不导入
assh import v1 --force            # 覆盖已存在的同名服务器
```

#### v1 → v2 字段映射

| v1 字段 | v2 字段 | 转换逻辑 |
|---------|---------|----------|
| `name` | `Name` | 保持 group.server 格式 |
| `host` | `Host` | 直接映射 |
| `port` | `Port` | 直接映射 |
| `user` | `User` | 默认 "root" |
| `password` | `Auth.Password` | 明文 → AES-GCM 加密存储 |
| `key` | `Auth.KeyFile` | 绝对路径，导入时复制到 v2 目录 |
| `remark` | `Remark` | 直接映射 |
| — | `Version` | 初始化为 1 |
| — | `Options` | 空 map |

#### 密钥文件处理

- 读取 v1 key 文件（如 `~/.assh/data/ali.24/id_rsa`）
- 复制到 v2 密钥目录（`~/.assh/v2/credentials/`）
- 更新 `Auth.KeyFile` 指向新路径

#### 并发与安全

- servers.db 解密使用内存安全处理（避免敏感数据残留）
- 导入完成后提示用户清理 v1 数据
- 导入操作应在交互式确认后进行，避免意外覆盖

### ADR

- **ADR-024**: v1→v2 配置导入指令（已创建，accepted）
- 文件: `docs/adrs/024-v1-to-v2-migration.md`

---

## 实现状态

| 任务 | 状态 |
|------|------|
| SSH 认证回退修复 | 代码已修改（client.go），待构建验证 |
| ADR-024 创建 | 已完成 |
| v1 数据格式分析 | 已完成，已验证解密成功 |
| v1 import: 核心解密逻辑 | 待实现 |
| v1 import: CLI 命令 | 待实现 |
| 文档更新 (TODO.md, AGENTS.md) | 待完成 |

## Git 日志记录

```
(当前分支: dev/v2.0.0-phase-5)
已提交: 931bef3 feat: implement F14 glob push and F20 glob pull
已提交: 799dc44 docs: mark F14/F20 glob as implemented
未提交: asshc/infra/ssh/client.go (认证回退修复)
未提交: docs/adrs/024-v1-to-v2-migration.md (ADR)
未提交: docs/adrs.md (ADR 索引更新)
```
