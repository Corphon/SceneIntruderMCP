// internal/api/user_items_handlers.go
package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/gin-gonic/gin"
)

// 用户道具相关处理器
// ----------------------------------------

// AddUserItem 添加用户自定义道具
func (h *Handler) AddUserItem(c *gin.Context) {
	userID := c.Param("user_id")

	var item models.UserItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的道具数据"})
		return
	}

	if err := h.UserService.AddUserItem(userID, item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("添加道具失败: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// GetUserItems 获取用户所有自定义道具
func (h *Handler) GetUserItems(c *gin.Context) {
	userID := c.Param("user_id")

	items, err := h.UserService.GetUserItems(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取道具失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, items)
}

// GetUserItem 获取用户特定自定义道具
func (h *Handler) GetUserItem(c *gin.Context) {
	userID := c.Param("user_id")
	itemID := c.Param("item_id")

	item, err := h.UserService.GetUserItem(userID, itemID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("获取道具失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, item)
}

// UpdateUserItem 更新用户自定义道具
func (h *Handler) UpdateUserItem(c *gin.Context) {
	userID := c.Param("user_id")
	itemID := c.Param("item_id")

	var item models.UserItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的道具数据"})
		return
	}

	if err := h.UserService.UpdateUserItem(userID, itemID, item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新道具失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, item)
}

// DeleteUserItem 删除用户自定义道具
func (h *Handler) DeleteUserItem(c *gin.Context) {
	userID := c.Param("user_id")
	itemID := c.Param("item_id")

	if err := h.UserService.DeleteUserItem(userID, itemID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除道具失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "道具已删除"})
}

// 用户技能相关处理器
// ----------------------------------------

// AddUserSkill 添加用户自定义技能
func (h *Handler) AddUserSkill(c *gin.Context) {
	userID := c.Param("user_id")

	var skill models.UserSkill
	if err := c.ShouldBindJSON(&skill); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的技能数据"})
		return
	}

	if err := h.UserService.AddUserSkill(userID, skill); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("添加技能失败: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, skill)
}

// GetUserSkills 获取用户所有自定义技能
func (h *Handler) GetUserSkills(c *gin.Context) {
	userID := c.Param("user_id")

	skills, err := h.UserService.GetUserSkills(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取技能失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, skills)
}

// GetUserSkill 获取用户特定自定义技能
func (h *Handler) GetUserSkill(c *gin.Context) {
	userID := c.Param("user_id")
	skillID := c.Param("skill_id")

	skill, err := h.UserService.GetUserSkill(userID, skillID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("获取技能失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, skill)
}

// UpdateUserSkill 更新用户自定义技能
func (h *Handler) UpdateUserSkill(c *gin.Context) {
	userID := c.Param("user_id")
	skillID := c.Param("skill_id")

	var skill models.UserSkill
	if err := c.ShouldBindJSON(&skill); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的技能数据"})
		return
	}

	if err := h.UserService.UpdateUserSkill(userID, skillID, skill); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新技能失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, skill)
}

// DeleteUserSkill 删除用户自定义技能
func (h *Handler) DeleteUserSkill(c *gin.Context) {
	userID := c.Param("user_id")
	skillID := c.Param("skill_id")

	if err := h.UserService.DeleteUserSkill(userID, skillID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除技能失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "技能已删除"})
}

// GetUserProfile 获取用户档案
func (h *Handler) GetUserProfile(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少用户ID"})
		return
	}

	user, err := h.UserService.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("用户不存在: %v", err)})
		return
	}

	// 不返回敏感信息
	profile := map[string]interface{}{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"bio":          user.Bio,
		"avatar":       user.Avatar,
		"created_at":   user.CreatedAt,
		"preferences":  user.Preferences,
		"items_count":  len(user.Items),
		"skills_count": len(user.Skills),
		"saved_scenes": user.SavedScenes,
	}

	c.JSON(http.StatusOK, profile)
}

// UpdateUserProfile 更新用户档案
func (h *Handler) UpdateUserProfile(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少用户ID"})
		return
	}

	var updateData struct {
		Username    string                  `json:"username,omitempty"`
		DisplayName string                  `json:"display_name,omitempty"`
		Bio         string                  `json:"bio,omitempty"`
		Avatar      string                  `json:"avatar,omitempty"`
		Preferences *models.UserPreferences `json:"preferences,omitempty"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的更新数据: " + err.Error()})
		return
	}

	// 获取现有用户
	user, err := h.UserService.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 更新字段
	if updateData.Username != "" {
		user.Username = updateData.Username
	}
	if updateData.DisplayName != "" {
		user.DisplayName = updateData.DisplayName
	}
	if updateData.Bio != "" {
		user.Bio = updateData.Bio
	}
	if updateData.Avatar != "" {
		user.Avatar = updateData.Avatar
	}
	if updateData.Preferences != nil {
		user.Preferences = *updateData.Preferences
	}

	// 更新时间戳
	user.LastUpdated = time.Now()

	// 保存用户
	if err := h.UserService.SaveUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存用户信息失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "用户档案更新成功",
	})
}

// GetUserPreferences 获取用户偏好设置
func (h *Handler) GetUserPreferences(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少用户ID"})
		return
	}

	preferences, err := h.UserService.GetUserPreferences(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取用户偏好失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, preferences)
}

// UpdateUserPreferences 更新用户偏好设置
func (h *Handler) UpdateUserPreferences(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少用户ID"})
		return
	}

	var preferences models.UserPreferences
	if err := c.ShouldBindJSON(&preferences); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的偏好设置数据: " + err.Error()})
		return
	}

	if err := h.UserService.UpdateUserPreferences(userID, preferences); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新用户偏好失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "用户偏好更新成功",
	})
}
