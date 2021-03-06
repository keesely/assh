# Assh  (An ssh) Client

一个可以记录密码的SSH客户端

# 功能特性
- [x] 支持 `assh [服务器名称]` 快捷登陆
- [x] 支持记录ssh密码
- [x] 支持独立公钥登陆(每个服务器用户可以独立关联公钥)
- [x] `assh [add/mod/rm/info/ls]` 服务器记录(新增/修改/删除/查看详情/列表)
- [x] 支持服务器群组
- [x] 支持安全启动密码
- [x] 使用数据文件密钥加密
- [x] `assh push / assh pull` 支持 scp [push / pull] 文件及文件夹
- [x] `assh sync [云端标识]` 同步备份服务器列表到自定义云端 - 目前只支持七牛云
- [x] `assh import/export <file.yml` 导入/导出服务器配置数据
- [x] `assh upgrade`自动检测更新/自动升级
- [ ] `assh ping [服务器名称]` ping服务器

# 安装

## 编译安装

执行编译脚本即可: `sh build.sh`

> 暂不支持 Windows 平台编译

## 下载安装
> 略

# 使用

## 命令列表

| 命令    | 命令描述                      | 命令参数                                      | 备注                             |
|---------|-------------------------------|-----------------------------------------------|----------------------------------|
| account | 设置安全密码                  | 密码                                          | 解析数据的关键密码               |
| ls      | 列出已添加的服务器            | 无                                            | 略                               |
| search  | 检索并列举服务器              | 检索key                                       | 略                               |
| info    | 输出服务器信息                | 组名称.服务器名称                             | 略                               |
| set     | 添加/修改服务器               | 组名称.服务器名称 `-u` `-p` `-k` `-host` `-P` | 略                               |
| rm      | 删除服务器                    | 组名称.服务器名称                             | 略                               |
| login   | 登陆服务器                    | 组名称.服务器名称 `-c`                        | 执行远程命令将不执行登陆         |
| sync    | 同步数据                      | `account` `push` `pull`                       | 同步数据到云端或从云端同步到本地 |
| keygen  | 生成ssh rsa key               | `-c` `-f`                                     | 略                               |
| upgrade | 检测更新                      | 无                                            | 略                               |
| mv      | 服务器更名/转移群组           | 略                                            | 略                               |
| push    | 推送本地文件/文件夹到服务器   |                                               |                                  |
| pull    | 从服务器拉取文件/文件夹到本地 |                                               |                                  |

配置参数:

```
$ assh account 设置安全密码
$ assh -log 设置日志文件路径
$ assh -llv 设置日志等级 (off | debug | info | warn | error | panic | fatal )
```

> 注意: 初始情况下，`assh` 不会开启日志，如需查看相关信息，请设置 `--log` `--llv` 设置对应的日志参数

## 安全密码

assh 开始使用之前需先设定一个安全密码, 安全密码为数据文件提供加密密钥

```
$ assh account <password>
```

> 注意:
如果您之前已设置过安全密码，并且同步文件到云端，安全密码不会同数据同步到云端
且每台机器会新生成独立的rsa公/私钥，所以安全密码即使明文相同，密钥文件也会不同
如果您在新机器上同步下载云端数据后，需要先设定与之前机器的安全密码相同的密码明文才可使用同步的数据
密码改动后相应的数据文件也会被重新编码，将会多台设备使用相同数据出错，请谨慎设置

## 添加服务器

```shell
$ assh add group.name [-u 用户名] [-p 密码] [-k 公钥位置] [-host 远程主机名/IP] [-P 端口]

## 或者

$ assh add group.name user@hostname [-P 端口] [-p 密码] [-k 公钥位置]

```

> 注意:
- 不指定登陆端口，则默认`22`端口
- 不指定密码默认使用`本地公钥文件(~/.ssh/id_rsa)`
- 不指定用户名，则默认`root`
- 添加服务器的服务器名称是必须的, 格式如: group.name
- group可为空，如: `assh server_1 -u root ...`


## 快捷登陆

```
$ assh group.name

## 执行远程命令
$ assh group.name -c "远程命令"

## 或者
$ assh login group.name

$ assh login group.name -c "远程命令"

## 陌生登陆
$ assh [-u 用户名] [-p 密码] [-k 公钥] [-host 远程主机名/ip] [-P 端口]

$ assh [-u 用户名] [-p 密码] [-k 公钥] [-host 远程主机名/ip] [-P 端口] [-c 远程命令]

```

添加`-c "需要执行的命令"` 参数可以执行远程命令，并在终端输出结果
陌生登陆的情况下，服务器并不会保存在服务器列表中，如需保存，请执行`assh add`命令

--ps: 后期将会添加群组执行命令的方法--

## 推送/拉取文件

支持多文件及文件夹推送/拉取

```
## 推送文件到远程主机
$ assh push group.name [local path] [remote path]

## 从远程主机拉取文件
$ assh pull group.name [remote path] [local path]

```
多文件推送/拉取:
`$ assh push/pull group.name 文件路径1 文件路径2 ... 目标路径`

> 注意:
- 单文件推送/拉取时可以不指定目标路径(默认使用本地当前路径 / 远程主机则为家目录)
- 多文件推送/拉取时最后一个参数为目标路径
- 群组推送将会在后期实现，目前只支持单服务器推送

## 同步数据文件到七牛云
Assh 可以支持数据文件同步到云端，目前只支持七牛云

```
## 同步权限设置
$ assh sync account <accessKey> <secretKey> <bucket> <enpoint>

## 同步本地数据到云端
$ assh sync push [同步标识]

## 同步云端数据到本地
$ assh sync pull [同步标识]
```

> 注意:
- 同步数据到云端时将会覆盖原有的数据
- 同步标识为空时，将使用默认标识`backup`
- 同步云端数据到本地时，本地文件将会被覆盖，请确认您本地的数据已做好备份或同步


## 生成ssh key
```
$ assh keygen [-c 指定密钥描述] [-f 密钥文件名称]
```
## 指定主机生成ssh key
```
$ assh keygen group.name
```
> 指定主机生成ssh key 会执行key的生成和 ssh-copy-id 文件

## 检测更新

```
$ assh upgrade
```
PS: 暂未实现
