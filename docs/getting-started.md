# Getting Started

## 开发命令

- **编译构建**: `cd asshv2 && sh build.sh`
- **单平台构建**: `cd asshv2 && sh build.sh <os> <arch> <alias>`
- **运行**: `cd asshv2 && go run . <命令>`

## 测试

- 运行所有测试: `cd asshv2 && go test ./... -v`
- 单独测试: `cd asshv2 && go test ./asshc/... -v`
- 日志测试: `cd asshv2 && go test ./log/... -v`

## 静态检查

```bash
cd asshv2 && go vet ./...
```

## TDD 流程

每个模块的开发步骤：

1. 编写测试 (`xxx_test.go`)
2. 运行测试（失败 - 编译失败或测试失败）
3. 编写实现 (`xxx.go`)
4. 运行测试（通过）
5. 重构代码
6. 提交代码
