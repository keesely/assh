// Package qiniu 提供七牛云存储的上传与下载功能。
package qiniu

import (
	"context"
	"fmt"
	"github.com/qiniu/api.v7/v7/auth/qbox"
	"github.com/qiniu/api.v7/v7/storage"
)

// Qiniu 封装七牛云存储的客户端，提供文件上传和下载功能。
type Qiniu struct {
	accessKey, secretKey string // 七牛云 AccessKey 和 SecretKey
	bucket               string // 存储空间名称
	Mac                  *qbox.Mac // 七牛云认证 MAC 对象
}

// New 创建七牛云客户端实例，使用指定的 AccessKey、SecretKey 和存储空间。
func New(accessKey, secretKey, bucket string) *Qiniu {
	mac := qbox.NewMac(accessKey, secretKey)
	return &Qiniu{
		accessKey: accessKey,
		secretKey: secretKey,
		bucket:    bucket,
		Mac:       mac,
	}
}

// Upload 将本地文件上传到七牛云存储。
// src 为本地文件路径，key 为存储在云上的对象名称。
func (q *Qiniu) Upload(src string, key string) (err error) {
	keyToOverwrite := key
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", q.bucket, keyToOverwrite),
	}
	upToken := putPolicy.UploadToken(q.Mac)
	cfg := getCfg()
	_, e := GetUpHost(&cfg, q.accessKey, q.bucket)
	if e != nil {
		err = e
		return
	}

	// 构建表单上传对象
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}
	putExtra := storage.PutExtra{}
	err = formUploader.PutFile(context.Background(), &ret, upToken, key, src, &putExtra)
	return
}

// Download 从七牛云存储下载文件到本地。
// key 为云上的对象名称，dst 为本地保存路径。
func (q *Qiniu) Download(key string, dst string) (err error) {
	bm := GetBucketManager(q)
	err = bm.Get(q.bucket, key, dst)
	return
}
