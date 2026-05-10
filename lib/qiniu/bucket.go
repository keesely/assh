package qiniu

import (
	"context"
	"fmt"
	"github.com/qiniu/api.v7/v7/auth"
	"github.com/qiniu/api.v7/v7/auth/qbox"
	"github.com/qiniu/api.v7/v7/storage"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// GetRet 表示从七牛云获取文件信息时的响应数据。
type GetRet struct {
	URL      string `json:"url"`      // 文件下载 URL
	Hash     string `json:"hash"`     // 文件哈希值
	MimeType string `json:"mimeType"` // 文件 MIME 类型
	Fsize    int64  `json:"fsize"`    // 文件大小（字节）
	Expiry   int64  `json:"expires"`  // URL 过期时间
	Version  string `json:"version"`  // 文件版本
}

// BucketManager 封装七牛云存储空间管理操作。
type BucketManager struct {
	*storage.BucketManager
}

// zone 映射表，将区域名称映射到七牛云 SDK 的区域配置。
var zone = map[string]storage.Region{
	"HUADONG":  storage.ZoneHuadong,
	"HUABEI":   storage.ZoneHuabei,
	"HUANAN":   storage.ZoneHuanan,
	"BEIMEI":   storage.ZoneBeimei,
	"XINJIAPO": storage.ZoneXinjiapo,
}

// getCfg 返回默认的存储配置（HTTP，非 CDN）。
func getCfg() storage.Config {
	return storage.Config{
		UseHTTPS:      false,
		UseCdnDomains: false,
	}
}

// GetBucketManager 从 Qiniu 客户端创建 BucketManager。
func GetBucketManager(q *Qiniu) *BucketManager {
	cfg := getCfg()
	return NewBucketManager(q.Mac, &cfg)
}

// NewBucketManager 使用认证 MAC 和配置创建 BucketManager。
func NewBucketManager(mac *qbox.Mac, cfg *storage.Config) *BucketManager {
	bm := storage.NewBucketManager(mac, cfg)
	return &BucketManager{
		BucketManager: bm,
	}
}

// GetUpHost 获取存储空间的上传域名。
func GetUpHost(cfg *storage.Config, ak, bucket string) (upHost string, err error) {
	var zone *storage.Zone
	if cfg.Zone != nil {
		zone = cfg.Zone
	} else {
		zone, err = storage.GetZone(ak, bucket)
		cfg.Zone = zone
		if err != nil {
			return
		}
	}

	scheme := "http"
	if cfg.UseHTTPS {
		scheme = "https://"
	}

	host := zone.SrcUpHosts[0]
	if cfg.UseCdnDomains {
		host = zone.CdnUpHosts[0]
	}

	upHost = fmt.Sprintf("%s%s", scheme, host)
	return
}

// rsHost 获取存储空间的资源管理域名。
func (m *BucketManager) rsHost(bucket string) (rsHost string, err error) {
	zone, err := m.Zone(bucket)
	if err != nil {
		return
	}
	rsHost = zone.GetRsHost(m.Cfg.UseHTTPS)
	return
}

// Get 从七牛云存储下载指定文件到本地。
// 先获取文件的下载 URL，然后通过 HTTP 下载并写入本地文件。
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

	err = storage.DefaultClient.Call(ctx, &data, "GET", url, headers)
	if err != nil {
		return
	}
	resp, err := storage.DefaultClient.DoRequest(context.Background(), "GET", data.URL, headers)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if resp.StatusCode/100 != 2 {
		os.Exit(1)
	}

	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		return
	}

	defer f.Close()
	f.Write(body)
	return
}

// RsHost 返回资源管理域名。
func RsHost() string {
	return "rs.qiniu.com"
}

// ApiHost 返回 API 域名。
func ApiHost() string {
	return "api.qiniu.com"
}

// RsfHost 返回资源列表域名。
func RsfHost() string {
	return "rsf.qiniu.com"
}
