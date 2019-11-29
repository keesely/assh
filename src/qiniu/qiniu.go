// qiniu.go kee > 2019/11/29

package qiniu

import (
	"context"
	"github.com/qiniu/api.v7/v7/auth/qbox"
	"github.com/qiniu/api.v7/v7/storage"
)

type Qiniu struct {
	accessKey, secretKey, bucket string
	Mac                          *qbox.Mac
}

func New(accessKey, secretKey, bucket string) *Qiniu {
	mac := qbox.NewMac(accessKey, secretKey)
	return &Qiniu{
		accessKey: accessKey,
		secretKey: secretKey,
		bucket:    bucket,
		Mac:       mac,
	}
}

func (q *Qiniu) Upload(src string, key string) (err error) {
	putPolicy := storage.PutPolicy{
		Scope: q.bucket,
	}
	upToken := putPolicy.UploadToken(q.Mac)
	cfg := getCfg()
	// 构建表单上传的对象
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}
	// 可选配置
	putExtra := storage.PutExtra{
		//Params: map[string]string{
		//"x:name": "kee topic",
		//},
	}
	err = formUploader.PutFile(context.Background(), &ret, upToken, key, src, &putExtra)
	return
}

func (q *Qiniu) Download(key string, dst string) (err error) {
	bm := GetBucketManager(q)
	err = bm.Get(q.bucket, key, dst)
	return
}
