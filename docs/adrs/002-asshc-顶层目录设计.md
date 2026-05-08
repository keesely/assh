# ADR-002: asshc 顶层目录设计（无 internal 嵌套）

## Status
accepted

## Date
2026-05-08

## Context
Go 1.22 的 internal/ 包有编译器级封装保护，但用户选择不使用 internal/ 嵌套，保持目录扁平。

## Decision
asshc/ 作为核心业务库放在 asshv2/ 顶层，内部继续使用 domain/port/service/infra 分层，不使用 Go 的 internal/ 包机制。

## Consequences
1. 架构更扁平，导入路径更短（assh/asshc/port 而非 assh/asshv2/internal/asshc/port）
2. 失去编译器级封装保护，依赖完全靠代码规范
3. 与 Go 社区标准实践略有偏离