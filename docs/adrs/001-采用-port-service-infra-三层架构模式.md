# ADR-001: 采用 port/service/infra 三层架构模式

## Status
accepted

## Date
2026-05-08

## Context
v2.0.0 重构需要清晰的架构分层，Service 层不应直接依赖具体实现，需依赖抽象接口。

## Decision
引入 port/ 目录定义接口（Repository, Connector, Transfer, Proxy, Syncer, Cryptor），service/ 实现业务逻辑仅依赖 port 接口，infra/ 实现具体功能（SSH、SFTP、存储等）。

## Consequences
1. Service 层可测试（Mock port 接口）
2. 更换实现（如换存储后端）无需改动 Service
3. 增加开发学习成本（需理解接口-实现分离）
4. 代码更冗长（每层多一次跳转）