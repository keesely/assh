package vulnstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"assh/asshc/domain"
)

// Store 实现漏洞检查存储。
type Store struct {
	mu       sync.RWMutex
	basePath string
	index    *Index
}

// Index 表示存储索引。
type Index struct {
	Checks []IndexEntry `json:"checks"`
}

// IndexEntry 表示索引条目。
type IndexEntry struct {
	ID        string    `json:"id"`
	VulnID    string    `json:"vulnId"`
	Target    string    `json:"target"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Script    string    `json:"script"`
	Report    string    `json:"report"`
}

// NewStore 创建 Store 实例。
func NewStore(basePath string) (*Store, error) {
	// 确保目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	store := &Store{
		basePath: basePath,
		index:    &Index{},
	}

	// 加载索引
	if err := store.loadIndex(); err != nil {
		// 索引文件不存在，创建新的
		store.index = &Index{}
	}

	return store, nil
}

// Save 保存漏洞检查记录。
func (s *Store) Save(check *domain.VulnCheck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建检查目录
	checkDir := filepath.Join(s.basePath, check.VulnID)
	if err := os.MkdirAll(checkDir, 0755); err != nil {
		return fmt.Errorf("failed to create check directory: %w", err)
	}

	// 保存脚本
	if check.Script != "" {
		scriptPath := filepath.Join(checkDir, "script.sh")
		if err := os.WriteFile(scriptPath, []byte(check.Script), 0755); err != nil {
			return fmt.Errorf("failed to save script: %w", err)
		}
	}

	// 保存结果日志
	if check.Result != "" {
		logPath := filepath.Join(checkDir, "result.log")
		if err := os.WriteFile(logPath, []byte(check.Result), 0644); err != nil {
			return fmt.Errorf("failed to save result log: %w", err)
		}
	}

	// 保存报告
	if check.Report != "" {
		reportPath := filepath.Join(checkDir, "report.md")
		if err := os.WriteFile(reportPath, []byte(check.Report), 0644); err != nil {
			return fmt.Errorf("failed to save report: %w", err)
		}
	}

	// 更新索引
	entry := IndexEntry{
		ID:        check.ID,
		VulnID:    check.VulnID,
		Target:    check.Target,
		Status:    check.Status.String(),
		Timestamp: check.CreatedAt,
		Script:    filepath.Join(check.VulnID, "script.sh"),
		Report:    filepath.Join(check.VulnID, "report.md"),
	}

	// 检查是否已存在
	found := false
	for i, e := range s.index.Checks {
		if e.ID == check.ID {
			s.index.Checks[i] = entry
			found = true
			break
		}
	}

	if !found {
		s.index.Checks = append(s.index.Checks, entry)
	}

	// 保存索引
	return s.saveIndex()
}

// Get 获取漏洞检查记录。
func (s *Store) Get(id string) (*domain.VulnCheck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 从索引中查找
	for _, entry := range s.index.Checks {
		if entry.ID == id {
			return s.loadCheck(entry)
		}
	}

	return nil, fmt.Errorf("check not found: %s", id)
}

// List 列出所有漏洞检查记录。
func (s *Store) List() ([]*domain.VulnCheck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	checks := make([]*domain.VulnCheck, 0, len(s.index.Checks))
	for _, entry := range s.index.Checks {
		check, err := s.loadCheck(entry)
		if err != nil {
			continue
		}
		checks = append(checks, check)
	}

	return checks, nil
}

// ListByTarget 列出指定目标的检查记录。
func (s *Store) ListByTarget(target string) ([]*domain.VulnCheck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var checks []*domain.VulnCheck
	for _, entry := range s.index.Checks {
		if entry.Target == target {
			check, err := s.loadCheck(entry)
			if err != nil {
				continue
			}
			checks = append(checks, check)
		}
	}

	return checks, nil
}

// ListByVulnID 列出指定漏洞的检查记录。
func (s *Store) ListByVulnID(vulnID string) ([]*domain.VulnCheck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var checks []*domain.VulnCheck
	for _, entry := range s.index.Checks {
		if entry.VulnID == vulnID {
			check, err := s.loadCheck(entry)
			if err != nil {
				continue
			}
			checks = append(checks, check)
		}
	}

	return checks, nil
}

// Delete 删除漏洞检查记录。
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从索引中删除
	for i, entry := range s.index.Checks {
		if entry.ID == id {
			// 删除目录
			checkDir := filepath.Join(s.basePath, entry.VulnID)
			os.RemoveAll(checkDir)

			// 从索引中删除
			s.index.Checks = append(s.index.Checks[:i], s.index.Checks[i+1:]...)

			// 保存索引
			return s.saveIndex()
		}
	}

	return fmt.Errorf("check not found: %s", id)
}

// Update 更新漏洞检查记录。
func (s *Store) Update(check *domain.VulnCheck) error {
	return s.Save(check)
}

// loadCheck 从索引条目加载检查记录。
func (s *Store) loadCheck(entry IndexEntry) (*domain.VulnCheck, error) {
	check := &domain.VulnCheck{
		ID:        entry.ID,
		VulnID:    entry.VulnID,
		Target:    entry.Target,
		CreatedAt: entry.Timestamp,
		UpdatedAt: entry.Timestamp,
	}

	// 加载脚本
	scriptPath := filepath.Join(s.basePath, entry.Script)
	if data, err := os.ReadFile(scriptPath); err == nil {
		check.Script = string(data)
	}

	// 加载结果日志
	logPath := filepath.Join(s.basePath, entry.VulnID, "result.log")
	if data, err := os.ReadFile(logPath); err == nil {
		check.Result = string(data)
	}

	// 加载报告
	reportPath := filepath.Join(s.basePath, entry.Report)
	if data, err := os.ReadFile(reportPath); err == nil {
		check.Report = string(data)
	}

	return check, nil
}

// loadIndex 加载索引文件。
func (s *Store) loadIndex() error {
	indexPath := filepath.Join(s.basePath, "index.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, s.index)
}

// saveIndex 保存索引文件。
func (s *Store) saveIndex() error {
	indexPath := filepath.Join(s.basePath, "index.json")

	data, err := json.MarshalIndent(s.index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	return os.WriteFile(indexPath, data, 0644)
}

// GetBasePath 获取存储基础路径。
func (s *Store) GetBasePath() string {
	return s.basePath
}

// GetCheckPath 获取检查目录路径。
func (s *Store) GetCheckPath(vulnID string) string {
	return filepath.Join(s.basePath, vulnID)
}
