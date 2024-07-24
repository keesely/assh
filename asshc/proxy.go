// proxy.go kee > 2020/03/28

package asshc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

func socks5Proxy(conn net.Conn) {
	defer conn.Close()

	var b [1024]byte

	n, err := conn.Read(b[:])
	if err != nil {
		log.Println(err)
		return
	}

	conn.Write([]byte{0x05, 0x00})

	n, err = conn.Read(b[:])
	if err != nil {
		log.Println(err)
		return
	}

	var addr string
	switch b[3] {
	case 0x01:
		sip := sockIP{}
		if err := binary.Read(bytes.NewReader(b[4:n]), binary.BigEndian, &sip); err != nil {
			log.Println("请求解析错误")
			return
		}
		addr = sip.toAddr()
	case 0x03:
		host := string(b[5 : n-2])
		var port uint16
		err = binary.Read(bytes.NewReader(b[n-2:n]), binary.BigEndian, &port)
		if err != nil {
			log.Println(err)
			return
		}
		addr = fmt.Sprintf("%s:%d", host, port)
	}

	fmt.Println("proxy: ", addr)
	server, err := proxy.Dial("tcp", addr)
	if err != nil {
		log.Println(err)
		return
	}
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	go io.Copy(server, conn)
	io.Copy(conn, server)
}

type sockIP struct {
	A, B, C, D byte
	PORT       uint16
}

func (ip sockIP) toAddr() string {
	return fmt.Sprintf("%d.%d.%d.%d:%d", ip.A, ip.B, ip.C, ip.D, ip.PORT)
}

func socks5ProxyStart(host, port string) {
	log.SetFlags(log.Ltime | log.Lshortfile)

	server, err := net.Listen("tcp", host+":"+port)
	if err != nil {
		log.Panic(err)
	}
	defer server.Close()
	log.Println("开始接受连接")
	for {
		proxy, err := server.Accept()
		if err != nil {
			log.Println(err)
			return
		}
		//log.Println("一个新连接", proxy)
		go socks5Proxy(proxy)
	}
}

var proxy *ssh.Client

func (this *Server) Proxy(proxy_host, proxy_port string) {
	cnf, err := this.SSHConfig()
	proxy, err = ssh.Dial("tcp", cnf.Addr, cnf.Config)
	if err != nil {
		check(err, " assh > dial")
		log.Panic("Assh: Connection fail: unable to authenticate \n")
	}
	log.Println("连接服务器成功")
	defer proxy.Close()
	//proxy.Dial("tcp", fmt.Sprintf("%s:%d", proxy_host, proxy_port))
	log.Println("本地代理: " + proxy_host + ":" + proxy_port)
	socks5ProxyStart(proxy_host, proxy_port)
}

// 映射远程端口到本地端口
// 实现远程端口转发: ssh -f -N -L
func (this *Server) PortForwarding(local_host, local_port, remote_host, remote_port string) {
	cnf, err := this.SSHConfig()
	proxy, err = ssh.Dial("tcp", cnf.Addr, cnf.Config)
	if err != nil {
		check(err, " assh > dial")
		log.Panic("Assh: Connection fail: unable to authenticate \n")
	}
	log.Println("Connectioned")
	defer proxy.Close()
	//proxy.Dial("tcp", fmt.Sprintf("%s:%d", proxy_host, proxy_port))
	log.Println("Local Porxy " + local_host + ":" + local_port)
	//socks5ProxyStart(local_host, local_port)
	// 本地端口
	l, err := net.Listen("tcp", local_host+":"+local_port)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		// 等待连接
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			// 远程端口
			remote, err := proxy.Dial("tcp", remote_host+":"+remote_port)
			if err != nil {
				log.Fatal(err)
			}
			defer remote.Close()
			// 代理
			go io.Copy(remote, conn)
			io.Copy(conn, remote)
		}()
	}
}

// 映射远程端口到本地端口, 并根据DomainList进行域名过滤
// 实现远程端口转发: ssh -f -N -L
// func (this *Server) PortForwardingWithDomainList(local_host, local_port, remote_host, remote_port string, domainList []string) {
