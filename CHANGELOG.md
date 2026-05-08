# Changelog

## v2.0.0-dev (2026-05-08)

### Phase 1: 基础设施移植

- `log/` — 基于 zerolog 的日志模块移植
  - 7 级日志（OFF/FATAL/PANIC/ERROR/WARN/INFO/DEBUG）
  - 文件输出 / stderr 输出
  - 28 个测试用例全部通过
- `config/` — 配置管理模块移植
  - 路径管理（ExpandPath、EnsureDir、FileExists）
  - 配置读写接口（日志、数据库、七牛云、密码）
  - 优化：移除 config 对 keygen 的依赖（crypto 移至 infra 层）
- `asshc/infra/crypto/` — 加解密模块移植
  - RSA 密钥生成、PEM 解析、加解密（PKCS#1.5 / OAEP）
  - AES 加解密（CBC / CTR / GCM / ECB）
  - Ed25519 密钥生成
  - ECDSA 密钥生成（P256 / P384 / P521）
  - 包名从 `keygen` 改为 `crypto`，移至 `asshc/infra/crypto/`
- `asshc/port/cryptor.go` — Cryptor 接口定义

### Phase 0: 项目骨架搭建

- 目录结构创建（asshc/domain、port、service、infra/ 分层）
- `build.sh` — 构建脚本（多平台交叉编译）
- `main.go` — 程序入口（DI 组合根）
- `cmd/app.go` — CLI 框架（urfave/cli）

### 架构变更

- 引入 `asshc/` 核心业务层，采用 port/service/infra 三层架构
- `service/` 仅依赖 `port/` 接口，不直接依赖具体实现
- 所有依赖通过构造器注入
- 移除 `config/` 中的加密方法，移至 `asshc/infra/crypto/`
