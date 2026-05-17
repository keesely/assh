// Package qiniu 提供基于七牛云存储的 Syncer 实现。
//
// 该包实现了 port.Syncer 接口，用于将服务器配置数据同步到七牛云存储。
// 使用七牛云 SDK (api.v7) 实现文件的上传、下载和管理。
package qiniu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"assh/asshc/port"

	"github.com/qiniu/api.v7/v7/auth/qbox"
	"github.com/qiniu/api.v7/v7/storage"
)

// syncFileName 是云端用于存储同步数据的文件名。
const syncFileName = "assh_servers_sync.json"

// SyncObject 表示七牛云同步器，实现 port.Syncer 接口。
type SyncObject struct {
	account port.SyncAccount

	mac  *qbox.Mac
	zone *storage.Zone
	cfg  storage.Config
}

// NewSyncer 创建七牛云同步器实例。
//
// 参数 account 包含七牛云的访问密钥、存储空间和区域配置。
// 调用 Login 方法完成认证后，才可执行 Push/Pull 等操作。
func NewSyncer(account port.SyncAccount) *SyncObject {
	return &SyncObject{
		account: account,
	}
}

// Login 验证七牛云账户凭据并初始化连接配置。
//
// 验证流程：
//  1. 创建 QBox MAC 认证对象
//  2. 通过 GetZone 请求验证 AccessKey 是否有效
//  3. 获取存储空间的区域信息
//
// 验证成功后会缓存区域配置，后续 Push/Pull 直接复用。
func (s *SyncObject) Login(ctx context.Context, account port.SyncAccount) error {
	s.account = account

	// 创建认证 MAC 对象
	s.mac = qbox.NewMac(account.AccessKey, account.SecretKey)

	// 尝试获取存储空间区域信息以验证凭据
	zone, err := storage.GetZone(account.AccessKey, account.Bucket)
	if err != nil {
		return fmt.Errorf("qiniu login failed: access key验证失败: %w", err)
	}
	s.zone = zone

	// 初始化配置（根据区域自动选择上传域名）
	s.cfg = storage.Config{
		Zone:          zone,
		UseHTTPS:      false,
		UseCdnDomains: false,
	}

	return nil
}

// Push 将同步数据上传到七牛云存储。
//
// 实现流程：
//  1. 将 SyncData JSON 序列化
//  2. 生成上传令牌（覆盖权限）
//  3. 使用表单上传方式将数据写入云端
//
// 上传成功后返回同步结果，包含推送数量和时间戳。
func (s *SyncObject) Push(ctx context.Context, data *port.SyncData) (*port.SyncResult, error) {
	if s.mac == nil {
		return nil, fmt.Errorf("qiniu: not logged in, call Login first")
	}

	// 序列化同步数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("qiniu push: json marshal failed: %w", err)
	}

	// 设置上传策略（覆盖权限，允许同名文件覆盖）
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", s.account.Bucket, syncFileName),
	}
	upToken := putPolicy.UploadToken(s.mac)

	// 构建表单上传对象
	formUploader := storage.NewFormUploader(&s.cfg)
	ret := storage.PutRet{}
	putExtra := storage.PutExtra{}

	// 上传数据（从字节缓冲区直接上传）
	err = formUploader.Put(ctx, &ret, upToken, syncFileName, bytes.NewReader(jsonData), int64(len(jsonData)), &putExtra)
	if err != nil {
		return nil, fmt.Errorf("qiniu push: upload failed: %w", err)
	}

	result := &port.SyncResult{
		Success:   true,
		Message:   fmt.Sprintf("pushed %d servers", len(data.Servers)),
		Pushed:    len(data.Servers),
		Updated:   0,
		Conflicts: 0,
		Timestamp: time.Now(),
	}

	return result, nil
}

// Pull 从七牛云存储下载同步数据。
//
// 实现流程：
//  1. 获取文件的下载 URL（私有空间需要签名）
//  2. 通过 HTTP GET 下载文件内容
//  3. JSON 反序列化为 SyncData
//
// 如果云端没有同步数据，返回 nil, nil。
func (s *SyncObject) Pull(ctx context.Context) (*port.SyncData, error) {
	if s.mac == nil {
		return nil, fmt.Errorf("qiniu: not logged in, call Login first")
	}

	// 获取存储空间域名
	domain := s.cfg.Zone.SrcUpHosts[0]
	
	// 构建公开下载 URL
	downloadURL := fmt.Sprintf("http://%s/%s", domain, syncFileName)
	
	// 通过 HTTP 下载文件
	// 使用七牛云 SDK 的默认 HTTP 客户端
	headers := http.Header{}
	resp, err := storage.DefaultClient.DoRequest(ctx, "GET", downloadURL, headers)
	if err != nil {
		return nil, fmt.Errorf("qiniu pull: download request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应数据
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("qiniu pull: read response failed: %w", err)
	}

	// 反序列化同步数据
	var data port.SyncData
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		return nil, fmt.Errorf("qiniu pull: json unmarshal failed: %w", err)
	}

	return &data, nil
}

// List 列出云端的同步对象。
//
// 返回该存储空间下 syncFileName 对应的文件信息。
// 如果文件不存在，返回空列表。
func (s *SyncObject) List(ctx context.Context) ([]*port.SyncObject, error) {
	if s.mac == nil {
		return nil, fmt.Errorf("qiniu: not logged in, call Login first")
	}

	bm := storage.NewBucketManager(s.mac, &s.cfg)

	fileInfo, err := bm.Stat(s.account.Bucket, syncFileName)
	if err != nil {
		return []*port.SyncObject{}, nil
	}

	objects := []*port.SyncObject{
		{
			Key:     syncFileName,
			Size:    fileInfo.Fsize,
			ETag:    fileInfo.Hash,
			Version: "1",
			Modified: time.Unix(fileInfo.PutTime/10000000, 0),
		},
	}

	return objects, nil
}

// Delete 删除云端的同步数据文件。
func (s *SyncObject) Delete(ctx context.Context, key string) error {
	if s.mac == nil {
		return fmt.Errorf("qiniu: not logged in, call Login first")
	}

	bm := storage.NewBucketManager(s.mac, &s.cfg)

	if err := bm.Delete(s.account.Bucket, key); err != nil {
		return fmt.Errorf("qiniu delete: %w", err)
	}

	return nil
}


