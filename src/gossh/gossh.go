// gossh.go kee > 2019/11/08

package gossh

import (
	"fmt"
	"github.com/keesely/kiris"
	"github.com/keesely/kiris/hash"
	"log"
	"os"
)

//var passwd = hash.Md5("keesely.net")
var passwd string

type GoSSH struct {
	data   *kiris.Yaml
	dbFile string
}

// 初始化必要数据
func init() {
	if "" == passwd {
		log.Fatal("请输入启动密码")
	}
	// 判断是否存在密码文件
	fmt.Println("Passwd: ", passwd)
	passwd = hash.Md5(passwd)
	fmt.Println("Passwd => : ", passwd)

}

func NewGoSSH() *GoSSH {
	cPath := kiris.RealPath("~/.gossh")
	cFile := cPath + "/servers.ydb"

	passwd = hash.Md5("keesely.net")
	if !kiris.IsDir(cPath) {
		// 创建配置目录
		if err := os.MkdirAll(cPath, os.ModePerm); err != nil {
			log.Fatalf("mkdir %s fail", cPath, err.Error())
		}
	}

	var _data = []byte{}
	if kiris.FileExists(cFile) {
		// 导入数据文件
		_data = getDataContents(cFile)
	}
	data := kiris.NewYaml(_data)

	return &GoSSH{data, cFile}
}

// 获取数据文件内容
func getDataContents(datafile string) []byte {
	content, err := kiris.FileGetContents(datafile)
	if err != nil {
		log.Fatal(err)
	}
	// 解码还原数据
	_c := kiris.AESDecrypt([]byte(content), passwd, "cbc")
	return []byte(_c)
}

func (c *GoSSH) ListServers() {
	fmt.Println(kiris.StrPad("", "=", 100, 0))
	fmt.Printf("| %-20s | %-20s | %-50s |\n", "Group Name", "Server Name", "Server Host")
	fmt.Println(kiris.StrPad("", "-", 100, 0))
	data := c.data.Get("")
	for g, ss := range data.(map[string]interface{}) {
		for n, _s := range ss.(map[string]interface{}) {
			s := &Server{}
			kiris.ConverStruct(_s.(map[string]interface{}), s, "yaml")
			passwd := kiris.Ternary(s.Password != "", "yes", "no").(string)
			fmt.Printf("> %-20s | %-20s | %s@%s:%d (password:%s) \n", g, n, s.User, s.Host, s.Port, passwd)
		}
	}
	c.save()
	fmt.Println(kiris.StrPad("", "=", 100, 0))
}

func (c *GoSSH) AddServer(name string, server *Server) {
	name = "default." + name
	c.data.Set(name, &server)
	// 保存
	c.save()
	fmt.Printf("Server [%s] add success!\n", name)
	//fmt.Println("Save to ", saveFs)
}

func (c *GoSSH) GetServer(name string) *Server {
	if server := c.data.Get(name); server != nil {
		ss := &Server{}
		kiris.ConverStruct(server.(map[string]interface{}), ss, "yaml")
		return ss
		/*
			return &Server{
				Name:     s.Name,
				Host:     s.Host,
				Port:     s.Port,
				User:     s.User,
				Options:  s.Options,
				Password: s.Password,
				PemKey:   s.PemKey,
			}
		*/
	}
	return nil
}

func (c *GoSSH) DelServer(name string) {
	c.data.Set(name, nil)
	c.save()
	fmt.Println("删除成功: ", name)
	return
}

func (c *GoSSH) save() {
	str, _ := c.data.SaveToString()
	// 加密
	save := kiris.AESEncrypt(string(str), passwd, "cbc")
	//fmt.Println("encrypt: ", string(kiris.Base64Decode(save)))

	saveFs := kiris.RealPath(c.dbFile)
	//if e := c.data.SaveAs(saveFs); e != nil {
	if e := kiris.FilePutContents(saveFs, string(save), 0); e != nil {
		log.Fatal(e)
	}
}
