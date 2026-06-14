# RQ-004: AI 安全检查 (X1)

> 状态：✅ 已确认
> 创建日期：2026-06-14
> 优先级：实验性
> 版本：v2.2.0-preview.1

---

## 1. 概述

### 1.1 核心概念

**AI 安全检查**是通过 AI 大模型自动生成漏洞检测脚本的功能。用户提供漏洞信息（CVE 编号或自由文本描述），AI 自动生成检测脚本，经安全审查后在目标服务器执行，输出检测报告。

### 1.2 使用场景

- 快速检测已知漏洞（CVE）
- 自定义漏洞检测
- 安全审计和合规检查
- 自动化安全扫描

### 1.3 设计原则

- **智能化**：AI 自动生成检测脚本
- **安全性**：双重审查机制，防止危险操作
- **可扩展**：支持多种 AI 服务和安全规则
- **易用性**：简单的命令行接口

---

## 2. 功能规格

### 2.1 核心流程

```
用户输入 (CVE/文本)
    │
    ▼
┌─────────────────────┐
│ 安全审查（输入过滤）│  ← 长度/格式/防注入
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ CVE 自动查询 (NVD)  │  ← 仅 CVE 编号时触发
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ AI 生成检测脚本     │  ← OpenAI 兼容 API
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ 脚本安全审查        │  ← 双重审查
│  ├─ 自动规则过滤    │
│  └─ 用户确认        │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ SSH 远程执行        │  ← 目标服务器执行
│ 本地脚本保存        │  ← ~/.assh/security/scripts/
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ 结果输出            │
│  ├─ 终端显示        │
│  └─ Markdown 报告   │
└─────────────────────┘
```

### 2.2 输入方式

#### CVE 编号输入

```bash
# 按 CVE 编号检查
assh security vuln-check -s myserver "CVE-2024-3094"

# 批量检查
assh security vuln-check -s server1,server2 "CVE-2024-3094"

# 按分组检查
assh security vuln-check -g prod "CVE-2024-3094"
```

**CVE 格式校验**：
- 格式：`CVE-YYYY-NNNNNN`
- 自动从 NVD NIST API 查询详情
- 查询失败时使用用户输入的文本

#### 自由文本输入

```bash
# 按漏洞描述检查
assh security vuln-check -s myserver "检查是否存在未修补的 Log4j 漏洞"

# 自定义检测
assh security vuln-check -s myserver "检查 SSH 版本是否低于 8.0"
```

### 2.3 AI 配置

#### 配置文件

**位置**：`~/.assh/v2/ai.yml`

**格式**：
```yaml
# ~/.assh/v2/ai.yml
ai:
  # OpenAI 兼容 API 配置
  endpoint: "https://api.openai.com/v1"
  model: "gpt-4"
  api_key: "sk-xxx"
  max_tokens: 4096
  temperature: 0.3

  # 可选：多个 AI 服务配置
  services:
    openai:
      endpoint: "https://api.openai.com/v1"
      model: "gpt-4"
      api_key: "sk-xxx"
    deepseek:
      endpoint: "https://api.deepseek.com/v1"
      model: "deepseek-chat"
      api_key: "sk-xxx"
    local:
      endpoint: "http://localhost:11434/v1"
      model: "llama2"
      api_key: ""

  # 默认服务
  default_service: "openai"
```

#### 配置字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| `endpoint` | string | ✅ | API 地址 |
| `model` | string | ✅ | 模型名称 |
| `api_key` | string | ✅ | API 密钥 |
| `max_tokens` | int | ❌ | 最大 Token 数（默认 4096） |
| `temperature` | float | ❌ | 温度参数（默认 0.3） |

### 2.4 安全审查

#### 第一层 — 自动规则过滤

**内置规则**：
```yaml
rules:
  # 危险命令
  - pattern: "rm\\s+-rf\\s+/"         # 禁止删除根目录
  - pattern: "mkfs|dd\\s+if=|format"   # 禁止格式化操作
  - pattern: "chmod\\s+777"            # 禁止 777 权限
  - pattern: ">/dev/(sda|sdb|nvme)"    # 禁止直接写块设备
  - pattern: "wget|curl.*\\|\\s*sh"    # 禁止管道执行下载内容
  - pattern: ":(){ :\\|:& };:"         # 禁止 fork 炸弹
  - pattern: "iptables.*-F"            # 禁止清空防火墙规则
  - pattern: "passwd"                  # 禁止修改密码

  # 高风险操作
  - pattern: "systemctl\\s+(stop|restart|disable)"  # 服务操作
  - pattern: "useradd|userdel|groupadd"             # 用户管理
  - pattern: "crontab\\s+-e"                        # 定时任务
  - pattern: "mount|umount"                         # 挂载操作
```

**自定义规则**：
```yaml
# ~/.assh/v2/security-rules.yml
custom_rules:
  - name: "禁止访问敏感目录"
    pattern: "/etc/shadow|/etc/passwd"
    risk_level: "high"
    message: "禁止访问系统敏感文件"

  - name: "限制网络操作"
    pattern: "nc\\s+-l|ncat|socat"
    risk_level: "medium"
    message: "网络监听操作需要确认"
```

#### 第二层 — 用户确认

**高风险操作确认**：
```
[WARNING] @security: 检测到高风险操作：
  - 操作：systemctl restart nginx
  - 风险级别：medium
  - 匹配规则：服务操作

是否继续执行？[y/N]:
```

**确认选项**：
- `y` / `yes`：确认执行
- `n` / `no`：取消执行
- `a` / `all`：本次检查全部确认

### 2.5 CVE 查询

#### NVD NIST API

**API 地址**：`https://services.nvd.nist.gov/rest/json/cves/2.0/`

**查询参数**：
```
?cveId=CVE-2024-3094
```

**返回数据**：
```json
{
  "vulnerabilities": [
    {
      "cve": {
        "id": "CVE-2024-3094",
        "descriptions": [
          {
            "lang": "en",
            "value": "Malicious code in xz/liblzma..."
          }
        ],
        "metrics": {
          "cvssMetricV31": [
            {
              "cvssData": {
                "baseScore": 10.0,
                "baseSeverity": "CRITICAL"
              }
            }
          ]
        }
      }
    }
  ]
}
```

**查询逻辑**：
1. 检查 CVE 编号格式
2. 调用 NVD API 查询
3. 解析返回数据
4. 提取描述和评分
5. 传递给 AI 生成脚本

### 2.6 脚本生成

#### AI 提示词

**系统提示词**：
```
你是一个安全检测专家。根据用户提供的漏洞信息，生成一个 bash 检测脚本。

要求：
1. 脚本必须是可执行的 bash 脚本
2. 包含注释说明检测目标
3. 检查逻辑清晰
4. 结果输出符合统一格式
5. 退出码约定：0=无漏洞, 1=存在漏洞, 2=检测失败

输出格式：
- 只输出脚本内容，不要包含其他说明
- 脚本必须以 #!/bin/bash 开头
- 包含检测结果输出
```

**用户提示词**：
```
漏洞信息：{vuln_info}
目标系统：{target_os}
检测范围：{scope}
```

#### 脚本模板

```bash
#!/bin/bash
# AI 安全检测脚本
# 漏洞：{vuln_id}
# 生成时间：{timestamp}
# 目标：{target}

set -e

# 检测函数
check_vulnerability() {
    # 检查逻辑
    if [条件]; then
        echo "[VULNERABLE] 发现漏洞：{vuln_id}"
        echo "详情：{details}"
        echo "建议：{suggestion}"
        return 1
    else
        echo "[SAFE] 未发现漏洞：{vuln_id}"
        return 0
    fi
}

# 主流程
main() {
    echo "开始检测：{vuln_id}"
    echo "目标：{target}"
    echo "时间：$(date)"

    check_vulnerability
    result=$?

    echo "检测完成"
    exit $result
}

main "$@"
```

### 2.7 输出格式

#### 终端输出

```
┌─ AI 安全检查 ─────────────────────┐
│ 漏洞: CVE-2024-3094               │
│ 目标: myserver (192.168.1.100)    │
│ 状态: ⚠️ 存在漏洞                  │
│ 详情: xz/liblzma 5.6.0/5.6.1     │
│ 建议: 升级至 5.6.2+                │
│ 报告: ~/.assh/security/...report  │
└────────────────────────────────────┘
```

#### Markdown 报告

```markdown
# AI 安全检查报告

- **漏洞**: CVE-2024-3094
- **目标**: myserver (192.168.1.100)
- **时间**: 2026-06-14 14:30:00
- **脚本**: script.sh

## 检测结果
- 状态: ⚠️ 存在漏洞
- 详情: 发现 xz/liblzma 5.6.0

## 执行日志
```
[14:30:01] 连接成功
[14:30:02] 执行检测脚本...
[14:30:05] 检测完成
```

## 建议
升级 xz 至 5.6.2+
```

### 2.8 脚本存储

#### 存储结构

```
~/.assh/security/scripts/
├── CVE-2024-3094/
│   ├── script.sh              # 生成的检测脚本
│   ├── result.log             # 执行日志
│   └── report.md              # Markdown 检测报告
├── log4j-check/
│   ├── script.sh
│   ├── result.log
│   └── report.md
└── index.json                 # 索引文件（可选）
```

#### 索引文件

```json
{
  "checks": [
    {
      "id": "CVE-2024-3094",
      "target": "myserver",
      "timestamp": "2026-06-14T14:30:00Z",
      "status": "vulnerable",
      "script": "CVE-2024-3094/script.sh",
      "report": "CVE-2024-3094/report.md"
    }
  ]
}
```

---

## 3. 命令规格

### 3.1 命令语法

```bash
assh security vuln-check [选项] <漏洞信息>
```

### 3.2 选项参数

| 选项 | 缩写 | 说明 | 必填 |
|------|------|------|:----:|
| `--server` | `-s` | 目标服务器（逗号分隔） | ✅ |
| `--group` | `-g` | 目标分组 | ❌ |
| `--output` | `-o` | 输出格式（text/json/markdown） | ❌ |
| `--save` | | 保存脚本（默认 true） | ❌ |
| `--execute` | | 执行脚本（默认 true） | ❌ |
| `--confirm` | | 高风险操作确认（默认 true） | ❌ |
| `--timeout` | | 执行超时（默认 300s） | ❌ |

### 3.3 使用示例

```bash
# 按 CVE 编号检查单台服务器
assh security vuln-check -s myserver "CVE-2024-3094"

# 按 CVE 编号检查多台服务器
assh security vuln-check -s server1,server2 "CVE-2024-3094"

# 按分组检查
assh security vuln-check -g prod "CVE-2024-3094"

# 按漏洞描述检查
assh security vuln-check -s myserver "检查是否存在未修补的 Log4j 漏洞"

# 只生成脚本不执行
assh security vuln-check -s myserver --no-execute "CVE-2024-3094"

# 输出 JSON 格式
assh security vuln-check -s myserver -o json "CVE-2024-3094"
```

---

## 4. 技术规格

### 4.1 架构设计

```
cmd/security.go
    │
    ▼
asshc/service/vuln.go (VulnService)
    │
    ├── port/ai.go (AIProvider)
    │       │
    │       └── infra/ai/openai.go (OpenAI 客户端)
    │
    ├── port/vuln.go (VulnerabilityChecker)
    │       │
    │       ├── infra/cve/nvd.go (NVD 查询)
    │       └── infra/security/scanner.go (安全审查)
    │
    └── port/connector.go (SSHConnector)
            │
            └── infra/ssh/ (SSH 连接)
```

### 4.2 模块划分

```
asshc/
├── domain/
│   └── vuln.go            # 漏洞检查实体
├── port/
│   ├── ai.go              # AIProvider 接口
│   └── vuln.go            # VulnerabilityChecker 接口
├── service/
│   └── vuln.go            # VulnService 实现
└── infra/
    ├── ai/
    │   └── openai.go      # OpenAI 客户端
    ├── cve/
    │   └── nvd.go         # NVD 查询
    └── security/
        └── scanner.go     # 安全审查引擎
```

### 4.3 接口定义

```go
// port/ai.go
type AIProvider interface {
    Generate(ctx context.Context, prompt string) (string, error)
    GenerateWithSystem(ctx context.Context, system, user string) (string, error)
}

// port/vuln.go
type VulnerabilityChecker interface {
    Check(ctx context.Context, server *domain.Server, vulnInfo string) (*domain.VulnReport, error)
    GenerateScript(ctx context.Context, vulnInfo, cveDetail string) (string, error)
    ValidateScript(script string) (*ScriptValidation, error)
    ExecuteScript(ctx context.Context, server *domain.Server, script string) (*domain.ExecResult, error)
}
```

---

## 5. 验收标准

### 5.1 功能验收

```bash
# 1. AI 配置
assh security vuln-check --help
# 预期：显示帮助信息

# 2. CVE 检查
assh security vuln-check -s myserver "CVE-2024-3094"
# 预期：生成检测脚本并执行

# 3. 自由文本检查
assh security vuln-check -s myserver "检查 Log4j 漏洞"
# 预期：生成检测脚本并执行

# 4. 安全审查
assh security vuln-check -s myserver "rm -rf /"
# 预期：被安全审查阻止

# 5. 报告输出
ls ~/.assh/security/scripts/
# 预期：生成脚本和报告文件
```

### 5.2 配置验收

```bash
# 1. 创建 AI 配置
cat > ~/.assh/v2/ai.yml << EOF
ai:
  endpoint: "https://api.openai.com/v1"
  model: "gpt-4"
  api_key: "sk-xxx"
EOF

# 2. 测试 AI 服务
assh security vuln-check -s myserver "CVE-2024-3094"
# 预期：正常调用 AI 服务

# 3. 测试配置切换
assh security vuln-check -s myserver --service deepseek "CVE-2024-3094"
# 预期：使用 deepseek 服务
```

### 5.3 安全验收

```bash
# 1. 测试危险命令过滤
assh security vuln-check -s myserver "rm -rf /"
# 预期：被安全审查阻止

# 2. 测试高风险操作确认
assh security vuln-check -s myserver "systemctl restart nginx"
# 预期：弹出确认提示

# 3. 测试自定义规则
assh security vuln-check -s myserver "访问 /etc/shadow"
# 预期：被自定义规则阻止
```

---

## 6. 未来扩展

### 6.1 可能的增强

- 支持更多 AI 服务（Claude、Gemini、本地模型）
- 批量漏洞检查
- 漏洞库集成（CVE、NVD、CNVD）
- 定时扫描任务
- 漏洞修复建议
- 集成 CI/CD 流程

### 6.2 与其他功能集成

- 与 Phase 9（健康检查）集成：安全检查作为健康检查的一部分
- 与 Phase 13（命令执行增强）集成：超时、输出格式化
- 与 NEXT-001（代理/隧道）集成：通过代理访问 AI 服务

---

## 7. 相关文档

- [X1 设计文档](../worklogs/2026-05-17-PhaseX-AI-Security-Design.md)
- [X2 需求文档](RQ-003-virtual-commands.md)
- [版本规划](../v2.2.0-preview.1-plan.md)

---

## 8. 变更记录

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2026-06-14 | 1.0 | 初始需求确认 |
