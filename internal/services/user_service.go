// internal/services/user_service.go
package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// UserService 处理用户相关的业务逻辑
type UserService struct {
	BasePath string
}

// NewUserService 创建用户服务
func NewUserService() *UserService {
	basePath := "data/users"

	// 确保用户数据目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		fmt.Printf("警告: 创建用户数据目录失败: %v\n", err)
	}

	return &UserService{
		BasePath: basePath,
	}
}

// GetUser 获取用户信息
func (s *UserService) GetUser(userID string) (*models.User, error) {
	userPath := filepath.Join(s.BasePath, userID+".json")

	// 检查用户文件是否存在
	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("用户不存在: %s", userID)
	}

	// 读取用户数据
	userDataBytes, err := os.ReadFile(userPath)
	if err != nil {
		return nil, fmt.Errorf("读取用户数据失败: %w", err)
	}

	var userData models.User
	if err := json.Unmarshal(userDataBytes, &userData); err != nil {
		return nil, fmt.Errorf("解析用户数据失败: %w", err)
	}

	return &userData, nil
}

// CreateUser 创建新用户
func (s *UserService) CreateUser(username string, email string) (*models.User, error) {
	// 生成用户ID
	userID := fmt.Sprintf("user_%d", time.Now().UnixNano())

	now := time.Now()

	// 创建用户对象
	user := &models.User{
		ID:          userID,
		Username:    username,
		Email:       email,
		CreatedAt:   now,
		LastLogin:   now,
		LastUpdated: now,
		Preferences: models.UserPreferences{
			CreativityLevel:   models.CreativityBalanced,
			AllowPlotTwists:   true,
			ResponseLength:    "medium",
			LanguageStyle:     "casual",
			NotificationLevel: "important",
			DarkMode:          false,
		},
	}

	// 保存用户数据
	if err := s.SaveUser(user); err != nil {
		return nil, err
	}

	return user, nil
}

// SaveUser 保存用户信息
func (s *UserService) SaveUser(user *models.User) error {
	userDataJSON, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化用户数据失败: %w", err)
	}

	userPath := filepath.Join(s.BasePath, user.ID+".json")
	if err := os.WriteFile(userPath, userDataJSON, 0644); err != nil {
		return fmt.Errorf("保存用户数据失败: %w", err)
	}

	return nil
}

// UpdateUserPreferences 更新用户偏好设置
func (s *UserService) UpdateUserPreferences(userID string, preferences models.UserPreferences) error {
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	user.Preferences = preferences
	user.LastUpdated = time.Now()

	return s.SaveUser(user)
}

// GetUserPreferences 获取用户偏好设置
func (s *UserService) GetUserPreferences(userID string) (models.UserPreferences, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return models.UserPreferences{}, err
	}

	return user.Preferences, nil
}

// 道具管理方法
// ----------------------------------------

// AddUserItem 为用户添加自定义道具
func (s *UserService) AddUserItem(userID string, item models.UserItem) error {
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	// 生成唯一ID
	if item.ID == "" {
		item.ID = fmt.Sprintf("item_%d", time.Now().UnixNano())
	}

	// 设置创建和更新时间
	now := time.Now()
	item.Created = now
	item.Updated = now

	// 添加道具
	user.Items = append(user.Items, item)
	user.LastUpdated = now

	return s.SaveUser(user)
}

// UpdateUserItem 更新用户自定义道具
func (s *UserService) UpdateUserItem(userID string, itemID string, updatedItem models.UserItem) error {
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	found := false
	for i, item := range user.Items {
		if item.ID == itemID {
			// 保留原始ID和创建时间
			updatedItem.ID = itemID
			updatedItem.Created = item.Created
			updatedItem.Updated = time.Now()

			// 更新道具
			user.Items[i] = updatedItem
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("道具不存在: %s", itemID)
	}

	user.LastUpdated = time.Now()
	return s.SaveUser(user)
}

// DeleteUserItem 删除用户自定义道具
func (s *UserService) DeleteUserItem(userID string, itemID string) error {
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	found := false
	for i, item := range user.Items {
		if item.ID == itemID {
			// 移除道具
			user.Items = append(user.Items[:i], user.Items[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("道具不存在: %s", itemID)
	}

	user.LastUpdated = time.Now()
	return s.SaveUser(user)
}

// GetUserItems 获取用户所有自定义道具
func (s *UserService) GetUserItems(userID string) ([]models.UserItem, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return nil, err
	}

	return user.Items, nil
}

// GetUserItem 获取用户特定自定义道具
func (s *UserService) GetUserItem(userID string, itemID string) (*models.UserItem, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return nil, err
	}

	for _, item := range user.Items {
		if item.ID == itemID {
			itemCopy := item // 创建副本
			return &itemCopy, nil
		}
	}

	return nil, fmt.Errorf("道具不存在: %s", itemID)
}

// 技能管理方法
// ----------------------------------------

// AddUserSkill 为用户添加自定义技能
func (s *UserService) AddUserSkill(userID string, skill models.UserSkill) error {
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	// 生成唯一ID
	if skill.ID == "" {
		skill.ID = fmt.Sprintf("skill_%d", time.Now().UnixNano())
	}

	// 设置创建和更新时间
	now := time.Now()
	skill.Created = now
	skill.Updated = now

	// 添加技能
	user.Skills = append(user.Skills, skill)
	user.LastUpdated = now

	return s.SaveUser(user)
}

// UpdateUserSkill 更新用户自定义技能
func (s *UserService) UpdateUserSkill(userID string, skillID string, updatedSkill models.UserSkill) error {
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	found := false
	for i, skill := range user.Skills {
		if skill.ID == skillID {
			// 保留原始ID和创建时间
			updatedSkill.ID = skillID
			updatedSkill.Created = skill.Created
			updatedSkill.Updated = time.Now()

			// 更新技能
			user.Skills[i] = updatedSkill
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("技能不存在: %s", skillID)
	}

	user.LastUpdated = time.Now()
	return s.SaveUser(user)
}

// DeleteUserSkill 删除用户自定义技能
func (s *UserService) DeleteUserSkill(userID string, skillID string) error {
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	found := false
	for i, skill := range user.Skills {
		if skill.ID == skillID {
			// 移除技能
			user.Skills = append(user.Skills[:i], user.Skills[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("技能不存在: %s", skillID)
	}

	user.LastUpdated = time.Now()
	return s.SaveUser(user)
}

// GetUserSkills 获取用户所有自定义技能
func (s *UserService) GetUserSkills(userID string) ([]models.UserSkill, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return nil, err
	}

	return user.Skills, nil
}

// GetUserSkill 获取用户特定自定义技能
func (s *UserService) GetUserSkill(userID string, skillID string) (*models.UserSkill, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return nil, err
	}

	for _, skill := range user.Skills {
		if skill.ID == skillID {
			skillCopy := skill // 创建副本
			return &skillCopy, nil
		}
	}

	return nil, fmt.Errorf("技能不存在: %s", skillID)
}
