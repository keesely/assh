# 2026-06-14 X2 虚拟命令开发进度

## 工作概述

**日期**: 2026-06-14
**任务**: X2 虚拟命令基础框架实现
**状态**: 🚧 开发中
**版本**: v2.2.0-preview.1

---

## 已完成工作

### 1. 基础框架实现

#### X2.1 虚拟命令实体 ✅
- **文件**: `asshc/domain/virtual.go`
- **内容**:
  - VirtualCommand 结构体定义
  - VirtualCommandConfig 配置结构
  - VirtualCommandResult 结果结构
  - IsVirtualCommand() 和 ParseVirtualCommand() 工具函数
  - 虚拟命令错误类型定义

#### X2.2 命令处理器接口 ✅
- **文件**: `asshc/port/virtual.go`
- **内容**:
  - VirtualCommandHandler 接口定义
  - VirtualCommandService 接口定义
  - VirtualCommandConfigLoader 接口定义

#### X2.3 命令服务实现 ✅
- **文件**: `asshc/service/virtual.go`
- **内容**:
  - VirtualCommandServiceImpl 实现
  - 命令注册和查找
  - 命令执行和超时控制
  - 配置管理

#### X2.4 配置加载器 ✅
- **文件**: `asshc/infra/virtual/config.go`
- **内容**:
  - ConfigLoader 实现
  - YAML 配置文件加载
  - 默认配置生成
  - 配置合并和保存

### 2. 内置命令实现

#### X2.5 @push 命令 ✅
- **文件**: `asshc/infra/virtual/push.go`
- **内容**:
  - PushCommandHandler 实现
  - 参数解析（-r, local, remote）
  - 文件存在性检查
  - 占位符实现（待集成 SFTP）

#### X2.6 @pull 命令 ✅
- **文件**: `asshc/infra/virtual/pull.go`
- **内容**:
  - PullCommandHandler 实现
  - 参数解析（-r, remote, local）
  - 占位符实现（待集成 SFTP）

#### X2.9 @help 命令 ✅
- **文件**: `asshc/infra/virtual/help.go`
- **内容**:
  - HelpCommandHandler 实现
  - 显示所有命令列表
  - 显示指定命令帮助

#### X2.10 输入拦截器 ✅
- **文件**: `asshc/infra/virtual/handler.go`
- **内容**:
  - InputHandler 实现
  - 虚拟命令识别和执行
  - 结果显示
  - 命令提示（自动补全）

### 3. 辅助命令实现

#### @version 命令 ✅
- **文件**: `asshc/infra/virtual/version.go`
- **内容**: 显示 assh 版本信息

#### @status 命令 ✅
- **文件**: `asshc/infra/virtual/status.go`
- **内容**: 显示连接状态

---

## 未完成工作

### 1. 待实现命令

| 编号 | 任务 | 状态 | 说明 |
|------|------|------|------|
| X2.7 | @run 实现 | ⏳ | 本地命令执行 |
| X2.8 | @deploy 实现 | ⏳ | 上传 + 执行 |
| X2.11 | 自定义命令 | ⏳ | YAML 配置的自定义命令解析 |

### 2. 集成工作

- [ ] 将虚拟命令集成到 SSH 会话
- [ ] 实现实际的 SFTP 操作（替换占位符）
- [ ] 添加 @reconnect 命令
- [ ] 实现自定义命令解析

### 3. 测试和验证

- [ ] 单元测试
- [ ] 集成测试
- [ ] 手动测试

---

## 技术架构

### 目录结构

```
asshv2/asshc/
├── domain/
│   └── virtual.go        # 虚拟命令实体定义
├── port/
│   └── virtual.go        # 接口定义
├── service/
│   └── virtual.go        # 服务实现
└── infra/
    └── virtual/
        ├── config.go     # 配置加载器
        ├── handler.go    # 输入拦截器
        ├── push.go       # @push 命令
        ├── pull.go       # @pull 命令
        ├── help.go       # @help 命令
        ├── version.go    # @version 命令
        └── status.go     # @status 命令
```

### 核心接口

```go
// VirtualCommandHandler 虚拟命令处理器接口
type VirtualCommandHandler interface {
    Execute(ctx context.Context, args []string, session SSHSession) (*domain.VirtualCommandResult, error)
    Name() string
    Description() string
    Usage() string
    IsEnabled() bool
}

// VirtualCommandService 虚拟命令服务接口
type VirtualCommandService interface {
    Register(handler VirtualCommandHandler) error
    Execute(ctx context.Context, input string, session SSHSession) (*domain.VirtualCommandResult, error)
    GetCommand(name string) (VirtualCommandHandler, error)
    ListCommands() []VirtualCommandHandler
    IsVirtualCommand(input string) bool
}
```

### 命令执行流程

```
用户输入 (@push file.txt)
    │
    ▼
InputHandler.HandleInput()
    │
    ├── 检查是否是虚拟命令
    │
    ▼
VirtualCommandService.Execute()
    │
    ├── 解析命令和参数
    ├── 查找命令处理器
    ├── 检查命令是否启用
    ├── 创建超时上下文
    └── 执行命令处理器
        │
        ▼
    PushCommandHandler.Execute()
        │
        ├── 解析参数
        ├── 检查本地文件
        └── 执行 SFTP 上传（待实现）
```

---

## 下一步计划

### 短期（1-2天）

1. **实现 @run 命令**
   - 本地命令执行
   - 输出捕获
   - 超时控制

2. **实现 @deploy 命令**
   - 文件上传
   - 远程执行
   - 结果收集

3. **集成 SFTP 操作**
   - 替换 @push 占位符
   - 替换 @pull 占位符
   - 进度显示

### 中期（3-5天）

1. **实现自定义命令**
   - YAML 配置解析
   - 脚本执行
   - 变量替换

2. **SSH 会话集成**
   - 输入拦截集成
   - 会话状态管理
   - 错误处理

3. **测试和验证**
   - 单元测试
   - 集成测试
   - 手动测试

### 长期（1周+）

1. **功能完善**
   - @reconnect 命令
   - 命令历史记录
   - 自动补全

2. **性能优化**
   - 并发传输
   - 进度显示优化
   - 错误恢复

3. **文档完善**
   - 用户文档
   - 开发文档
   - API 文档

---

## 问题和风险

### 技术问题

1. **SFTP 集成**
   - 需要复用现有的 SFTP 实现
   - 需要处理连接管理和错误恢复

2. **输入拦截**
   - 需要在 SSH 会话中拦截用户输入
   - 需要处理特殊字符和转义

3. **超时控制**
   - 需要正确处理命令超时
   - 需要优雅地取消正在执行的操作

### 进度风险

1. **SFTP 集成复杂度**
   - 可能需要重构现有代码
   - 需要处理并发和同步

2. **测试覆盖**
   - 需要编写大量测试用例
   - 需要模拟 SSH 会话

---

## 经验总结

### 开发经验

1. **接口设计**
   - 清晰的接口定义有助于模块解耦
   - 使用接口可以方便测试和扩展

2. **配置管理**
   - YAML 配置文件易于维护
   - 默认值可以简化配置

3. **错误处理**
   - 统一的错误类型有助于调试
   - 详细的错误信息有助于用户理解

### 改进建议

1. **代码组织**
   - 可以考虑将命令处理器进一步模块化
   - 可以添加命令注册的自动发现机制

2. **测试策略**
   - 应该先编写测试用例
   - 应该使用 mock 对象进行单元测试

3. **文档维护**
   - 应该及时更新文档
   - 应该添加代码注释

---

## 相关文档

- [X2 需求确认](../requirements/RQ-003-virtual-commands.md)
- [版本规划](../v2.2.0-preview.1-plan.md)
- [TODO.md](../TODO.md)

---

## 变更记录

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2026-06-14 | 1.0 | 初始开发进度记录 |
