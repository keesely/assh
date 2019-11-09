// config.go kee > 2019/11/08

package config

import (
	"encoding/gob"
	"fmt"
	"github.com/keesely/kiris"
	"gossh/gossh"
	"log"
	"os"
	//"os/exec"
)

type Config struct {
	db     *kiris.K_VMaps
	dbFile string
}

func NewConfig() *Config {
	dbpath := "~/.gossh"
	dbfile := dbpath + "/servers.kvmap"
	db := kiris.NewK_VMaps()

	// 注册格式化结构
	gob.Register(gossh.Server{})

	if !kiris.IsDir(dbpath) {
		//cmd := exec.Command("sh", "-c", "mkdir -p "+dbpath)
		//if err := cmd.Run(); err != nil {
		if err := os.MkdirAll(kiris.RealPath(dbpath), os.ModePerm); err != nil {
			log.Fatalf("mkdir %s fail", dbpath, err.Error())
		}
	}

	// 导入数据文件
	if kiris.FileExists(dbfile) {
		if _, e := db.Load(dbfile); e != nil {
			log.Fatal(e)
		}
	}

	return &Config{db, dbfile}
}

func (c *Config) ListServers() {
	fmt.Println(kiris.StrPad("", "=", 100, 0))
	//fmt.Println(kiris.StrPad(" Server List ", "-", 100, 2))
	//fmt.Println(kiris.StrPad("", "-", 100, 0))
	fmt.Printf("|  %-22s | %-70s |\n", "Name", "Server Host")
	fmt.Println(kiris.StrPad("", "-", 100, 0))
	data := c.db.GetData()
	for n, v := range data {
		s := v.Value.(gossh.Server)
		passwd := kiris.Ternary(s.Password != "", "yes", "no").(string)
		fmt.Printf(" > %-22s | %s@%s:%d (password:%s) \n", n, s.User, s.Host, s.Port, passwd)
	}
	fmt.Println(kiris.StrPad("", "=", 100, 0))
}

func (c *Config) AddServer(name string, server *gossh.Server) {
	c.db.Set(name, &server)
	// 保存
	saveFs := kiris.RealPath(c.dbFile)
	//c.db.Print()
	if e := c.db.Save(saveFs); e != nil {
		log.Fatal(e)
	}
	fmt.Printf("Server [%s] add success!\n", name)
	//fmt.Println("Save to ", saveFs)
}

func (c *Config) GetServer(name string) *gossh.Server {
	if server := c.db.GetValue(name); server != nil {
		s := server.(gossh.Server)
		return &gossh.Server{
			Name:     s.Name,
			Host:     s.Host,
			Port:     s.Port,
			User:     s.User,
			Options:  s.Options,
			Password: s.Password,
			PemKey:   s.PemKey,
		}
	}
	return nil
}

func (c *Config) DelServer(name string) {
	if f := c.db.Del(name); f == true {
		saveFs := kiris.RealPath(c.dbFile)
		if e := c.db.Save(saveFs); e != nil {
			log.Fatal(e)
		}
		fmt.Println("删除成功: ", name)
		return
	}
	fmt.Println("删除失败: ", name)
}
