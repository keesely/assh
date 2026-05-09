package main

import (
	"os"

	"assh/cmd"
	sshinfra "assh/asshc/infra/ssh"
	"assh/asshc/service"
)

var Version = "v2.0.0"
var Build string

func main() {
	// DI: 基础设施层
	connector := sshinfra.NewConnector()
	session := sshinfra.NewSession()

	// DI: 服务层（repo 暂为 nil，Phase 4 完成完整注入）
	connectSvc := service.NewConnectService(connector, session, nil)

	// CLI 入口
	app := cmd.NewApp(Version, Build, connectSvc)
	app.Run(os.Args)
}
