// Package service 实现应用核心业务逻辑，协调 port 接口和领域对象。
package service

import (
	"context"
	"fmt"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// SyncService 提供云同步相关业务逻辑的编排。
//
// 负责：
//   - 云账户管理（添加、查询、删除）
//   - 数据同步（推送、拉取）
//   - 冲突处理（保留双方策略）
//   - 同步历史记录
type SyncService struct {
	syncer port.Syncer
	store  SyncStore
	now    func() time.Time // 可注入的时间函数，便于测试
}

// SyncStore 定义 SyncService 所需的存储接口。
//
// 该接口组合了云账户管理、服务器配置管理和同步历史记录的能力。
// 由 store.Store 提供具体实现。
type SyncStore interface {
	// 账户管理
	GetAccount() (*domain.CloudAccount, error)
	ListAccounts() ([]*domain.CloudAccount, error)
	SetAccount(acct *domain.CloudAccount) error
	DeleteAccount() error

	// 服务器管理（复用已有接口）
	List() (map[string]map[string]*domain.Server, error)
	Set(name string, server *domain.Server) error

	// 同步历史
	RecordSyncHistory(history *domain.SyncHistory) error
	GetSyncHistory(limit int) ([]*domain.SyncHistory, error)
	GetLatestSyncTimestamp() (time.Time, error)
}

// NewSyncService 创建 SyncService 实例。
//
// syncer 是云存储服务的适配器实现。
// store 提供对数据库的读写能力（账户+服务器+同步历史）。
func NewSyncService(syncer port.Syncer, store SyncStore) *SyncService {
	return &SyncService{
		syncer: syncer,
		store:  store,
		now:    time.Now,
	}
}

// GetAccount 获取当前云账户配置。
//
// 返回默认云账户信息，如果尚未配置返回 domain.ErrAccountNotFound。
func (s *SyncService) GetAccount() (*domain.CloudAccount, error) {
	return s.store.GetAccount()
}

// SetAccount 配置云账户信息。
//
// 保存云账户凭证和存储配置。SecretKey 会在存储层自动加密。
func (s *SyncService) SetAccount(acct *domain.CloudAccount) error {
	if acct.AccessKey == "" || acct.SecretKey == "" {
		return fmt.Errorf("access key and secret key are required")
	}
	if acct.Bucket == "" {
		return fmt.Errorf("bucket name is required")
	}
	if acct.Type == "" {
		acct.Type = "qiniu"
	}
	if acct.Zone == "" {
		acct.Zone = "huadong"
	}

	return s.store.SetAccount(acct)
}

// DeleteAccount 删除云账户配置。
func (s *SyncService) DeleteAccount() error {
	return s.store.DeleteAccount()
}

// TestAccount 测试云账户连接是否正常。
//
// 检查账户是否存在，凭据是否完整，并尝试连接到云存储服务。
func (s *SyncService) TestAccount() error {
	acct, err := s.store.GetAccount()
	if err != nil {
		return fmt.Errorf("cannot test account: %w", err)
	}

	// 创建并登录云存储客户端
	syncer := s.syncer
	if err := syncer.Login(context.Background(), s.toPortAccount(acct)); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	return nil
}

// Push 将本地服务器配置推送到云端。
//
// 同步策略：
//   - 首次同步：全量推送所有服务器配置
//   - 增量同步：仅推送上次同步后有变更的服务器
//
// 冲突处理：推送以本地版本为准，覆盖云端。
func (s *SyncService) Push(ctx context.Context) (*port.SyncResult, error) {
	// 1. 获取云账户并登录
	acct, err := s.store.GetAccount()
	if err != nil {
		return nil, fmt.Errorf("push failed: %w", err)
	}

	if err := s.syncer.Login(ctx, s.toPortAccount(acct)); err != nil {
		return nil, fmt.Errorf("push failed: login error: %w", err)
	}

	// 2. 获取所有服务器配置
	serverMap, err := s.store.List()
	if err != nil {
		return nil, fmt.Errorf("push failed: list servers error: %w", err)
	}

	// 3. 构建同步数据
	serverData := make(map[string]*port.ServerData)
	for group, groupServers := range serverMap {
		for _, svr := range groupServers {
			name := domain.JoinName(group, svr.Name)
			serverData[name] = &port.ServerData{
				Name:    svr.Name,
				Group:   svr.Group,
				Host:    svr.Host,
				Port:    svr.Port,
				User:    svr.User,
				Password: s.getPassword(svr),
				KeyFile: s.getKeyFile(svr),
				Remark:  svr.Remark,
				Options: svr.Options,
				Version: svr.Version,
			}
		}
	}

	// 4. 检查是否增量同步
	// 当前实现使用全量推送策略（数据量小，服务器配置通常不超过数百条）。
	// 后续可优化为：读取上次同步时间戳，只推送版本号更新的服务器。

	syncData := &port.SyncData{
		Version:   "1.0",
		Timestamp: s.now(),
		Servers:   serverData,
	}

	// 5. 执行推送
	result, err := s.syncer.Push(ctx, syncData)
	if err != nil {
		// 记录失败历史
		_ = s.store.RecordSyncHistory(&domain.SyncHistory{
			Direction: domain.SyncDirectionPush,
			Status:    domain.SyncStatusFailed,
			Message:   err.Error(),
			Timestamp: s.now(),
		})
		return nil, fmt.Errorf("push failed: %w", err)
	}

	// 6. 记录成功历史
	_ = s.store.RecordSyncHistory(&domain.SyncHistory{
		Direction: domain.SyncDirectionPush,
		Status:    domain.SyncStatusSuccess,
		Message:   result.Message,
		Pushed:    len(serverData),
		Timestamp: s.now(),
	})

	return result, nil
}

// Pull 从云端拉取服务器配置到本地。
//
// 同步策略：
//   - 增量合并：对比本地和云端数据，合并差异
//   - 冲突处理：保留双方（云端版本添加 "_cloud_<ts>" 后缀）
//
// 返回值为本次拉取的同步结果，包含新增、更新和冲突的服务器数量。
func (s *SyncService) Pull(ctx context.Context) (*port.SyncResult, error) {
	// 1. 获取云账户并登录
	acct, err := s.store.GetAccount()
	if err != nil {
		return nil, fmt.Errorf("pull failed: %w", err)
	}

	if err := s.syncer.Login(ctx, s.toPortAccount(acct)); err != nil {
		return nil, fmt.Errorf("pull failed: login error: %w", err)
	}

	// 2. 从云端拉取数据
	cloudData, err := s.syncer.Pull(ctx)
	if err != nil {
		_ = s.store.RecordSyncHistory(&domain.SyncHistory{
			Direction: domain.SyncDirectionPull,
			Status:    domain.SyncStatusFailed,
			Message:   err.Error(),
			Timestamp: s.now(),
		})
		return nil, fmt.Errorf("pull failed: %w", err)
	}

	// 云端没有数据
	if cloudData == nil || len(cloudData.Servers) == 0 {
		result := &port.SyncResult{
			Success:   true,
			Message:   "no data on cloud",
			Pushed:    0,
			Updated:   0,
			Conflicts: 0,
			Timestamp: s.now(),
		}

		_ = s.store.RecordSyncHistory(&domain.SyncHistory{
			Direction: domain.SyncDirectionPull,
			Status:    domain.SyncStatusSuccess,
			Message:   "no data on cloud",
			Timestamp: s.now(),
		})

		return result, nil
	}

	// 3. 获取本地所有服务器
	localMap, err := s.store.List()
	if err != nil {
		return nil, fmt.Errorf("pull failed: list local servers error: %w", err)
	}

	// 展平为 fullName -> server 映射
	flatLocalMap := make(map[string]*domain.Server)
	for group, servers := range localMap {
		for _, svr := range servers {
			name := domain.JoinName(group, svr.Name)
			flatLocalMap[name] = svr
		}
	}

	// 4. 合并数据
	var added int
	var updated int
	var conflicts int

	for fullName, cloudSvr := range cloudData.Servers {
		localSvr, exists := flatLocalMap[fullName]

		if !exists {
			// 云端新增服务器，添加到本地
			server := &domain.Server{
				Name:    cloudSvr.Name,
				Group:   cloudSvr.Group,
				Host:    cloudSvr.Host,
				Port:    cloudSvr.Port,
				User:    cloudSvr.User,
				Remark:  cloudSvr.Remark,
				Options: cloudSvr.Options,
				Auth: &domain.Auth{
					Password: cloudSvr.Password,
					KeyFile:  cloudSvr.KeyFile,
				},
			}
			if err := s.store.Set(fullName, server); err != nil {
				return nil, fmt.Errorf("pull failed: add server %s: %w", fullName, err)
			}
			added++
		} else {
			// 服务器在本地也存在，使用版本号比较
			cloudVersion := cloudSvr.Version
			localVersion := localSvr.Version

			if cloudVersion > localVersion {
				// 云端版本更新，更新本地
				server := &domain.Server{
					Name:    cloudSvr.Name,
					Group:   cloudSvr.Group,
					Host:    cloudSvr.Host,
					Port:    cloudSvr.Port,
					User:    cloudSvr.User,
					Remark:  cloudSvr.Remark,
					Options: cloudSvr.Options,
					Auth: &domain.Auth{
						Password: cloudSvr.Password,
						KeyFile:  cloudSvr.KeyFile,
					},
				}
				if err := s.store.Set(fullName, server); err != nil {
					return nil, fmt.Errorf("pull failed: update server %s: %w", fullName, err)
				}
				updated++
			} else if localVersion > cloudVersion {
				// 本地版本更新 → 保留双方策略
				now := s.now()
				ts := now.Format("20060102-150405")
				conflictName := fullName + "_cloud_" + ts
				server := &domain.Server{
					Name:    conflictName,
					Group:   "",
					Host:    cloudSvr.Host,
					Port:    cloudSvr.Port,
					User:    cloudSvr.User,
					Remark:  cloudSvr.Remark + " (conflict from cloud, " + now.Format("2006-01-02 15:04:05") + ")",
					Options: cloudSvr.Options,
					Auth: &domain.Auth{
						Password: cloudSvr.Password,
						KeyFile:  cloudSvr.KeyFile,
					},
				}
				if err := s.store.Set(conflictName, server); err != nil {
					// 冲突名可能已存在，尝试使用不同的后缀
					conflictName = fullName + "_cloud_" + ts + "_" + fmt.Sprintf("%d", now.UnixNano()%1000)
					server.Name = conflictName
					if err := s.store.Set(conflictName, server); err != nil {
						return nil, fmt.Errorf("pull failed: add conflict server %s: %w", conflictName, err)
					}
				}
				conflicts++
			}
			// 版本号相同 → 跳过（无变更）
		}
	}

	// 5. 记录同步历史
	result := &port.SyncResult{
		Success:   true,
		Message:   fmt.Sprintf("added: %d, updated: %d, conflicts: %d", added, updated, conflicts),
		Pushed:    0,
		Updated:   updated + added,
		Conflicts: conflicts,
		Timestamp: s.now(),
	}

	status := domain.SyncStatusSuccess
	if conflicts > 0 {
		status = domain.SyncStatusPartial
	}

	_ = s.store.RecordSyncHistory(&domain.SyncHistory{
		Direction: domain.SyncDirectionPull,
		Status:    status,
		Message:   result.Message,
		Pushed:    added,
		Updated:   updated,
		Conflicts: conflicts,
		Timestamp: s.now(),
	})

	return result, nil
}

// GetSyncHistory 获取同步历史记录列表。
func (s *SyncService) GetSyncHistory(limit int) ([]*domain.SyncHistory, error) {
	return s.store.GetSyncHistory(limit)
}

// getPassword 从 Server 中提取密码。
func (s *SyncService) getPassword(svr *domain.Server) string {
	if svr.Auth != nil {
		return svr.Auth.Password
	}
	return ""
}

// getKeyFile 从 Server 中提取密钥文件路径。
func (s *SyncService) getKeyFile(svr *domain.Server) string {
	if svr.Auth != nil {
		return svr.Auth.KeyFile
	}
	return ""
}

// toPortAccount 将领域 CloudAccount 转换为 port.SyncAccount。
func (s *SyncService) toPortAccount(acct *domain.CloudAccount) port.SyncAccount {
	return port.SyncAccount{
		Type:      acct.Type,
		AccessKey: acct.AccessKey,
		SecretKey: acct.SecretKey,
		Bucket:    acct.Bucket,
		Zone:      acct.Zone,
	}
}
