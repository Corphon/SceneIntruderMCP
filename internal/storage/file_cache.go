// internal/storage/file_cache.go
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FileCacheService 提供文件内存缓存功能
type FileCacheService struct {
	cache      map[string]*FileCacheEntry
	mutex      sync.RWMutex
	maxSize    int           // 最大缓存条目数
	expiration time.Duration // 缓存过期时间
}

// FileCacheEntry 缓存条目
type FileCacheEntry struct {
	Data      interface{}
	CreatedAt time.Time
	LastRead  time.Time
	FileInfo  os.FileInfo // 用于检测文件是否被修改
}

// NewFileCacheService 创建文件缓存服务
func NewFileCacheService(maxSize int, expiration time.Duration) *FileCacheService {
	if maxSize <= 0 {
		maxSize = 1000 // 默认缓存1000个条目
	}

	if expiration <= 0 {
		expiration = 5 * time.Minute // 默认5分钟过期
	}

	return &FileCacheService{
		cache:      make(map[string]*FileCacheEntry),
		mutex:      sync.RWMutex{},
		maxSize:    maxSize,
		expiration: expiration,
	}
}

// ReadFile 读取文件并缓存
func (s *FileCacheService) ReadFile(path string, target interface{}) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("获取文件绝对路径失败: %w", err)
	}

	// 检查缓存
	s.mutex.RLock()
	entry, exists := s.cache[absPath]
	s.mutex.RUnlock()

	if exists {
		// 获取文件信息以检查是否被修改
		fileInfo, err := os.Stat(absPath)
		if err == nil {
			// 检查文件是否被修改以及是否过期
			isModified := fileInfo.ModTime().After(entry.FileInfo.ModTime()) ||
				fileInfo.Size() != entry.FileInfo.Size()
			isExpired := time.Since(entry.CreatedAt) > s.expiration

			if !isModified && !isExpired {
				// 缓存有效，更新最后读取时间并返回缓存数据
				s.mutex.Lock()
				entry.LastRead = time.Now()
				s.mutex.Unlock()

				// 将缓存数据转换为目标类型
				data, err := json.Marshal(entry.Data)
				if err == nil {
					return json.Unmarshal(data, target)
				}
			}
		}
	}

	// 缓存无效或不存在，读取文件
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 解析文件内容
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	// 获取文件信息以用于后续检查
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		// 不阻止操作，仅记录错误
		fmt.Printf("警告: 获取文件信息失败: %v\n", err)
	} else {
		// 保存到缓存
		s.mutex.Lock()
		s.cache[absPath] = &FileCacheEntry{
			Data:      target,
			CreatedAt: time.Now(),
			LastRead:  time.Now(),
			FileInfo:  fileInfo,
		}

		// 如果缓存太大，清理最少使用的条目
		if len(s.cache) > s.maxSize {
			s.cleanupLRU(s.maxSize / 5) // 清理20%
		}
		s.mutex.Unlock()
	}

	return nil
}

// WriteFile 写入文件并更新缓存
func (s *FileCacheService) WriteFile(path string, data interface{}) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("获取文件绝对路径失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 序列化数据
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化JSON失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(absPath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	// 获取文件信息
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		fmt.Printf("警告: 获取文件信息失败: %v\n", err)
	} else {
		s.mutex.Lock()
		s.cache[absPath] = &FileCacheEntry{
			Data:      data,
			CreatedAt: time.Now(),
			LastRead:  time.Now(),
			FileInfo:  fileInfo,
		}

		// 添加这段代码：检查缓存大小，如果超出最大值，清理最少使用的条目
		if len(s.cache) > s.maxSize {
			// 计算需要删除的条目数量，通常是 20% 或至少 1 个
			toRemove := max(1, s.maxSize/5)
			s.cleanupLRU(toRemove)
		}

		s.mutex.Unlock()
	}

	return nil
}

// DeleteFromCache 从缓存中删除条目
func (s *FileCacheService) DeleteFromCache(path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	s.mutex.Lock()
	delete(s.cache, absPath)
	s.mutex.Unlock()
}

// ClearCache 清空缓存
func (s *FileCacheService) ClearCache() {
	s.mutex.Lock()
	s.cache = make(map[string]*FileCacheEntry)
	s.mutex.Unlock()
}

// 清理最少使用的条目
func (s *FileCacheService) cleanupLRU(count int) {
	type keyAge struct {
		key  string
		time time.Time
	}

	entries := make([]keyAge, 0, len(s.cache))
	for k, v := range s.cache {
		entries = append(entries, keyAge{k, v.LastRead})
	}

	// 按最后读取时间排序
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].time.Before(entries[j].time)
	})

	// 删除最少使用的条目
	maxToDelete := min(count, len(entries))
	for i := 0; i < maxToDelete; i++ {
		delete(s.cache, entries[i].key)
	}
}
