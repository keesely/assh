// bucket.go kee > 2019/11/29

package qiniu

import (
	"context"
	"github.com/qiniu/api.v7/v7/auth"
	"github.com/qiniu/api.v7/v7/auth/qbox"
	"github.com/qiniu/api.v7/v7/storage"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type GetRet struct {
	URL      string `json:"url"`
	Hash     string `json:"hash"`
	MimeType string `json:"mimeType"`
	Fsize    int64  `json:"fsize"`
	Expiry   int64  `json:"expires"`
	Version  string `json:"version"`
}

type BucketManager struct {
	*storage.BucketManager
}

func getCfg() storage.Config {
	return storage.Config{
		// 空间对应的机房
		Zone: &storage.ZoneHuanan,
		// 是否使用https域名
		UseHTTPS: false,
		// 上传是否使用CDN上传加速
		UseCdnDomains: false,
	}
}

func GetBucketManager(q *Qiniu) *BucketManager {
	cfg := getCfg()
	return NewBucketManager(q.Mac, &cfg)
}

func NewBucketManager(mac *qbox.Mac, cfg *storage.Config) *BucketManager {
	bm := storage.NewBucketManager(mac, cfg)
	return &BucketManager{
		BucketManager: bm,
	}
}

func (m *BucketManager) rsHost(bucket string) (rsHost string, err error) {
	zone, err := m.Zone(bucket)
	if err != nil {
		return
	}
	rsHost = zone.GetRsHost(m.Cfg.UseHTTPS)
	return
}

func (m *BucketManager) Get(bucket, key string, dst string) (err error) {
	entryUri := strings.Join([]string{bucket, key}, ":")

	var reqHost string

	reqHost, err = m.rsHost(bucket)
	if err != nil {
		return
	}
	if !strings.HasPrefix(reqHost, "http") {
		reqHost = "http://" + reqHost
	}
	url := strings.Join([]string{reqHost, "get", Encode(entryUri)}, "/")

	var data GetRet
	ctx := auth.WithCredentials(context.Background(), m.Mac)
	headers := http.Header{}

	//fmt.Printf("GET: %s\n\tHEADER: %v\n", url, headers)
	err = storage.DefaultClient.Call(ctx, &data, "GET", url, headers)
	if err != nil {
		return
	}
	resp, err := storage.DefaultClient.DoRequest(context.Background(), "GET", data.URL, headers)
	//fmt.Printf("RESPONSE > \n GET: %s\n\tHEADER: %v\n", data.URL, headers)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}
		os.Exit(1)
	}
	if strings.ContainsRune(dst, os.PathSeparator) {
		dst = filepath.Base(dst)
	}

	//f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		return
	}
	defer f.Close()

	io.Copy(f, resp.Body)
	return
}

func RsHost() string {
	return "rs.qiniu.com"
}

func ApiHost() string {
	return "api.qiniu.com"
}

func RsfHost() string {
	return "rsf.qiniu.com"
}
