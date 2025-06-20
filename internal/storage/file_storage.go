// internal/storage/file_storage.go
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileStorage 提供文件存储服务
type FileStorage struct {
	BaseDir string
}

// NewFileStorage 创建文件存储服务
func NewFileStorage(baseDir string) (*FileStorage, error) {
	// 确保基础目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	return &FileStorage{
		BaseDir: baseDir,
	}, nil
}

// SaveTextFile 保存文本文件
func (fs *FileStorage) SaveTextFile(dirPath, filename string, content []byte) error {
	// 确保目录存在
	fullDirPath := filepath.Join(fs.BaseDir, dirPath)
	if err := os.MkdirAll(fullDirPath, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 保存文件
	fullPath := filepath.Join(fullDirPath, filename)
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

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
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	return content, nil
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
