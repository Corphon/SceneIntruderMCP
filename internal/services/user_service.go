// internal/services/user_service.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
	"github.com/Corphon/SceneIntruderMCP/internal/utils"
)

// UserService 处理用户相关的业务逻辑
type UserService struct {
	BasePath    string
	FileStorage *storage.FileStorage

	// 并发控制
	userLocks   sync.Map                   // 用户级别锁
	cacheMutex  sync.RWMutex               // 缓存锁
	userCache   map[string]*CachedUserData // 用户缓存
	cacheExpiry time.Duration              // 缓存过期时间
}

// CachedUserData 缓存的用户数据
type CachedUserData struct {
	User      *models.User
	Timestamp time.Time
}

var ErrSkillValidation = errors.New("skill validation failed")

// ---------------------------------
// NewUserService 创建用户服务
func NewUserService() *UserService {
	basePath := "data/users"

	if err := os.MkdirAll(basePath, 0755); err != nil {
		utils.GetLogger().Warn("创建用户数据目录失败", map[string]interface{}{"base_path": basePath, "err": err})
	}

	// 创建 FileStorage 实例
	fileStorage, err := storage.NewFileStorage(basePath)
	if err != nil {
		utils.GetLogger().Warn("创建文件存储服务失败", map[string]interface{}{"base_path": basePath, "err": err})
		fileStorage = nil
	}

	service := &UserService{
		BasePath:    basePath,
		FileStorage: fileStorage,
		userCache:   make(map[string]*CachedUserData),
		cacheExpiry: 5 * time.Minute,
	}

	// 启动缓存清理
	service.startCacheCleanup()

	// 确保控制台默认用户可用，便于技能等功能直接使用
	service.ensureDefaultUser("console_user")

	return service
}

// 用户锁管理
func (s *UserService) getUserLock(userID string) *sync.RWMutex {
	value, _ := s.userLocks.LoadOrStore(userID, &sync.RWMutex{})
	return value.(*sync.RWMutex)
}

// GetUser 获取用户信息（带缓从）
func (s *UserService) GetUser(userID string) (*models.User, error) {
	// 检查缓存
	s.cacheMutex.RLock()
	if cached, exists := s.userCache[userID]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			// 返回深度副本
			userCopy := *cached.User
			return &userCopy, nil
		}
	}
	s.cacheMutex.RUnlock()

	// 获取用户锁
	lock := s.getUserLock(userID)
	lock.RLock()
	defer lock.RUnlock()

	// 双重检查
	s.cacheMutex.RLock()
	if cached, exists := s.userCache[userID]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			userCopy := *cached.User
			return &userCopy, nil
		}
	}
	s.cacheMutex.RUnlock()

	// 统一使用 loadUserDirect 方法，确保路径一致性
	userData, err := s.loadUserDirect(userID)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	s.cacheMutex.Lock()
	s.userCache[userID] = &CachedUserData{
		User:      userData,
		Timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	return userData, nil
}

// 缓存失效方法
func (s *UserService) invalidateUserCache(userID string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	delete(s.userCache, userID)
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

// ensureDefaultUser 会在缺少指定用户时创建一个基础用户，保证控制台功能可用
func (s *UserService) ensureDefaultUser(userID string) {
	if err := s.EnsureUserExists(userID); err != nil {
		utils.GetLogger().Warn("创建默认用户失败", map[string]interface{}{"user_id": userID, "err": err})
	}
}

// EnsureUserExists 确保给定用户存在，如不存在则创建一个带默认偏好的用户
func (s *UserService) EnsureUserExists(userID string) error {
	if userID == "" {
		return fmt.Errorf("用户ID不能为空")
	}

	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	userPath := filepath.Join(s.BasePath, userID+".json")
	if _, err := os.Stat(userPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查用户数据失败: %w", err)
	}

	now := time.Now()
	user := &models.User{
		ID:          userID,
		Username:    userID,
		Email:       fmt.Sprintf("%s@local", userID),
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
			PreferredModel:    "qwen3-max",
			AutoSave:          true,
		},
		Items:  []models.UserItem{},
		Skills: []models.UserSkill{},
	}

	return s.saveUserDirect(user)
}

// SaveUser 保存用户信息（线程安全）
func (s *UserService) SaveUser(user *models.User) error {
	if user == nil || user.ID == "" {
		return fmt.Errorf("用户数据无效")
	}

	// 获取用户锁
	lock := s.getUserLock(user.ID)
	lock.Lock()
	defer lock.Unlock()

	// 更新时间戳
	user.LastUpdated = time.Now()

	// 统一使用 saveUserDirect 方法，确保路径一致性
	return s.saveUserDirect(user)
}

// UpdateUserPreferences 更新用户偏好设置
func (s *UserService) UpdateUserPreferences(userID string, preferences models.UserPreferences) error {
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	user, err := s.loadUserDirect(userID)
	if err != nil {
		return err
	}

	user.Preferences = preferences
	user.LastUpdated = time.Now()

	// 保存后失效缓存，确保下次读取最新数据
	if err := s.saveUserDirect(user); err != nil {
		return err
	}

	// 确保缓存失效
	s.invalidateUserCache(userID)

	return nil
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

// AddUserItem 为用户添加自定义道具（线程安全）
func (s *UserService) AddUserItem(userID string, item models.UserItem) error {
	// 获取用户锁
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	// 重新获取最新用户数据
	user, err := s.loadUserDirect(userID)
	if err != nil {
		return err
	}

	// 生成唯一ID
	if item.ID == "" {
		item.ID = s.generateUniqueItemID(userID)
	}

	// 设置时间
	now := time.Now()
	item.Created = now
	item.Updated = now

	// 添加道具
	user.Items = append(user.Items, item)
	user.LastUpdated = now

	// 保存并失效缓存
	if err := s.saveUserDirect(user); err != nil {
		return err
	}

	// 确保缓存失效
	s.invalidateUserCache(userID)
	return nil
}

// 直接文件操作（在锁内使用）
func (s *UserService) loadUserDirect(userID string) (*models.User, error) {
	// 统一使用直接文件读取，确保路径一致性
	userPath := filepath.Join(s.BasePath, userID+".json")

	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("用户不存在: %s", userID)
	}

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

// 直接保存用户数据（在锁内使用）
func (s *UserService) saveUserDirect(user *models.User) error {
	userDataJSON, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化用户数据失败: %w", err)
	}

	userPath := filepath.Join(s.BasePath, user.ID+".json")
	tempPath := userPath + ".tmp"

	if err := os.WriteFile(tempPath, userDataJSON, 0644); err != nil {
		return fmt.Errorf("保存用户数据失败: %w", err)
	}

	if err := os.Rename(tempPath, userPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("保存用户数据失败: %w", err)
	}

	// 保存成功后更新缓存而不是清除
	s.cacheMutex.Lock()
	s.userCache[user.ID] = &CachedUserData{
		User:      user,
		Timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	return nil
}

// 安全的ID生成
func (s *UserService) generateUniqueItemID(userID string) string {
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("item_%d_%d", time.Now().UnixNano(), i)

		// 检查ID是否已存在（简单检查）
		user, err := s.loadUserDirect(userID)
		if err != nil {
			break
		}

		exists := false
		for _, item := range user.Items {
			if item.ID == id {
				exists = true
				break
			}
		}

		if !exists {
			return id
		}

		time.Sleep(time.Microsecond)
	}

	// 如果重试失败，使用随机数
	return fmt.Sprintf("item_%d_%d", time.Now().UnixNano(), rand.Intn(1000000))
}

// UpdateUserItem 更新用户自定义道具
func (s *UserService) UpdateUserItem(userID string, itemID string, updatedItem models.UserItem) error {
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	user, err := s.loadUserDirect(userID)
	if err != nil {
		return err
	}

	found := false
	for i, item := range user.Items {
		if item.ID == itemID {
			updatedItem.ID = itemID
			updatedItem.Created = item.Created
			updatedItem.Updated = time.Now()
			user.Items[i] = updatedItem
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("道具不存在: %s", itemID)
	}

	user.LastUpdated = time.Now()

	// 保存并失效缓存
	if err := s.saveUserDirect(user); err != nil {
		return err
	}

	s.invalidateUserCache(userID)
	return nil
}

// DeleteUserItem 删除用户自定义道具
func (s *UserService) DeleteUserItem(userID string, itemID string) error {
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	user, err := s.loadUserDirect(userID)
	if err != nil {
		return err
	}

	found := false
	for i, item := range user.Items {
		if item.ID == itemID {
			user.Items = append(user.Items[:i], user.Items[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("道具不存在: %s", itemID)
	}

	user.LastUpdated = time.Now()

	// 保存并失效缓存
	if err := s.saveUserDirect(user); err != nil {
		return err
	}

	s.invalidateUserCache(userID)
	return nil
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

// ---技能管理方法---
// AddUserSkill 为用户添加自定义技能
func (s *UserService) AddUserSkill(userID string, skill models.UserSkill) error {
	// 获取用户锁
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	// 重新获取最新用户数据
	user, err := s.loadUserDirect(userID)
	if err != nil {
		return err
	}

	sanitizeUserSkill(&skill)
	if err := validateUserSkill(&skill); err != nil {
		return err
	}

	// 生成唯一ID
	if skill.ID == "" {
		skill.ID = s.generateUniqueSkillID(userID)
	}

	// 设置时间
	now := time.Now()
	skill.Created = now
	skill.Updated = now

	// 添加技能
	user.Skills = append(user.Skills, skill)
	user.LastUpdated = now

	// 保存并失效缓存
	if err := s.saveUserDirect(user); err != nil {
		return err
	}

	s.invalidateUserCache(userID)
	return nil
}

// 直接生成技能ID
func (s *UserService) generateUniqueSkillID(userID string) string {
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("skill_%d_%d", time.Now().UnixNano(), i)
		// 检查ID是否已存在（简单检查）
		user, err := s.loadUserDirect(userID)
		if err != nil {
			break
		}
		exists := false
		for _, skill := range user.Skills {
			if skill.ID == id {
				exists = true
				break
			}
		}
		if !exists {
			return id
		}
		time.Sleep(time.Microsecond)
	}
	// 如果重试失败，使用随机数
	return fmt.Sprintf("skill_%d_%d", time.Now().UnixNano(), rand.Intn(1000000))
}

// UpdateUserSkill 更新用户自定义技能
func (s *UserService) UpdateUserSkill(userID string, skillID string, updatedSkill models.UserSkill) error {
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	user, err := s.loadUserDirect(userID)
	if err != nil {
		return err
	}

	found := false
	for i, skill := range user.Skills {
		if skill.ID == skillID {
			sanitizeUserSkill(&updatedSkill)
			if err := validateUserSkill(&updatedSkill); err != nil {
				return err
			}
			updatedSkill.ID = skillID
			updatedSkill.Created = skill.Created
			updatedSkill.Updated = time.Now()
			user.Skills[i] = updatedSkill
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("技能不存在: %s", skillID)
	}

	user.LastUpdated = time.Now()

	// 保存并失效缓存
	if err := s.saveUserDirect(user); err != nil {
		return err
	}

	s.invalidateUserCache(userID)
	return nil
}

// DeleteUserSkill 删除用户自定义技能
func (s *UserService) DeleteUserSkill(userID string, skillID string) error {
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	user, err := s.loadUserDirect(userID)
	if err != nil {
		return err
	}

	found := false
	for i, skill := range user.Skills {
		if skill.ID == skillID {
			user.Skills = append(user.Skills[:i], user.Skills[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("技能不存在: %s", skillID)
	}

	user.LastUpdated = time.Now()

	// 保存并失效缓存
	if err := s.saveUserDirect(user); err != nil {
		return err
	}

	s.invalidateUserCache(userID)
	return nil
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

// 启动缓存清理
func (s *UserService) startCacheCleanup() {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.cleanupExpiredCache()
		}
	}()
}

// 清理过期缓存
func (s *UserService) cleanupExpiredCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	now := time.Now()
	for userID, cached := range s.userCache {
		if now.Sub(cached.Timestamp) > s.cacheExpiry {
			delete(s.userCache, userID)
		}
	}
}

func sanitizeUserSkill(skill *models.UserSkill) {
	skill.Name = strings.TrimSpace(skill.Name)
	skill.Description = strings.TrimSpace(skill.Description)
	skill.Category = strings.TrimSpace(skill.Category)
	skill.IconURL = strings.TrimSpace(skill.IconURL)
	skill.Tags = sanitizeStringSlice(skill.Tags)
	skill.Requirements = sanitizeStringSlice(skill.Requirements)

	if len(skill.Effects) == 0 {
		skill.Effects = []models.SkillEffect{}
	}
	for i := range skill.Effects {
		effect := &skill.Effects[i]
		effect.Target = strings.TrimSpace(effect.Target)
		effect.Type = strings.TrimSpace(effect.Type)
		effect.Description = strings.TrimSpace(effect.Description)
		if effect.Target == "" {
			effect.Target = "self"
		}
		if effect.Type == "" {
			effect.Type = "custom"
		}
		if effect.Probability < 0 {
			effect.Probability = 0
		} else if effect.Probability > 1 {
			effect.Probability = 1
		}
		if effect.Value < 0 {
			effect.Value = 0
		}
		if effect.Duration < 0 {
			effect.Duration = 0
		}
	}

	if skill.Cooldown < 0 {
		skill.Cooldown = 0
	}
}

func sanitizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func validateUserSkill(skill *models.UserSkill) error {
	if strings.TrimSpace(skill.Name) == "" {
		return fmt.Errorf("%w: 技能名称不能为空", ErrSkillValidation)
	}
	if len(skill.Effects) == 0 {
		return fmt.Errorf("%w: 至少需要一个技能效果", ErrSkillValidation)
	}
	for idx, effect := range skill.Effects {
		if strings.TrimSpace(effect.Description) == "" && strings.TrimSpace(effect.Type) == "" {
			return fmt.Errorf("%w: 第 %d 个技能效果缺少描述或类型", ErrSkillValidation, idx+1)
		}
		if effect.Value < 0 {
			return fmt.Errorf("%w: 第 %d 个技能效果的数值不能为负", ErrSkillValidation, idx+1)
		}
		if effect.Duration < 0 {
			return fmt.Errorf("%w: 第 %d 个技能效果的持续时间不能为负", ErrSkillValidation, idx+1)
		}
		if effect.Probability < 0 || effect.Probability > 1 {
			return fmt.Errorf("%w: 第 %d 个技能效果的概率必须在 0-1 之间", ErrSkillValidation, idx+1)
		}
	}
	return nil
}
