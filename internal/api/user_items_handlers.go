// internal/api/user_items_handlers.go
package api

import (
	"fmt"
	"net/http"

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
