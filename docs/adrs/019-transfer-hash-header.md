# ADR-019: 传输文件 Hash 头部记录与断点续传安全验证

## Status
accepted

## Date
2026-05-10

## Context

当前 Phase 5 SFTP 的断点续传方案仅依赖 **文件大小比对**（`sftp.Stat()` vs `os.Stat()`）来判断是否可以续传。这种方式存在安全隐患：

1. **文件内容被修改但大小不变**：远程/本地文件在中断期间被修改，但巧合保持相同大小 → 续传后产生损坏文件
2. **无源文件完整性确认**：无法确认源文件在两次传输之间是否被篡改或修改
3. **已有 `--checksum` 是传输后校验**：只能检测最终结果，无法在续传前预防

需要一个在传输过程中持续可用的哈希验证机制，既能保障续传安全，又不显著增加传输开销。

## Decision

### 方案：固定长度 Hash Header

在目标文件头部写入固定 256 字节的 hash header，记录源文件的 SHA256 哈希。

### Header 格式

```
偏移   长度   字段        说明
0      4     Magic       "ASSH" (0x41 0x53 0x53 0x48)
4      2     Version     Header 格式版本 (uint16, 当前=1)
6      2     HashType    哈希算法 (uint16, 1=SHA256)
8      8     OrigSize    原始文件大小 (int64, 不含 header)
16     64    HashHex     SHA256 小写 hex (64 bytes + null terminator)
80     176   Reserved    预留字段，全零填充
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
总计: 256 bytes
```

### 传输流程

#### ① 首次传输（push）

```
本地文件                       远程服务器
┌─────────────┐              ┌──────────────┐
│  data.bin   │              │  ASSH Header  │ ← 256B hash header
│  (1MB)      │  ───────→    │  ──────────── │
│             │              │  data.bin     │ ← 原始文件内容
└─────────────┘              │  (1MB)        │
                             └──────────────┘
```

步骤：
1. 计算本地文件 SHA256
2. 构建 256B header（含 hash + 原始大小）
3. 发送 header → 发送文件内容
4. 传输完成后：重写远程文件 **去除 header**（恢复到原始格式）

#### ② 续传验证（push resume）

```
1. 读取远程文件前 256B → 解析 header
2. 提取 stored_hash + stored_size
3. 计算当前本地文件 SHA256 = current_hash
4. 比对：
   ├── stored_hash == current_hash → 文件未被修改 → 从 offset=256 续传
   ├── stored_hash != current_hash → 文件被修改 → 询问/强制覆盖
   └── 无法读取 header（文件不存在或无 magic）→ 回退到大小比对
```

#### ③ 首次传输（pull）

```
远程文件                       本地服务器
┌──────────────┐             ┌──────────────┐
│  data.bin    │             │  ASSH Header  │ ← 256B hash header
│  (1MB)       │  ←───────  │  ──────────── │
│              │             │  data.bin     │ ← 原始文件内容
└──────────────┘             │  (1MB)        │
                             └──────────────┘
```

步骤：
1. 通过 SSH exec 在远程执行 `sha256sum data.bin` 获取远程文件哈希（或通过 sftp 流式读取计算）
2. 构建 256B header
3. 写入本地 header → 下载文件内容
4. 传输完成后：重写本地文件 **去除 header**

#### ④ 续传验证（pull resume）

```
1. 读取本地文件前 256B → 解析 header
2. 通过 SSH exec 获取远程文件 SHA256
3. 比对 → 一致则续传，不一致则询问
```

### 完成后的清理

传输完成后，重写文件去除 header：

```go
func stripHeader(path string) error {
    f, _ := os.Open(path)           // 或 sftp.Open(remotePath)
    defer f.Close()
    
    header := make([]byte, 256)
    f.Read(header)                  // 读取 header
    
    stat, _ := f.Stat()
    contentLen := stat.Size() - 256 // 实际内容长度
    
    // 创建临时文件
    tmpPath := path + ".tmp"
    tmp, _ := os.Create(tmpPath)
    
    // 跳过 header，写入内容
    f.Seek(256, io.SeekStart)
    io.Copy(tmp, io.LimitReader(f, contentLen))
    
    // 替换原文件
    tmp.Close()
    f.Close()
    os.Rename(tmpPath, path)
}
```

### 交互关系

| 场景 | hash header | `--checksum` |
|------|-------------|--------------|
| 目的 | **续传前**验证源文件未修改 | **传输后**验证目标文件完整性 |
| 触发条件 | `--resume` 时自动启用 | `--checksum`/`-S` 显式指定 |
| 计算对象 | 源文件（本地/远程） | 目标文件（远程/本地） |
| 存储方式 | 目标文件头部 256B（临时的） | 不存储，仅输出结果 |
| 清理方式 | 传输完成后自动去除 header | 不涉及 |

### 回退策略

当以下情况发生时，自动回退到纯大小比对：
- 目标文件不存在（首次传输，无需验证）
- 目标文件小于 256B（无法读取 header）
- header magic 不匹配（文件非 assh 传输产生，可能是用户手动放置的文件）
- header version 不支持（未来格式升级，向前兼容）

## Consequences

### 正面影响
- 续传安全：确保源文件在中断期间未被修改，防止静默损坏
- 与 `--checksum` 互补：前者预防，后者验证，双重保障
- 固定 256B 头部开销极小（1MB 文件仅 0.024% 开销）
- 头部格式有 magic + version，向前兼容
- 传输完成后目标文件完全恢复原始格式，对用户透明

### 负面影响
- **传输完成后需重写文件去除 header**：在远程（push）或本地（pull）多一次写操作
  - 大文件场景：重写时间 ≈ 传输时间 × 0.1（本地重写 vs 网络传输）
  - 缓解：传输完成后立即重写，保留原始文件直到重写完成
- **pull 操作需额外 SSH exec 获取远程哈希**：一次 SSH 命令执行开销
- **并发传输时的冲突**：多 goroutine 同时写 header 需加锁
- 增加传输逻辑复杂度（header 构建/解析/剥离三阶段）

### 影响范围

| 范围 | 文件 |
|------|------|
| 新增 | `asshc/infra/sftp/header.go` — HashHeader 构建/解析/剥离 |
| 修改 | `asshc/infra/sftp/upload.go` — push 流程插入 header 逻辑 |
| 修改 | `asshc/infra/sftp/download.go` — pull 流程插入 header 逻辑 |
| 修改 | `asshc/infra/sftp/verify.go` — 新增 resume 前 hash 比对 |
| 新增测试 | `asshc/infra/sftp/header_test.go` |
| 文档 | `docs/modules/sftp.md` — 更新设计说明 |

### 性能分析

| 文件大小 | header 开销 | 重写时间 (本地 SSD) | 备注 |
|----------|-------------|---------------------|------|
| 1MB | 256B (0.024%) | ~2ms | 可忽略 |
| 100MB | 256B (0.0002%) | ~50ms | 可忽略 |
| 1GB | 256B (0.00002%) | ~500ms | 可接受 |
| 10GB | 256B (0.000002%) | ~5s | 较大，可考虑流式剥离 |

## Related
- ADR-017: SFTP 文件传输架构设计（基础架构）
- docs/modules/sftp.md — SFTP 模块设计
- docs/TODO.md Phase 5 — 实现任务
