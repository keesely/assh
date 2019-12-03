// sftp.go kee > 2019/11/23

package asshc

import (
	"fmt"
	"github.com/keesely/kiris"
	"github.com/pkg/sftp"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

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

func (this *Server) ScpPushFiles(localList []string, remote string) error {
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
			err = pushDir(scp, local, remoteFile)
		} else {
			err = pushFile(scp, local, remoteFile)
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

func pushFile(scp *sftp.Client, local, remote string) error {
	src, err := os.Open(local)
	if err != nil {
		return fmt.Errorf("Assh: local file open fail: %s \n", err.Error())
	}
	defer src.Close()

	dst, err := scp.Create(remote)
	if err != nil {
		return fmt.Errorf("Assh: remote file create fail: %s \n", err.Error())
	}
	defer dst.Close()

	buf := make([]byte, 1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			break
		}
		dst.Write(buf[:n])
	}
	return nil
}

func pushDir(scp *sftp.Client, local, remote string) error {
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
			pushDir(scp, localFilePath, remoteFilePath)
		} else {
			pushFile(scp, localFilePath, remoteFilePath)
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

	if _, err := src.WriteTo(dst); err != nil {
		return err
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
