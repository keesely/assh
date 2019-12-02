// assh.go kee > 2019/12/02

package asshc

import (
	"assh/log"
	"encoding/json"
	"fmt"
	"github.com/keesely/kiris"
	"github.com/keesely/kiris/hash"
	"os"
	"path"
	"strings"
	"sync"
)

type Assh struct {
	data map[string]map[string]*Server
	sync.RWMutex
}

var (
	passwd  = fmt.Sprintf("%s", GetPasswd())
	sDbFile string
)

func NewAssh() *Assh {
	data := make(map[string]map[string]*Server)

	sDbFile = kiris.RealPath(GetDbPath() + "/servers.db")
	if c := decryptData(sDbFile, passwd); c != "" {
		if err := json.Unmarshal([]byte(c), &data); err != nil {
			log.Fatal(err)
		}
	}
	return &Assh{data: data}
}

// 列表 - All
func (s *Assh) List() map[string]map[string]*Server {
	return s.data
}

// 检索
func (s *Assh) Search(name string) map[string]map[string]*Server {
	name = strings.ToLower(name)

	data := make(map[string]map[string]*Server)
	for g, gs := range s.data {
		for n, s := range gs {
			sName := strings.ToLower(s.Name)

			var checkout = func(str, key string) bool {
				if strings.Index(str, key) >= 0 {
					return true
				}
				return false
			}

			if checkout(sName, name) || checkout(s.Host, name) || checkout(s.Remark, name) {
				if _, ok := data[g]; !ok {
					data[g] = make(map[string]*Server)
				}
				data[g][n] = s
			}
		}
	}
	return data
}

// 群组获取
func (s *Assh) GetGroup(group string) map[string]*Server {
	_s, ok := s.data[group]
	if !ok {
		return nil
	}
	return _s
}

// 获取信息
func (s *Assh) Get(name string) *Server {
	group, sname := parseName(name)
	if server, ok := s.data[group][sname]; ok {
		return server
	}
	return nil
}

// 设置 - 新增/修改
func (s *Assh) Set(name string, server Server) {
	if server.Password == "" && server.PemKey == "" {
		server.PemKey = "~/.ssh/id_rsa"
	}
	if server.PemKey != "" {
		setPemKey(&server)
	}

	group, sname := parseName(name)
	g, ok := s.data[group]
	if !ok {
		s.data[group] = make(map[string]*Server)
		g = s.data[group]
	}
	g[sname] = &server

	// 保存
	s.save()
}

// 删除
func (s *Assh) Del(name string) {
	group, sname := parseName(name)
	g := s.GetGroup(group)

	if _, ok := g[sname]; !ok {
		log.Fatalf("Server (%s) not found\n", name)
	}
	delete(g, sname)
	delPemKey(name)
	s.save()
}

// 迁移
func (ss *Assh) Move(from, to string) {
	if targetS := ss.Get(to); targetS != nil {
		log.Fatalf("The target server [%s] already exists, can not cover it. \n", to)
	}

	s := ss.Get(from)
	if s == nil {
		log.Fatalf("Server [%s] does not exist. \n", from)
	}

	ss.Set(to, Server{
		Name:     s.Name,
		Host:     s.Host,
		Port:     s.Port,
		User:     s.User,
		Password: s.Password,
		PemKey:   s.PemKey,
		Remark:   s.Remark,
	})

	ss.Del(from)
}

// 保存数据
func (s *Assh) save() {
	// 转码
	jsonBytes, _ := json.Marshal(s.data)
	content := encryptSave(jsonBytes, passwd)
	if e := kiris.FilePutContents(sDbFile, content, 0); e != nil {
		log.Fatal(e)
	}
}

// 加密存储
func encryptSave(c []byte, passwd string) string {
	// 编码
	cryptoBytes := kiris.AESEncrypt(string(c), passwd, "cbc")
	// 签名
	cryptoHash := hash.Md5(string(cryptoBytes))
	// 保存
	content := hash.Md5(passwd+string(cryptoBytes)) + cryptoHash + string(cryptoBytes)
	return content
}

// 解码数据
func decryptData(file, passwd string) string {
	var c string
	if kiris.FileExists(file) {
		sc, err := kiris.FileGetContents(file)
		if err != nil {
			log.Fatal(err)
		}
		// 校验数据完整性
		if hash.Md5(string(sc[64:])) != string(sc[32:64]) {
			log.Fatal("The data signature has been modified.")
		}
		// 校验密码
		if hash.Md5(passwd+string(sc[64:])) != string(sc[:32]) {
			log.Fatal("Access denied for password. run assh account [your password]")
		}
		// 解码
		cryptoStr := string(sc[64:])
		c = kiris.AESDecrypt([]byte(cryptoStr), passwd, "cbc")
	}
	return c
}

func parseName(name string) (string, string) {
	keys := strings.Split(name, ".")
	if len(keys) < 2 {
		def := []string{""}
		keys = append(def, keys[0])
	}
	return keys[0], keys[1]
}

// 设置密钥
func setPemKey(server *Server) {
	if server.PemKey != "" && !kiris.FileExists(server.PemKey) {
		return
	}
	serverName := strings.Replace(server.Name, ".", "/", 0)
	pemKeyPath := strings.Join([]string{GetDbPath(), serverName}, "/")
	dstPemFile := path.Join(pemKeyPath, path.Base(server.PemKey))

	// copy ...
	os.Mkdir(pemKeyPath, os.ModePerm)
	CopyFile(server.PemKey, dstPemFile)

	// copy public key file
	pubFile := server.PemKey + ".pub"
	if kiris.FileExists(pubFile) {
		dstPubFile := dstPemFile + ".pub"
		CopyFile(pubFile, dstPubFile)
	}

	server.PemKey = dstPemFile
	return
}

// 删除密钥
func delPemKey(name string) {
	serverName := strings.Replace(name, ".", "/", 0)
	pemKeyPath := strings.Join([]string{GetDbPath(), serverName}, "/")

	if kiris.FileExists(pemKeyPath) {
		os.Remove(pemKeyPath)
	}
	return
}
