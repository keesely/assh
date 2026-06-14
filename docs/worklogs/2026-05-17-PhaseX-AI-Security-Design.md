# Phase X: AI 安全检查 — 需求设计方案

**日期**: 2026-05-17
**版本**: v1.0
**状态**: 需求已确认，待实现

## 1. 背景与目标

### 1.1 为什么需要 AI 安全检查

传统的漏洞扫描需要手动编写检测脚本，效率低且容易遗漏。通过 AI 大模型，可以根据漏洞描述自动生成检测脚本，大幅提升安全检测的效率和覆盖范围。

### 1.2 核心目标

用户提供漏洞信息（CVE 编号或自由文本描述）→ AI 自动生成检测脚本 → 经安全审查后在目标服务器执行 → 输出报告。

## 2. 需求总览

### 2.1 AI 配置
- **配置位置**: 合入现有配置系统（`config/`）
- **配置字段**:
  - `ai.endpoint` — OpenAI 兼容 API 地址
  - `ai.model` — 模型名称（如 gpt-4, deepseek-chat 等）
  - `ai.api_key` — API 密钥
  - `ai.max_tokens` — 最大 Token 数（默认 4096）
  - `ai.temperature` — 温度参数（默认 0.3，偏低以保持一致性）

### 2.2 输入处理
- **输入方式**: CVE 编号 + 自由文本描述，重点支持 CVE 编号
- **安全审查**:
  - 对用户输入进行安全过滤（长度限制、特殊字符过滤、防 prompt injection）
  - CVE 编号格式校验（CVE-YYYY-NNNNNN）
- **CVE 自动查询**: 从 [NVD NIST 公共 API](https://services.nvd.nist.gov/rest/json/cves/2.0/) 自动获取 CVE 详情，将描述信息传给 AI

### 2.3 AI 脚本生成
- AI 接收系统提示词（System Prompt）约束生成行为
- 输出格式要求：生成可执行的 Shell 检测脚本（bash）
- 脚本需包含：
  - 注释说明检测目标
  - 检查逻辑
  - 结果输出（符合统一格式，便于解析）
  - 退出码约定（0=无漏洞, 1=存在漏洞, 2=检测失败）

### 2.4 安全双重审查

#### 第一层 — 自动规则过滤
内置安全规则列表，阻止以下危险模式：
```yaml
rules:
  - pattern: "rm\\s+-rf\\s+/"         # 禁止删除根目录
  - pattern: "mkfs|dd\\s+if=|format"   # 禁止格式化操作
  - pattern: "chmod\\s+777"            # 禁止 777 权限
  - pattern: ">/dev/(sda|sdb|nvme)"    # 禁止直接写块设备
  - pattern: "wget|curl.*\\|\\s*sh"    # 禁止管道执行下载内容
  - pattern: ":(){ :\\|:& };:"         # 禁止 fork 炸弹
  - pattern: "iptables.*-F"            # 禁止清空防火墙规则
  - pattern: "passwd"                  # 禁止修改密码
```

#### 第二层 — 用户确认
- 自动审查通过后，向用户展示脚本摘要和检测范围
- 高风险操作（写文件、修改配置等）弹出确认提示
- 用户可选择：确认执行 / 取消（优雅终止）

### 2.5 执行方式
- **SSH 远程执行**: 登录目标服务器，上传临时脚本，执行并收集输出
- **本地保存**: 脚本同时保存到 `~/.assh/security/scripts/`

### 2.6 存储结构
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
- **时间**: 2026-05-17 14:30:00
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

## 3. 架构设计

### 3.1 分层结构

```
asshc/
├── port/
│   ├── ai.go              — AIProvider 接口
│   └── vuln.go            — VulnerabilityChecker 接口
├── domain/
│   └── vuln.go            — 漏洞检查相关实体
├── infra/
│   ├── ai/
│   │   └── openai.go      — OpenAI 兼容 API 调用
│   ├── cve/
│   │   └── nvd.go         — NVD NIST CVE 查询
│   └── security/
│       └── scanner.go     — 脚本安全审查引擎
├── service/
│   └── vuln.go            — VulnerabilityService
config/
└── (合入现有配置系统)
cmd/
└── security.go            — security vuln-check 命令
```

### 3.2 核心接口定义

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

type ScriptValidation struct {
    Safe      bool
    RiskLevel RiskLevel  // low / medium / high
    Warnings  []string
    MatchedRules []string
}

type RiskLevel int
const (
    RiskLow    RiskLevel = iota
    RiskMedium
    RiskHigh
)
```

### 3.3 工作流程

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

## 4. 任务分解

| 编号 | 任务 | 文件 | 预估工时 |
|------|------|------|:--------:|
| X.1 | AI 配置（合入 config） | `config/` | 0.5d |
| X.2 | AIProvider 接口定义 | `asshc/port/ai.go` | 0.5d |
| X.3 | OpenAI 客户端实现 | `asshc/infra/ai/openai.go` | 1d |
| X.4 | NVD CVE 查询实现 | `asshc/infra/cve/nvd.go` | 1d |
| X.5 | 漏洞领域实体定义 | `asshc/domain/vuln.go` | 0.5d |
| X.6 | 安全审查引擎 | `asshc/infra/security/scanner.go` | 1.5d |
| X.7 | VulnerabilityChecker 接口 | `asshc/port/vuln.go` | 0.5d |
| X.8 | VulnerabilityService 服务 | `asshc/service/vuln.go` | 2d |
| X.9 | security CLI 命令 | `cmd/security.go` | 1d |
| X.10 | 脚本/日志存储系统 | `~/.assh/security/scripts/` | 0.5d |
| **合计** | | | **~9d** |

## 5. 风险与注意事项

### 5.1 安全风险
- **AI 生成恶意脚本**: 即使有双重审查，仍需警惕对抗性提示注入
- **SSH 凭据泄露**: 执行过程中需确保凭据安全
- **敏感信息暴露**: 生成的脚本和日志可能包含服务器敏感信息，存储时需注意权限控制

### 5.2 技术风险
- **AI API 可用性**: 依赖外部 AI 服务，需处理超时和失败降级
- **CVE API 限制**: NVD API 有速率限制，需实现缓存
- **脚本兼容性**: AI 生成的脚本可能在不同系统（Ubuntu/CentOS/Alpine）上行为不一致

### 5.3 降级策略
- AI 服务不可用 → 提示用户稍后重试，建议本地手动编写脚本
- CVE 查询失败 → 仅使用用户输入的文本描述传递给 AI
- 脚本执行失败 → 详细记录错误，保存脚本供人工审查

## 6. 相关参考

- [OpenAI API 文档](https://platform.openai.com/docs/api-reference/chat)
- [NVD NIST API 2.0](https://nvd.nist.gov/developers/rest-api)
- [OWASP Prompt Injection Guide](https://owasp.org/www-community/attacks/Prompt_Injection)
