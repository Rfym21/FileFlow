package api

import (
	"net/http"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// TokenRequest 创建 Token 请求
type TokenRequest struct {
	Name        string   `json:"name" binding:"required"`
	Permissions []string `json:"permissions" binding:"required"`
}

// TokenResponse Token 响应
type TokenResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Token       string   `json:"token,omitempty"` // 仅创建时返回
	Permissions []string `json:"permissions"`
	CreatedAt   string   `json:"createdAt"`
}

// GetTokens 获取所有 Token（不返回 token 值）
func GetTokens(c *gin.Context) {
	tokens := store.GetTokens()

	var result []TokenResponse
	for _, t := range tokens {
		result = append(result, TokenResponse{
			ID:          t.ID,
			Name:        t.Name,
			Permissions: t.Permissions,
			CreatedAt:   t.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, result)
}

// CreateToken 创建 Token
func CreateToken(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 验证权限值
	validPerms := map[string]bool{"read": true, "write": true, "delete": true}
	for _, p := range req.Permissions {
		if !validPerms[p] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的权限值: " + p})
			return
		}
	}

	token := &store.Token{
		Name:        req.Name,
		Permissions: req.Permissions,
	}

	if err := store.CreateToken(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建时返回完整的 token 值
	c.JSON(http.StatusCreated, TokenResponse{
		ID:          token.ID,
		Name:        token.Name,
		Token:       token.Token,
		Permissions: token.Permissions,
		CreatedAt:   token.CreatedAt,
	})
}

// DeleteToken 删除 Token
func DeleteToken(c *gin.Context) {
	id := c.Param("id")

	if err := store.DeleteToken(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
