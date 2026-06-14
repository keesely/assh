package store

import (
	"database/sql"
	"time"

	"assh/asshc/port"
)

// selectMountCols 定义挂载表查询的基础列名。
const selectMountCols = `id, server_name, remote_path, mount_point, mount_type,
	local_port, daemon, pid, options, created_at, updated_at`

// MountStoreImpl 实现 port.MountStore 接口。
type MountStoreImpl struct {
	store *Store
}

// NewMountStoreImpl 创建 MountStoreImpl 实例。
func NewMountStoreImpl(store *Store) *MountStoreImpl {
	return &MountStoreImpl{store: store}
}

// 编译期接口合规检查
var _ port.MountStore = (*MountStoreImpl)(nil)

// Save 保存挂载信息到数据库。
// 如果挂载点已存在则更新，否则插入新记录。
func (ms *MountStoreImpl) Save(info port.MountInfo) error {
	ms.store.mu.Lock()
	defer ms.store.mu.Unlock()

	// 序列化 options（使用空 map，因为 MountInfo 没有 ExtraOpts 字段）
	optsJSON := []byte("{}")

	// 检查是否已存在
	var existing int64
	err := ms.store.db.QueryRow(`SELECT id FROM mounts WHERE mount_point = ?`, info.MountPoint).Scan(&existing)
	if err == nil {
		// 已存在，更新
		_, err = ms.store.db.Exec(`
			UPDATE mounts SET
				server_name = ?, remote_path = ?, mount_type = ?,
				local_port = ?, daemon = ?, pid = ?, options = ?,
				updated_at = datetime('now')
			WHERE mount_point = ?`,
			info.ServerName, info.RemotePath, string(info.MountType),
			info.LocalPort, boolToInt(info.Daemon), info.PID, string(optsJSON),
			info.MountPoint,
		)
		return err
	}

	// 不存在，插入
	_, err = ms.store.db.Exec(`
		INSERT INTO mounts (server_name, remote_path, mount_point, mount_type,
			local_port, daemon, pid, options, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		info.ServerName, info.RemotePath, info.MountPoint, string(info.MountType),
		info.LocalPort, boolToInt(info.Daemon), info.PID, string(optsJSON),
	)
	return err
}

// DeleteByMountPoint 按挂载点删除挂载记录。
func (ms *MountStoreImpl) DeleteByMountPoint(mountPoint string) error {
	ms.store.mu.Lock()
	defer ms.store.mu.Unlock()

	_, err := ms.store.db.Exec(`DELETE FROM mounts WHERE mount_point = ?`, mountPoint)
	return err
}

// DeleteByServer 按服务器名删除所有挂载记录。
func (ms *MountStoreImpl) DeleteByServer(serverName string) error {
	ms.store.mu.Lock()
	defer ms.store.mu.Unlock()

	_, err := ms.store.db.Exec(`DELETE FROM mounts WHERE server_name = ?`, serverName)
	return err
}

// List 列出所有挂载信息。
func (ms *MountStoreImpl) List() ([]port.MountInfo, error) {
	ms.store.mu.RLock()
	defer ms.store.mu.RUnlock()

	rows, err := ms.store.db.Query(`SELECT ` + selectMountCols + ` FROM mounts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer closeAndLog(rows, "mounts rows")

	var mounts []port.MountInfo
	for rows.Next() {
		info, err := scanMount(rows)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, *info)
	}
	return mounts, rows.Err()
}

// GetByMountPoint 按挂载点查询挂载信息。
func (ms *MountStoreImpl) GetByMountPoint(mountPoint string) (*port.MountInfo, error) {
	ms.store.mu.RLock()
	defer ms.store.mu.RUnlock()

	rows, err := ms.store.db.Query(`SELECT `+selectMountCols+` FROM mounts WHERE mount_point = ?`, mountPoint)
	if err != nil {
		return nil, err
	}
	defer closeAndLog(rows, "mounts rows")

	if rows.Next() {
		return scanMount(rows)
	}
	return nil, sql.ErrNoRows
}

// scanMount 从数据库行扫描挂载信息。
func scanMount(rows *sql.Rows) (*port.MountInfo, error) {
	var info port.MountInfo
	var mountType string
	var daemonInt int
	var optionsJSON string
	var createdAt, updatedAt string

	err := rows.Scan(
		&info.ID, &info.ServerName, &info.RemotePath, &info.MountPoint,
		&mountType, &info.LocalPort, &daemonInt, &info.PID,
		&optionsJSON, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	info.MountType = port.MountType(mountType)
	info.Daemon = daemonInt != 0

	// 解析时间
	if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
		info.MountedAt = t
	}

	return &info, nil
}
