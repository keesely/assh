package asshc

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// 1.通过ssh连接远程服务器
// 2.在远程服务器上启动一个tcp服务
// 3.当有请求到达时，将请求转发到本地的一个tcp服务
// 4.将本地tcp服务的响应转发到远程服务器
func (this *Server) LocalProxy(remote_host, remote_port, local_host, local_port string) {
	if "" == remote_host {
		remote_host = "0.0.0.0"
	}
	if "" == remote_port {
		log.Fatal("remote_port is empty")
	}
	if "" == local_host {
		local_host = "127.0.0.1"
	}
	if "" == local_port {
		log.Fatal("local_port is empty")
	}

	cnf, err := this.SSHConfig()
	log.Println("SSH Connecting to", cnf.Addr)
	proxy, err := ssh.Dial("tcp", cnf.Addr, cnf.Config)
	if err != nil {
		check(err, " assh > dial")
		log.Fatal("Assh: Connection fail: unable to authenticate \n")
	}
	defer proxy.Close()

	log.Println("SSH Connectioned.")

	// 在远程服务器上启动一个tcp服务并且监听
	log.Println("Listening on", remote_host+":"+remote_port)
	reverse, err := proxy.Listen("tcp", remote_host+":"+remote_port)
	if err != nil {
		log.Fatal("unable to register tcp forward: ", err)
		//panic(err)
	}
	defer reverse.Close()

	for {
		log.Println("Waiting for connection by ", remote_host+":"+remote_port)
		conn, err := reverse.Accept()
		log.Println("Connected from", conn.RemoteAddr())
		if err != nil {
			log.Println("Accept fail")
			panic(err)
		}

		go func() {
			defer conn.Close()
			local_addr := local_host + ":" + local_port
			local, err := net.Dial("tcp", local_addr)
			if err != nil {
				log.Println(err)
				return
			}
			defer local.Close()

			go io.Copy(local, conn)
			io.Copy(conn, local)
		}()
	}
}

// 解析端口
// 取值范围从1到65535；
// 支持输入多个端口范围，以","隔开
// 范围端口使用"-"分隔：例如"1-1024"
func parsePorts(ports string) ([]string, error) {
	if "" == ports {
		return nil, fmt.Errorf("ports is empty")
	}
	out := []string{}

	if strings.Contains(ports, ",") {
		ports = strings.Replace(ports, ",", " ", -1)
		out = strings.Fields(ports)
	}
	if strings.Contains(ports, "-") {
		begin := strings.Split(ports, "-")[0]
		end := strings.Split(ports, "-")[1]
		beginInt, e := strconv.Atoi(begin)
		if e != nil {
			log.Fatal("begin port is not a number")
		}
		endInt, e := strconv.Atoi(end)
		if e != nil {
			log.Fatal("end port is not a number")
		}

		for i := beginInt; i <= endInt; i++ {
			out = append(out, strconv.Itoa(i))
		}

	}
	return out, nil
}
