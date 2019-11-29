// assh.go kee > 2019/11/08

package assh

import (
	"assh/src/log"
	"encoding/json"
	"fmt"
	"github.com/keesely/kiris"
	"github.com/keesely/kiris/hash"
	//"log"
	"os"
	"path"
	"strings"
	"sync"
)

type Assh struct {
	data   map[string]map[string]*Server
	dbFile string
	sync.RWMutex
}

var passwd string

func NewAssh() *Assh {
	cFile := cPath + "/servers.db"
	passwd = GetPasswd()
	return loadData(cFile)
}

// 载入数据
func loadData(file string) *Assh {
	data := make(map[string]map[string]*Server)
	if kiris.FileExists(file) {
		c, err := kiris.FileGetContents(file)
		if err != nil {
			log.Fatal(err)
		}
		if hash.Md5(passwd) != string(c[:32]) {
			log.Fatal("Access denied for password. run assh account [your password]")
		}
		cryptoStr := string(c[64:])
		_c := kiris.AESDecrypt([]byte(cryptoStr), passwd, "cbc")
		if err := json.Unmarshal([]byte(_c), &data); err != nil {
			log.Fatal(err)
		}
		servers := Assh{data: data, dbFile: file}
		return &servers
	}
	return &Assh{data: data, dbFile: file}
}

func (c *Assh) ListServers() {
	fmt.Println(kiris.StrPad("", "=", 160, 0))
	fmt.Printf("  %-20s | %-20s | %-50s | %-50s \n", "Group Name", "Server Name", "Server Host", "Remark")
	fmt.Println(kiris.StrPad("", "-", 160, 0))
	data := c.data
	for g, ss := range data {
		for n, s := range ss {
			sInfo := fmt.Sprintf("%s@%s:%d (%s)",
				s.User,
				s.Host,
				s.Port,
				kiris.Ternary(s.Password != "",
					"passwd:yes",
					kiris.Ternary(s.PemKey != "", "pub key:yes", "passwd:no").(string),
				).(string),
			)
			remark := kiris.Ternary(s.Remark != "", s.Remark, " (no remark) ")
			fmt.Printf("> %-20s | %-20s | %-50s | %s \n", g, n, sInfo, remark)
		}
	}
	//c.save()
	fmt.Println(kiris.StrPad("", "=", 160, 0))
}

func (c *Assh) GetGroup(group string) map[string]*Server {
	_s, ok := c.data[group]
	if !ok {
		return nil
	}
	return _s
}

func (c *Assh) GetServer(name string) *Server {
	group, _name := parseName(name)
	if server, ok := c.data[group][_name]; ok {
		return server
	}
	return nil
}

func (c *Assh) AddServer(name string, server Server) {
	if server.Password == "" && server.PemKey == "" {
		server.PemKey = "~/.ssh/id_rsa"
	}
	if server.PemKey != "" {
		SetPemKey(&server)
	}
	group, _name := parseName(name)
	g, ok := c.data[group]
	if !ok {
		c.data[group] = make(map[string]*Server)
		g = c.data[group]
	}
	g[_name] = &server

	// 保存
	c.save()
	//fmt.Printf("Server [%s] add success!\n", name)
}

func (c *Assh) MoveServer(name, newName string) {
	s := c.GetServer(name)
	if s == nil {
		log.Fatalf("Server [%s] is not exists \n", name)
	}
	c.AddServer(newName, Server{
		Name:     s.Name,
		Host:     s.Host,
		Port:     s.Port,
		User:     s.User,
		Password: s.Password,
		PemKey:   s.PemKey,
		Remark:   s.Remark,
	})

	c.DelServer(name)
}

func (c *Assh) SetRemark(name, remark string) {
	ss := c.GetServer(name)
	if ss == nil {
		log.Fatalf("Server (%s) not found \n", name)
	}
	ss.Remark = remark
	c.save()
}

// 设置密钥
func SetPemKey(server *Server) {
	if server.PemKey != "" && !kiris.FileExists(server.PemKey) {
		return
	}
	serverName := strings.Replace(server.Name, ".", "/", 0)
	pemKeyPath := strings.Join([]string{GetDbPath(), serverName}, "/")
	os.Mkdir(pemKeyPath, os.ModePerm)

	// copy ...
	src, err := os.Open(server.PemKey)
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()

	fmt.Println("< base path >", pemKeyPath, path.Base(server.PemKey))
	dstPemFile := path.Join(pemKeyPath, path.Base(server.PemKey))
	dst, err := os.Create(dstPemFile)
	if err != nil {
		log.Fatal(err)
	}
	defer dst.Close()

	var buf = make([]byte, 2048)
	for {
		n, err := src.Read(buf)
		if err != nil {
			break
		}
		dst.Write(buf[:n])
	}
	server.PemKey = dstPemFile
	return
}

func (c *Assh) DelServer(name string) {
	group, _name := parseName(name)
	g := c.GetGroup(group)

	if _, ok := g[_name]; !ok {
		log.Fatalf("Server (%s) not found\n", name)
	}
	delete(g, _name)
	c.save()
}

func (c *Assh) save() {
	saveFs := kiris.RealPath(c.dbFile)
	jsonBytes, _ := json.Marshal(c.data)
	jsonStr := string(jsonBytes)
	cryptoBytes := kiris.AESEncrypt(jsonStr, passwd, "cbc")
	cryptoHash := hash.Md5(string(cryptoBytes))
	content := hash.Md5(passwd) + cryptoHash + string(cryptoBytes)
	if e := kiris.FilePutContents(saveFs, content, 0); e != nil {
		log.Fatal(e)
	}
}

func parseName(name string) (string, string) {
	keys := strings.Split(name, ".")
	if len(keys) < 2 {
		def := []string{""}
		keys = append(def, keys[0])
	}
	return keys[0], keys[1]
}
