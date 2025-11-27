// internal/services/item_service.go
package services

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// ItemService 处理物品相关的业务逻辑
type ItemService struct {
	ScenesPath string

	// 并发控制
	sceneLocks  sync.Map // sceneID -> *sync.RWMutex
	cacheMutex  sync.RWMutex
	itemCache   map[string]*CachedItemData
	cacheExpiry time.Duration

	// 文件操作保护
	fileMutex sync.Mutex
}

// CachedItemData 缓存的物品数据
type CachedItemData struct {
	Items     map[string]*models.Item // itemID -> Item
	Timestamp time.Time
}

// NewItemService 创建物品服务
func NewItemService(scenesPath string) *ItemService {
	if scenesPath == "" {
		scenesPath = filepath.Join("data", "scenes")
	}

	if err := os.MkdirAll(scenesPath, 0755); err != nil {
		fmt.Printf("警告: 创建场景数据目录失败: %v\n", err)
	}

	service := &ItemService{
		ScenesPath:  scenesPath,
		itemCache:   make(map[string]*CachedItemData),
		cacheExpiry: 3 * time.Minute, // 3分钟缓存
	}

	// 启动缓存清理
	service.startCacheCleanup()

	return service
}

// 获取场景锁
func (s *ItemService) getSceneLock(sceneID string) *sync.RWMutex {
	value, _ := s.sceneLocks.LoadOrStore(sceneID, &sync.RWMutex{})
	return value.(*sync.RWMutex)
}

// 安全加载场景物品数据（带缓存）
func (s *ItemService) loadSceneItemsSafe(sceneID string) (*CachedItemData, error) {
	// 防护措施：确保 itemCache 已初始化
	if s.itemCache == nil {
		s.cacheMutex.Lock()
		if s.itemCache == nil {
			s.itemCache = make(map[string]*CachedItemData)
		}
		s.cacheMutex.Unlock()
	}

	// 检查缓存
	s.cacheMutex.RLock()
	if cached, exists := s.itemCache[sceneID]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			return cached, nil
		}
	}
	s.cacheMutex.RUnlock()

	// 获取场景锁
	lock := s.getSceneLock(sceneID)
	lock.RLock()
	defer lock.RUnlock()

	// 双重检查
	s.cacheMutex.RLock()
	if cached, exists := s.itemCache[sceneID]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			return cached, nil
		}
	}
	s.cacheMutex.RUnlock()

	// 加载数据
	scenePath := filepath.Join(s.ScenesPath, sceneID, "items")
	cached := &CachedItemData{
		Items:     make(map[string]*models.Item),
		Timestamp: time.Now(),
	}

	// 检查目录是否存在
	if _, err := os.Stat(scenePath); os.IsNotExist(err) {
		// 目录不存在，返回空缓存
		s.cacheMutex.Lock()
		s.itemCache[sceneID] = cached
		s.cacheMutex.Unlock()
		return cached, nil
	}

	// 批量读取所有物品文件
	files, err := os.ReadDir(scenePath)
	if err != nil {
		return nil, fmt.Errorf("读取场景物品目录失败: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		itemPath := filepath.Join(scenePath, file.Name())
		itemDataBytes, err := os.ReadFile(itemPath)
		if err != nil {
			continue // 跳过无法读取的文件
		}

		var itemData models.Item
		if err := json.Unmarshal(itemDataBytes, &itemData); err != nil {
			continue // 跳过无法解析的文件
		}

		cached.Items[itemData.ID] = &itemData
	}

	// 更新缓存
	s.cacheMutex.Lock()
	s.itemCache[sceneID] = cached
	s.cacheMutex.Unlock()

	return cached, nil
}

// AddItem 添加物品
// 🔧 线程安全的 AddItem 方法
func (s *ItemService) AddItem(sceneID string, item *models.Item) error {
	// 获取场景写锁
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	// 🔧 线程安全的目录创建
	s.fileMutex.Lock()
	scenePath := filepath.Join(s.ScenesPath, sceneID, "items")
	if err := os.MkdirAll(scenePath, 0755); err != nil {
		s.fileMutex.Unlock()
		return fmt.Errorf("创建场景物品目录失败: %w", err)
	}
	s.fileMutex.Unlock()

	// 🔧 线程安全的ID生成
	if item.ID == "" {
		// 使用更安全的ID生成策略
		item.ID = s.generateUniqueItemID(sceneID)
		item.CreatedAt = time.Now()
	}

	// 确保SceneID正确设置
	item.SceneID = sceneID
	item.LastUpdated = time.Now()

	// 🔧 原子性文件写入
	itemPath := filepath.Join(scenePath, item.ID+".json")
	tempPath := itemPath + ".tmp"

	itemDataJSON, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化物品数据失败: %w", err)
	}

	// 先写入临时文件，再原子性重命名
	if err := os.WriteFile(tempPath, itemDataJSON, 0644); err != nil {
		return fmt.Errorf("保存物品数据失败: %w", err)
	}

	if err := os.Rename(tempPath, itemPath); err != nil {
		os.Remove(tempPath) // 清理临时文件
		return fmt.Errorf("保存物品数据失败: %w", err)
	}

	// 🔧 更新缓存
	s.invalidateSceneCache(sceneID)

	return nil
}

// 🔧 生成唯一物品ID
func (s *ItemService) generateUniqueItemID(sceneID string) string {
	for {
		id := fmt.Sprintf("item_%d_%d", time.Now().UnixNano(), rand.Intn(1000))
		itemPath := filepath.Join(s.ScenesPath, sceneID, "items", id+".json")

		// 检查文件是否已存在
		if _, err := os.Stat(itemPath); os.IsNotExist(err) {
			return id
		}

		// 如果ID冲突，稍微等待后重试
		time.Sleep(time.Microsecond)
	}
}

// GetItem 获取物品
func (s *ItemService) GetItem(sceneID, itemID string) (*models.Item, error) {
	// 使用缓存加载场景物品数据
	cachedData, err := s.loadSceneItemsSafe(sceneID)
	if err != nil {
		return nil, err
	}

	item, exists := cachedData.Items[itemID]
	if !exists {
		return nil, fmt.Errorf("物品不存在: %s", itemID)
	}

	// 返回副本以避免外部修改
	itemCopy := *item
	return &itemCopy, nil
}

// GetAllItems 获取场景中的所有物品
func (s *ItemService) GetAllItems(sceneID string) ([]*models.Item, error) {
	// 使用缓存加载场景物品数据
	cachedData, err := s.loadSceneItemsSafe(sceneID)
	if err != nil {
		return nil, err
	}

	items := make([]*models.Item, 0, len(cachedData.Items))
	for _, item := range cachedData.Items {
		// 返回副本以避免外部修改
		itemCopy := *item
		items = append(items, &itemCopy)
	}

	return items, nil
}

// UpdateItem 更新物品
func (s *ItemService) UpdateItem(sceneID string, item *models.Item) error {
	item.LastUpdated = time.Now()
	return s.AddItem(sceneID, item)
}

// 清除指定场景的缓存
func (s *ItemService) invalidateSceneCache(sceneID string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// nil 检查
	if s.itemCache != nil {
		delete(s.itemCache, sceneID)
	}

	delete(s.itemCache, sceneID)
}

// 清理过期缓存
func (s *ItemService) cleanupExpiredCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// nil 检查
	if s.itemCache == nil {
		return
	}

	now := time.Now()
	for sceneID, cached := range s.itemCache {
		if now.Sub(cached.Timestamp) > s.cacheExpiry {
			delete(s.itemCache, sceneID)
		}
	}
}

// 启动后台缓存清理
func (s *ItemService) startCacheCleanup() {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.cleanupExpiredCache()
		}
	}()
}

// DeleteItem 删除物品
func (s *ItemService) DeleteItem(sceneID, itemID string) error {
	// 获取场景写锁
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	// 删除物品文件
	itemPath := filepath.Join(s.ScenesPath, sceneID, "items", itemID+".json")
	if err := os.Remove(itemPath); err != nil {
		return fmt.Errorf("删除物品失败: %w", err)
	}

	// 更新缓存
	s.invalidateSceneCache(sceneID)

	return nil
}
