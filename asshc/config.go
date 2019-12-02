// config.go kee > 2019/11/25

package asshc

import (
	key "assh/asshc/keygen"
	"assh/log"
	"fmt"
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
logLevel: OFF
dbPath: ` + dbPath + `
qiniuAccessKey: 
qiniuSecretKey:
qiniuBucket:
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
	log.LogPath = cnfYaml.Get("logPath").(string)
	logLevel := cnfYaml.Get("logLevel")
	if "string" == kiris.Typeof(logLevel) {
		log.LogLevel = log.GetLogLevel(logLevel.(string))
	}
	log.SetInit()
}

func SetDbPath(dbPath string) {
	cnfYaml.Set("dbPath", dbPath)
	cnfSave()
}
func SetLogPath(logPath string) {
	cnfYaml.Set("logPath", logPath)
	log.LogPath = logPath
	cnfSave()
}

func SetLogLevel(logLevel string) {
	cnfYaml.Set("logLevel", logLevel)
	log.LogLevel = log.GetLogLevel(logLevel)
	cnfSave()
}

func SetQiniuAccessKey(accessKey, secretKey, bucket string) {
	cnfYaml.Set("qiniuAccessKey", accessKey)
	cnfYaml.Set("qiniuSecretKey", secretKey)
	cnfYaml.Set("qiniuBucket", bucket)
	cnfSave()
}

func cnfSave() {
	cnfYaml.Save()
	c, _ := cnfYaml.SaveToString()
	saveConfig(string(c))
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

func GetQiniuAccessKey() (accessKey, secretKey, bucket string) {
	accessKey = cnfYaml.Get("qiniuAccessKey", "").(string)
	secretKey = cnfYaml.Get("qiniuSecretKey", "").(string)
	bucket = cnfYaml.Get("qiniuBucket", "").(string)
	return
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

func SetPasswd(passwd string, oldPasswd string) (err error) {
	if passwd == "" {
		log.Fatal("The password is empty")
	}
	if passwd == oldPasswd {
		return
	}
	passwd = hash.Md5(passwd)
	oldPasswd = hash.Md5(oldPasswd)
	if oldPasswd != GetPasswd() {
		err = fmt.Errorf("Invalid old password")
		return
	}

	publicKey, err := kiris.FileGetContents(pubFile)
	if err != nil {
		log.Fatal(err)
	}

	encrypt, err := key.RsaEncrypt([]byte(passwd), publicKey)
	if err != nil {
		log.Fatal(err)
	}
	if kiris.FileExists(passFile) {
		CopyFile(passFile, passFile+"-old")
	}
	// 重编码数据文件
	dbFile := kiris.RealPath(GetDbPath() + "/servers.db")
	if c := decryptData(dbFile, GetPasswd()); c != "" {
		cc := encryptSave([]byte(c), passwd)
		if e := kiris.FilePutContents(dbFile, cc, 0); e != nil {
			log.Fatal(e)
		}
	}
	kiris.FilePutContents(passFile, string(encrypt), 0)
	return
}
