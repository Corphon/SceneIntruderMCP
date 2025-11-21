// internal/storage/file_storage.go
package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileStorage 提供文件存储服务
type FileStorage struct {
	BaseDir string

	// 并发控制
	fileLocks sync.Map // 文件级别锁 path -> *sync.RWMutex

	// 简单缓存
	cache        map[string]*CacheEntry
	cacheMutex   sync.RWMutex
	cacheExpiry  time.Duration
	maxCacheSize int
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Data      []byte
	Timestamp time.Time
}

// NewFileStorage 创建文件存储服务
func NewFileStorage(baseDir string) (*FileStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	fs := &FileStorage{
		BaseDir:      baseDir,
		cache:        make(map[string]*CacheEntry),
		cacheExpiry:  5 * time.Minute,
		maxCacheSize: 100,
	}

	// 启动缓存清理
	fs.StartCacheCleanup()

	return fs, nil
}

// 获取文件锁
func (fs *FileStorage) getFileLock(fullPath string) *sync.RWMutex {
	value, _ := fs.fileLocks.LoadOrStore(fullPath, &sync.RWMutex{})
	return value.(*sync.RWMutex)
}

// SaveTextFile 保存文本文件
func (fs *FileStorage) SaveTextFile(dirPath, filename string, content []byte) error {
	fullDirPath := filepath.Join(fs.BaseDir, dirPath)
	fullPath := filepath.Join(fullDirPath, filename)

	// 获取文件锁
	lock := fs.getFileLock(fullPath)
	lock.Lock()
	defer lock.Unlock()

	// 确保目录存在
	if err := os.MkdirAll(fullDirPath, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 原子性文件写入
	tempPath := fullPath + ".tmp"

	if err := os.WriteFile(tempPath, content, 0644); err != nil {
		return fmt.Errorf("保存临时文件失败: %w", err)
	}

	if err := os.Rename(tempPath, fullPath); err != nil {
		// Ensure temp file is cleaned up on error
		if removeErr := os.Remove(tempPath); removeErr != nil {
			// Log the cleanup error but return the original error
			fmt.Printf("Warning: failed to clean up temporary file %s after rename failure: %v\n", tempPath, removeErr)
		}
		return fmt.Errorf("保存文件失败: %w", err)
	}

	// Clear cache after successful write
	fs.invalidateCache(fullPath)

	return nil
}

// SaveJSONFile 保存JSON文件
func (fs *FileStorage) SaveJSONFile(dirPath, filename string, data interface{}) error {
	// 序列化JSON
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化JSON失败: %w", err)
	}

	// 保存文件
	return fs.SaveTextFile(dirPath, filename, content)
}

// LoadTextFile 读取文本文件
func (fs *FileStorage) LoadTextFile(dirPath, filename string) ([]byte, error) {
	fullPath := filepath.Join(fs.BaseDir, dirPath, filename)

	// 检查缓存
	fs.cacheMutex.RLock()
	if entry, exists := fs.cache[fullPath]; exists {
		if time.Since(entry.Timestamp) < fs.cacheExpiry {
			fs.cacheMutex.RUnlock()
			return entry.Data, nil
		}
	}
	fs.cacheMutex.RUnlock()

	// 获取文件锁（读锁）
	lock := fs.getFileLock(fullPath)
	lock.RLock()
	defer lock.RUnlock()

	// 双重检查缓存
	fs.cacheMutex.RLock()
	if entry, exists := fs.cache[fullPath]; exists {
		if time.Since(entry.Timestamp) < fs.cacheExpiry {
			fs.cacheMutex.RUnlock()
			return entry.Data, nil
		}
	}
	fs.cacheMutex.RUnlock()

	// 读取文件
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 更新缓存
	fs.updateCache(fullPath, content)

	return content, nil
}

// 缓存管理
func (fs *FileStorage) updateCache(path string, data []byte) {
	fs.cacheMutex.Lock()
	defer fs.cacheMutex.Unlock()

	fs.cache[path] = &CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
	}

	// 简单的缓存大小控制
	if len(fs.cache) > fs.maxCacheSize {
		// 删除最老的条目
		var oldestKey string
		var oldestTime time.Time

		for key, entry := range fs.cache {
			if oldestKey == "" || entry.Timestamp.Before(oldestTime) {
				oldestKey = key
				oldestTime = entry.Timestamp
			}
		}

		if oldestKey != "" {
			delete(fs.cache, oldestKey)
		}
	}
}

// LoadJSONFile 读取并解析JSON文件
func (fs *FileStorage) LoadJSONFile(dirPath, filename string, v interface{}) error {
	content, err := fs.LoadTextFile(dirPath, filename)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(content, v); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	return nil
}

// DirExists 检查目录是否存在
func (fs *FileStorage) DirExists(dirPath string) bool {
	fullPath := filepath.Join(fs.BaseDir, dirPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FileExists 检查文件是否存在
func (fs *FileStorage) FileExists(dirPath, filename string) bool {
	fullPath := filepath.Join(fs.BaseDir, dirPath, filename)
	_, err := os.Stat(fullPath)
	return err == nil
}

// DeleteFile 删除文件
func (fs *FileStorage) DeleteFile(dirPath, filename string) error {
	fullPath := filepath.Join(fs.BaseDir, dirPath, filename)

	// 获取文件锁
	lock := fs.getFileLock(fullPath)
	lock.Lock()
	defer lock.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", fullPath)
	}

	// 删除文件
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}

	// 清除缓存
	fs.invalidateCache(fullPath)

	return nil
}

// DeleteDir 删除目录及其内容
func (fs *FileStorage) DeleteDir(dirPath string) error {
	fullPath := filepath.Join(fs.BaseDir, dirPath)

	// 获取目录锁 (使用目录路径作为锁键)
	lock := fs.getFileLock(fullPath)
	lock.Lock()
	defer lock.Unlock()

	// 检查目录是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("目录不存在: %s", fullPath)
	}

	// 删除目录及其内容
	if err := os.RemoveAll(fullPath); err != nil {
		return fmt.Errorf("删除目录失败: %w", err)
	}

	// 清除目录相关的缓存项
	fs.removeCacheEntriesWithPrefix(fullPath)

	return nil
}

// removeCacheEntriesWithPrefix 移除指定前缀的缓存条目
func (fs *FileStorage) removeCacheEntriesWithPrefix(prefix string) {
	fs.cacheMutex.Lock()
	defer fs.cacheMutex.Unlock()

	for key := range fs.cache {
		if strings.HasPrefix(key, prefix) {
			delete(fs.cache, key)
		}
	}
}

// ListDirs 列出目录下的所有子目录
func (fs *FileStorage) ListDirs(dirPath string) ([]string, error) {
	fullPath := filepath.Join(fs.BaseDir, dirPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}

	return dirs, nil
}

// 开始缓存清理
func (fs *FileStorage) StartCacheCleanup() {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			fs.cleanupExpiredCache()
			fs.enforceMaxCacheSize()
		}
	}()
}

// 清理过期缓存
func (fs *FileStorage) cleanupExpiredCache() {
	fs.cacheMutex.Lock()
	defer fs.cacheMutex.Unlock()

	now := time.Now()
	for path, entry := range fs.cache {
		if now.Sub(entry.Timestamp) > fs.cacheExpiry {
			delete(fs.cache, path)
		}
	}
}

// enforceMaxCacheSize enforces the maximum cache size by removing oldest entries
func (fs *FileStorage) enforceMaxCacheSize() {
	fs.cacheMutex.Lock()
	defer fs.cacheMutex.Unlock()

	// Check if cache size exceeds maximum
	if len(fs.cache) <= fs.maxCacheSize {
		return
	}

	// Find oldest entries to remove
	type cacheEntryWithTime struct {
		key       string
		timestamp time.Time
	}

	var entries []cacheEntryWithTime
	for key, entry := range fs.cache {
		entries = append(entries, cacheEntryWithTime{key: key, timestamp: entry.Timestamp})
	}

	// Sort by timestamp (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].timestamp.Before(entries[j].timestamp)
	})

	// Remove excess entries
	removeCount := len(entries) - fs.maxCacheSize
	if removeCount > 0 {
		for i := 0; i < removeCount; i++ {
			delete(fs.cache, entries[i].key)
		}
		log.Printf("缓存大小限制执行: 移除了 %d 个最旧的缓存条目", removeCount)
	}
}

// invalidateCache 清除指定路径的缓存
func (fs *FileStorage) invalidateCache(path string) {
	fs.cacheMutex.Lock()
	defer fs.cacheMutex.Unlock()

	delete(fs.cache, path)
}
