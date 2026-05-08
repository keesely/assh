# ADR-004: crypto 模块从 config 移至 asshc/infra/crypto

## Status
accepted

## Date
2026-05-08

## Context
原 config/keygen/ 混合了用户级密钥生成（keygen 命令用）和内部数据加密，职责不清晰。

## Decision
crypto 相关的 RSA/AES/Ed25519/ECDSA 实现移至 asshc/infra/crypto/，作为内部实现细节，config/ 仅保留配置路径管理，不依赖 crypto；asshc/port/cryptor.go 定义加密接口。

## Consequences
1. 职责分离：config 是公开 API，crypto 是内部实现
2. 可替换加密算法实现（接口抽象）
3. 需同步更新 import 路径
4. 原 config 包的 EncryptConfig 功能需重构至 crypto 层