// config.go kee > 2019/11/25

package assh

import (
	key "assh/src/keygen"
	"assh/src/log"
	"github.com/keesely/kiris"
	"github.com/keesely/kiris/hash"
	"os"
)

var (
	cPath         = kiris.RealPath("~/.assh")
	dbPath        = kiris.RealPath(cPath + "/data")
	pemFile       = cPath + "/.rsa"
	pubFile       = cPath + "/.rsa.pub"
	passFile      = cPath + "/.account"
	cnfFile       = cPath + "/assh.yml"
	customCnfFile = kiris.RealPath("~/.assh.yml")
	cnfYaml       *kiris.Yaml
)

var cnfDefaultContent = `logPath: ~/.assh/assh.log
logLevel: INFO
dbPath: ` + dbPath + `
qiniuAccessKey: 
qiniuSecretKey:
`

func init() {
	if !kiris.IsDir(cPath) {
		// 创建配置目录
		if err := os.MkdirAll(cPath, os.ModePerm); err != nil {
			log.Fatalf("mkdir %s fail", cPath, err.Error())
		}
	}

	// 密钥文件生成
	if !kiris.FileExists(pemFile) && !kiris.FileExists(pubFile) {
		rsa, err := key.NewRsa(2048)
		if err != nil {
			log.Fatal(err)
		}
		public, private := rsa.GenPem()
		if err = kiris.FilePutContents(pubFile, public, 0); err != nil {
			log.Fatal(err)
		}
		if err = kiris.FilePutContents(pemFile, private, 0); err != nil {
			log.Fatal(err)
		}
	}

	// 创建默认配置
	if !kiris.FileExists(cnfFile) {
		saveConfig(cnfDefaultContent)
	}
	cnfYaml = getConfig()

	dbPath = GetDbPath()
	if !kiris.IsDir(dbPath) {
		// 创建数据目录
		if err := os.Mkdir(dbPath, os.ModePerm); err != nil {
			log.Fatalf("mkdir %s fail", err.Error())
		}
	}

	// 初始化日志
	log.LogFile = cnfYaml.Get("logPath").(string)
	log.LogLevel = log.GetLogLevel(cnfYaml.Get("logLevel").(string))
	log.SetInit()
}

func SetDbPath(dbPath string) {
	cnfYaml.Set("dbPath", dbPath)
	cnfYaml.Save()
}
func SetLogPath(logPath string) {
	cnfYaml.Set("logPath", logPath)
	log.LogFile = logPath
	cnfYaml.Save()
}

func SetLogLevel(logLevel string) {
	cnfYaml.Set("logLevel", logLevel)
	log.LogLevel = log.GetLogLevel(logLevel)
	cnfYaml.Save()
}

func SetQiniuAccessKey(accessKey, secretKey string) {
	cnfYaml.Set("qiniuAccessKey", accessKey)
	cnfYaml.Set("qiniuSecretKey", secretKey)
	cnfYaml.Save()
}

func getConfig() *kiris.Yaml {
	cnfByte, _ := kiris.FileGetContents(cnfFile)
	cnf, err := RsaDecrypt([]byte(cnfByte), pemFile)
	if err != nil {
		log.Fatal(err)
	}
	cnfYaml = kiris.NewYaml(cnf)

	if kiris.FileExists(customCnfFile) {
		custom := kiris.NewYamlLoad(customCnfFile)
		for k, _ := range cnfYaml.Get("").(map[string]interface{}) {
			if cv := custom.Get(k); cv != nil || cv != "" {
				cnfYaml.Set(k, cv)
			}
		}
	}
	return cnfYaml
}

func saveConfig(content string) {
	output, err := RsaEncrypt([]byte(content), pubFile)
	if err != nil {
		log.Fatal(err)
	}
	kiris.FilePutContents(cnfFile, string(output), 0)
}

func GetDbPath() string {
	return cnfYaml.Get("dbPath", dbPath).(string)
}

func GetLogPath() string {
	return cnfYaml.Get("logPath", cPath+"/assh.log").(string)
}

func GetLogLevel() string {
	return cnfYaml.Get("logLevel", "OFF").(string)
}

func GetPasswd() string {
	// 判断是否存在密码文件
	if !kiris.FileExists(passFile) {
		log.Fatal("You have not set the password.")
	}

	ciphertext, err := kiris.FileGetContents(passFile)
	if err != nil {
		log.Fatal(err)
	}

	privateKey, err := kiris.FileGetContents(pemFile)
	if err != nil {
		log.Fatal(err)
	}
	decrypt, err := key.RsaDecrypt(ciphertext, privateKey)
	if err != nil {
		log.Fatal(err)
	}
	return string(decrypt)
}

func SetPasswd(passwd string) {
	if passwd == "" {
		log.Fatal("The password is empty")
	}
	publicKey, err := kiris.FileGetContents(pubFile)
	if err != nil {
		log.Fatal(err)
	}
	passwd = hash.Md5(passwd)
	encrypt, err := key.RsaEncrypt([]byte(passwd), publicKey)
	if err != nil {
		log.Fatal(err)
	}
	kiris.FilePutContents(passFile, string(encrypt), 0)
}
