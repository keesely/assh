# Gossh 
----

一个可以记录密码的SSH客户端，具有 sftp/scp 及跨平台特性。

# 功能特性
- [x] `gossh c [服务器名称]` | `gossh [服务器名称]` 支持 ssh 快捷登陆
- [x] 支持记录ssh密码
- [x] 支持独立公钥登陆(每个服务器用户可以独立关联一个公钥)
- [x] `gossh [add/mod/rm/info/ls]` 服务器记录(新增/修改/删除/查看详情/列表)
- [ ] 支持服务器群组编排
- [ ] 使用数据文件密钥加密
- [ ] 支持安全启动密码
- [ ] `gossh sync [云端标识]` 同步备份服务器列表到自定义云端(七牛/OSS/AWS)
- [ ] `gossh sftp / gossh scp` 支持 sftp / scp 功能
- [ ] `gossh fs`支持 sshfs 挂载
- [ ] `gossh upgrade`自动检测更新/自动升级
- [ ] `gossh logs [服务器名称]`日志记录/查看
- [ ] `gossh ping [服务器名称]` ping服务器

# 安装

## 编译安装

执行编译脚本即可: `sh build.sh`

> 暂不支持 Windows 平台编译

## 下载安装
> 略

# 使用

### 添加服务器

```shell
$ gossh add
请按照提示填入服务器信息(标记* 为必要填写项目): 
1. Please input [*Name] > 
1. Please input [*Host] > 
1. Please input [*Port] > 
1. Please input [*User] > 
1. Please input [Password] > 
1. Please input [PemKey] > 
```

### 登陆服务器


```shell
## 安全启动密码
$ gossh account (密码)

## 快捷登陆
$ gossh (groupName.serverName)

$ gossh login groupName.serverName

## 陌生登陆
$ gossh login [-u 用户名] [-p 密码] [-k 公钥] [-h 远程主机名/ip] [-P 端口] [-c 执行命令]

## 添加服务器
$ gossh add (groupName.serverName) [-h 远程主机名/ip] [-P 端口/22] [-u 用户名/root] [-p 登陆密码] [-k 指定公钥]

## 同步配置
$ gossh sync (配置的同步云端名称)

## 检测更新
$ gossh upgrade

## 推送文件
$ gossh push (groupName.serverName) [local file] [remote file]

## 服务器群组(批量)文件推送
$ gossh push (groupName) [local file] [remote file]

## 拉取文件
$ gossh pull (groupName.serverName) [remote file] [local file]
## 服务器群组(批量)文件拉取
$ gossh pull (groupName) [remote file] [local file]

## 生成ssh key
$ gossh keygen [-c 指定密钥描述] [-f 密钥文件名称]

## 指定主机生成ssh key
### 指定主机生成ssh key 会执行key的生成和 ssh-copy-id 文件
$ gossh keygen (groupName.serverName)

```

