// sftp.go kee > 2019/11/23

package asshc

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/keesely/kiris"
	"github.com/pkg/sftp"
)

var timeCost = func(s time.Time) {
	tc := time.Since(s)
	fmt.Printf("\n time cost: %v \n", tc)
}
var formatSize = func(bt float64) string {
	if bt > (1024 * 1024) {
		return fmt.Sprintf("%4.1f%3s", float64(bt/1024/1024), "MB")
	} else {
		return fmt.Sprintf("%4.f%3s", float64(bt/1024), "KiB")
	}
}
var formatTime = func(t int) string {
	if t >= 3600 {
		return fmt.Sprintf("%2dh%2dm%2ds", t/60/60, t/60%60, t%60)
	} else if t >= 60 {
		return fmt.Sprintf("%dm%ds", t/60, t%60)
	}
	return fmt.Sprintf("%ds", t)
}

func (this *Server) sftpClient() (*sftp.Client, error) {
	client, err := this.SSHClient()
	if err != nil {
		check(err, " assh > dial")
		return nil, fmt.Errorf("Assh: Connection fail: unable to authenticate \n")
	}

	c, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("Assh: connect sftp server fail: %s \n", err.Error())
	}
	return c, nil
}

func (this *Server) ScpPushFiles(localList []string, remote string, bufSize int) error {
	scp, err := this.sftpClient()
	if err != nil {
		return err
	}
	defer scp.Close()
	for n, local := range localList {
		local = kiris.RealPath(local)
		remote = getRemoteRealPath(remote)

		local = filepath.ToSlash(local)
		fname := path.Base(local)
		remoteFile := path.Join(remote, fname)
		fmt.Printf("%d: '%s' -> %s\n", n, local, fmt.Sprintf("%s@%s:%s", this.User, this.Host, remoteFile))
		var err error
		if kiris.IsDir(local) {
			err = pushDir(scp, local, remoteFile, bufSize)
		} else {
			err = pushFile(scp, local, remoteFile, bufSize)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *Server) ScpPullFiles(remoteList []string, local string) error {
	scp, err := this.sftpClient()
	if err != nil {
		return err
	}
	defer scp.Close()

	for n, remote := range remoteList {
		remote = getRemoteRealPath(remote)
		localFile := path.Join(local, path.Base(remote))
		var err error
		rf, err := scp.Stat(remote)
		if err != nil {
			return fmt.Errorf("Assh: remote file not found! %s \n", err.Error())
		}

		fmt.Printf("%d: '%s' -> %s\n", n, fmt.Sprintf("%s@%s:%s", this.User, this.Host, remote), localFile)
		if rf.IsDir() {
			err = pullDir(scp, remote, localFile)
		} else {
			err = pullFile(scp, remote, localFile)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func pushFile(scp *sftp.Client, local, remote string, bufSize int) error {
	defer timeCost(time.Now())

	src, err := os.Open(local)
	if err != nil {
		return fmt.Errorf("Assh: local file open fail: %s \n", err.Error())
	}
	defer src.Close()

	// 统计文件信息
	fi, _ := os.Stat(local)
	fsize := float64(fi.Size())
	tsize := formatSize(fsize)

	dst, err := scp.Create(remote)
	if err != nil {
		return fmt.Errorf("Assh: remote file create fail: %s \n", err.Error())
	}
	defer dst.Close()

	buf := make([]byte, bufSize)
	size := 0
	ss := time.Now()
	for {
		begin := time.Now()
		n, err := src.Read(buf)
		if err != nil {
			break
		}
		dst.Write(buf[:n])
		size += n

		// 计算每秒速率
		tc := float64(time.Since(begin)) / 1e9
		bc := float64(n)
		bc = bc / tc
		buf = make([]byte, int(bc))
		fmt.Printf("upload: %-100s \r", fmt.Sprintf("%s / %s (%s/s, %3.2f%%, need: %s / cost: %s)",
			formatSize(float64(size)),
			tsize,
			formatSize(float64(bc)),
			float64(float64(size)/fsize*100),
			formatTime(int((fsize-float64(size))/(bc))),
			formatTime((int(time.Since(ss))/1e9)),
		))
	}
	return nil
}

func pushDir(scp *sftp.Client, local, remote string, bufSize int) error {
	localFiles, err := ioutil.ReadDir(local)
	if err != nil {
		return fmt.Errorf("Assh: %s\n", err.Error())
	}
	scp.Mkdir(remote)
	for _, backupDir := range localFiles {
		localFilePath := path.Join(local, backupDir.Name())
		remoteFilePath := path.Join(remote, backupDir.Name())
		if backupDir.IsDir() {
			scp.Mkdir(remote)
			pushDir(scp, localFilePath, remoteFilePath, bufSize)
		} else {
			pushFile(scp, localFilePath, remoteFilePath, bufSize)
		}
	}
	return nil
}

func pullFile(scp *sftp.Client, remote, local string) error {
	src, err := scp.Open(remote)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(local)
	if err != nil {
		return err
	}
	defer dst.Close()

	defer timeCost(time.Now())
	//if _, err := src.WriteTo(dst); err != nil {
	//return err
	//}

	fi, _ := scp.Stat(remote)
	tfsize := float64(fi.Size())
	buf := make([]byte, 10240)
	size := 0
	ss := time.Now()
	for {
		begin := time.Now()
		n, err := src.Read(buf)

		dst.Write(buf[:n])
		size += n

		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}

		// 计算速率
		tc := float64(time.Since(begin)) / 1e9
		bc := float64(n)
		bc = bc / tc
		buf = make([]byte, int(bc))

		fmt.Printf("download: %-100s \r", fmt.Sprintf("%s / %s (%s/s, %3.2f%%, need: %s / cost: %s)",
			formatSize(float64(size)),
			formatSize(tfsize),
			formatSize(float64(bc)),
			float64(float64(size)/tfsize*100),
			formatTime(int((tfsize-float64(size))/(bc))),
			formatTime(int(time.Since(ss))/1e9),
		))
	}
	return nil
}

func pullDir(scp *sftp.Client, remote, local string) error {
	remoteFiles, err := scp.ReadDir(remote)
	if err != nil {
		return fmt.Errorf("Assh: %s\n", err.Error())
	}
	os.Mkdir(local, os.ModePerm)
	for _, backupDir := range remoteFiles {
		localFilePath := path.Join(local, backupDir.Name())
		remoteFilePath := path.Join(remote, backupDir.Name())
		if backupDir.IsDir() {
			os.Mkdir(localFilePath, os.ModePerm)
			pullDir(scp, remoteFilePath, localFilePath)
		} else {
			pullFile(scp, remoteFilePath, localFilePath)
		}
	}
	return nil
}

func getRemoteRealPath(remote string) string {
	remoteByte := []rune(remote)
	if string(remoteByte[:1]) == "~" {
		remote = "." + string(remoteByte[1:])
	}
	return remote
}

// 挂载远程目录到本地
func mountRemoteDir(scp *sftp.Client, remote, local string) error {
	defer timeCost(time.Now())
	remote = getRemoteRealPath(remote)
	local = getRemoteRealPath(local)
	// 检查本地目录是否存在
	if _, err := os.Stat(local); os.IsNotExist(err) {
		return fmt.Errorf("Assh: local dir not exist: %s \n", local)
	}
	// 检查远程目录是否存在
	if _, err := scp.Stat(remote); os.IsNotExist(err) {
		return fmt.Errorf("Assh: remote dir not exist: %s \n", remote)
	}
	// 挂载远程目录到本地
	if err := scp.MkdirAll(local); err != nil {
		return fmt.Errorf("Assh: local dir create fail: %s \n", err.Error())
	}

	return nil
}
