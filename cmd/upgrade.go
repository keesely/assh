// upgrade.go kee > 2019/12/08

package cmd

import (
	"assh/asshc"
	"assh/log"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/keesely/kiris"
	"github.com/urfave/cli"
)

var (
	latestUrl = "https://api.github.com/repos/keesely/assh/releases/latest"
	project   = "assh"
)

type upgrade struct {
	Version string
	latest  githubReleases
}

type githubReleases struct {
	Name    string                 `json:"name"`
	TagName string                 `json:"tag_name"`
	Author  map[string]interface{} `json:"author"`
	Assets  []githubAssets         `json:"assets"`
}

type githubAssets struct {
	Url         string `json:"url"`
	Name        string `json:"name"`
	Label       string `json:"label"`
	Size        int    `json:"size"`
	DownloadURL string `json:"browser_download_url"`
}

func Version(c *cli.Context) (err error) {
	fmt.Println(version)
	return
}

func Upgrade(c *cli.Context) (err error) {
	var (
		wg   = sync.WaitGroup{}
		lock = true
	)

	up := &upgrade{Version: version}

	// 协程检测版本更新
	go func() {
		fmt.Print("upgrade checking..")
		for {
			if !lock {
				wg.Done()
				break
			}
			fmt.Print(".")
			time.Sleep(time.Second)
		}
	}()

	go func() {
		up.getLatestVersion()
		lock = false
		fmt.Println("")
		wg.Done()
	}()

	wg.Add(2)
	wg.Wait()

	fmt.Println("Current Version: ", version)
	fmt.Println("Latest Version: ", up.Latest().TagName)

	if 0 >= up.compareVersion() {
		fmt.Println("The current is the latest version.")
		return
	}

	var (
		src string
		dst = cwd()
	)
	//saveAs := kiris.SubStr(dst, 0, strings.LastIndex(dst, "/"))
	saveAs := os.TempDir()
	if src, err = up.Download(); err != nil {
		log.Panic("upgrade fail: ", err.Error())
		return
	}
	fmt.Println("download completed.")
	//fmt.Println("unzip :", src)
	if err = Unzip(src, saveAs); err != nil {
		log.Panic("upgrade unzip fail: ", err.Error())
	}
	fname := path.Base(src)
	fname = strings.Trim(fname, ".zip")
	src = path.Join(saveAs, fname, "assh")
	fmt.Printf("install as: %s -> %s\n", src, dst)
	if err = asshc.CopyFile(src, dst); err != nil {
		log.Panic("install fail: ", err.Error())
	}
	fmt.Println("Instalation successful.")
	//fmt.Printf("mv %s %s\n", dst+".bin", dst)
	return
}

func cwd() string {
	cwd, _ := exec.LookPath(os.Args[0])
	return strings.Replace(cwd, "\\", "/", -1)
}

func (up *upgrade) Latest() githubReleases {
	return up.latest
}

func (up *upgrade) Download() (string, error) {
	var (
		sysOS   = runtime.GOOS
		fsize   int64
		buf     = make([]byte, 32*1024)
		written int64
		fb      = func(length, downLen int64) {
			process := float64(downLen) / float64(length) * 100
			kb := float64(downLen) / 1024
			kbs := fmt.Sprintf("%.2fkb", kb)
			if kb > 1024 {
				kb = float64(kb) / 1024
				kbs = fmt.Sprintf("%.2f MB", kb)
			}
			fmt.Printf("\r downloading (%s/%.2f MB) %s ", kbs, float64(length)/1024/1024, fmt.Sprintf("%.2f", process)+"%")
		}
		err error
	)
	//创建一个http client
	client := new(http.Client)
	//get方法获取资源
	down := up.DownloadURL()
	if down == nil {
		log.Panicf("don't support system(%s) to download upgrade, please use the source code to compile", sysOS)
	}
	downPath := path.Join(os.TempDir(), path.Base(down.DownloadURL))
	resp, cErr := client.Get(down.DownloadURL)
	if cErr != nil {
		return "", cErr
	}
	//读取服务器返回的文件大小
	fsize, err = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return "", err
	}
	if kiris.FileExists(downPath) && fsize == int64(down.Size) {
		return downPath, nil
	}
	//创建文件
	file, err := os.Create(downPath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()
	if resp.Body == nil {
		log.Panic("body is nil")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	//下面是 io.copyBuffer() 的简化版本
	for {
		//读取bytes
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			//写入bytes
			nw, ew := file.Write(buf[0:nr])
			//数据长度大于0
			if nw > 0 {
				written += int64(nw)
			}
			//写入出错
			if ew != nil {
				err = ew
				break
			}
			//读取是数据长度不等于写入的数据长度
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
		fb(fsize, written)
	}
	return downPath, err
}

func (up *upgrade) DownloadURL() *githubAssets {
	os := runtime.GOOS
	if os == "darwin" {
		os = "macOS"
	}
	arch := runtime.GOARCH
	latest := up.latest
	ver := latest.TagName
	packName := fmt.Sprintf("%s-%s-%s_%s", project, os, arch, ver)

	for _, asset := range latest.Assets {
		if 1 != strings.Index(asset.Name, packName) {
			return &asset
		}
	}
	return nil
}

func (up *upgrade) getLatestVersion() {
	resp, err := http.Get(latestUrl)
	if err != nil {
		log.Panic("upgrade: ", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Panic("upgrade: ", err)
	}

	var latest githubReleases
	err = json.Unmarshal(body, &latest)
	if err != nil {
		log.Panic("upgrade: ", err)
	}
	up.latest = latest
}

func (up *upgrade) compareVersion() int {
	latest := up.latest.TagName
	cVersion := version
	latest = strings.Trim(latest, "v")
	cVersion = strings.Trim(cVersion, "v")

	lv := strings.Split(latest, ".")
	cv := strings.Split(cVersion, ".")

	lim := len(cv)
	if len(lv) > len(cv) {
		lim = len(lv)
	}

	for {
		if len(lv) > lim {
			break
		}
		lv = append(lv, "0")
	}
	for {
		if len(cv) > lim {
			break
		}
		cv = append(cv, "0")
	}

	for i := 0; i < lim; i++ {
		lvn, _ := strconv.Atoi(lv[i])
		cvn, _ := strconv.Atoi(cv[i])

		if lvn > cvn {
			return 1
		}
		if lvn < cvn {
			return -1
		}
	}
	return 0
}
