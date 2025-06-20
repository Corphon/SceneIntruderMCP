// internal/services/item_service.go
package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// ItemService 处理物品相关的业务逻辑
type ItemService struct {
	BasePath string
}

// NewItemService 创建物品服务
func NewItemService() *ItemService {
	basePath := "data/items"

	// 确保物品数据目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		fmt.Printf("警告: 创建物品数据目录失败: %v\n", err)
	}

	return &ItemService{
		BasePath: basePath,
	}
}

// AddItem 添加物品
func (s *ItemService) AddItem(sceneID string, item *models.Item) error {
	// 确保场景物品目录存在
	scenePath := filepath.Join(s.BasePath, sceneID)
	if err := os.MkdirAll(scenePath, 0755); err != nil {
		return fmt.Errorf("创建场景物品目录失败: %w", err)
	}

	// 如果是新物品(ID为空)，设置创建时间
	if item.ID == "" {
		item.ID = fmt.Sprintf("item_%d", time.Now().UnixNano())
		item.CreatedAt = time.Now()
	}

	// 确保SceneID正确设置
	item.SceneID = sceneID

	// 始终更新LastUpdated
	item.LastUpdated = time.Now()

	// 保存物品数据
	itemPath := filepath.Join(scenePath, item.ID+".json")
	itemDataJSON, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化物品数据失败: %w", err)
	}

	if err := os.WriteFile(itemPath, itemDataJSON, 0644); err != nil {
		return fmt.Errorf("保存物品数据失败: %w", err)
	}

	return nil
}

// GetItem 获取物品
func (s *ItemService) GetItem(sceneID, itemID string) (*models.Item, error) {
	itemPath := filepath.Join(s.BasePath, sceneID, itemID+".json")

	// 检查物品文件是否存在
	if _, err := os.Stat(itemPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("物品不存在: %s", itemID)
	}

	// 读取物品数据
	itemDataBytes, err := os.ReadFile(itemPath)
	if err != nil {
		return nil, fmt.Errorf("读取物品数据失败: %w", err)
	}

	var itemData models.Item
	if err := json.Unmarshal(itemDataBytes, &itemData); err != nil {
		return nil, fmt.Errorf("解析物品数据失败: %w", err)
	}

	return &itemData, nil
}

// GetAllItems 获取场景中的所有物品
func (s *ItemService) GetAllItems(sceneID string) ([]*models.Item, error) {
	scenePath := filepath.Join(s.BasePath, sceneID)

	// 检查目录是否存在
	if _, err := os.Stat(scenePath); os.IsNotExist(err) {
		return []*models.Item{}, nil
	}

	// 读取目录中的所有文件
	files, err := os.ReadDir(scenePath)
	if err != nil {
		return nil, fmt.Errorf("读取场景物品目录失败: %w", err)
	}

	items := make([]*models.Item, 0, len(files))
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		itemPath := filepath.Join(scenePath, file.Name())
		itemDataBytes, err := os.ReadFile(itemPath)
		if err != nil {
			continue
		}

		var itemData models.Item
		if err := json.Unmarshal(itemDataBytes, &itemData); err != nil {
			continue
		}

		items = append(items, &itemData)
	}

	return items, nil
}

// UpdateItem 更新物品
func (s *ItemService) UpdateItem(sceneID string, item *models.Item) error {
	item.LastUpdated = time.Now()
	return s.AddItem(sceneID, item)
}
