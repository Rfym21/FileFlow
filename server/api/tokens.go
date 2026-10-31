package api

import (
	"net/http"

	"fileflow/server/middleware"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// TokenRequest 创建 Token 请求
type TokenRequest struct {
	UserID      string   `json:"userId"`
	Name        string   `json:"name" binding:"required"`
	Permissions []string `json:"permissions" binding:"required"`
}

// TokenResponse Token 响应
type TokenResponse struct {
	UserID      string   `json:"userId"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Token       string   `json:"token"`
	Permissions []string `json:"permissions"`
	CreatedAt   string   `json:"createdAt"`
}

// GetTokens 获取所有 Token
func GetTokens(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	tokens := store.GetTokens()

	result := make([]TokenResponse, 0, len(tokens))
	for _, t := range tokens {
		if !canManageCredentialUser(c, t.UserID) {
			continue
		}
		result = append(result, TokenResponse{
			UserID:      t.UserID,
			ID:          t.ID,
			Name:        t.Name,
			Token:       t.Token,
			Permissions: t.Permissions,
			CreatedAt:   t.CreatedAt,
		})
		// Token-authenticated agents can enumerate credentials without acquiring
		// the bearer values of other, potentially more privileged integrations.
		if c.GetString(middleware.ContextKeyAuthType) == middleware.AuthTypeToken {
			result[len(result)-1].Token = ""
		}
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
	validPerms := map[string]bool{"read": true, "write": true, "delete": true, "manage": true}
	for _, p := range req.Permissions {
		if !validPerms[p] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的权限值: " + p})
			return
		}
	}
	for _, permission := range req.Permissions {
		if permission == "manage" && !store.HasAllFilePermissions(req.Permissions) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "MCP 管理权限必须同时授予 read、write、delete"})
			return
		}
	}

	user, ok := tokenCredentialMember(c, req.UserID, req.Permissions)
	if !ok {
		return
	}
	token := &store.Token{
		UserID:      user.ID,
		Name:        req.Name,
		Permissions: req.Permissions,
	}

	if err := store.CreateToken(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建时返回完整的 token 值
	c.Set("audit_target", token.ID)
	c.JSON(http.StatusCreated, TokenResponse{
		UserID:      token.UserID,
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
	for _, token := range store.GetTokens() {
		if token.ID == id && !canManageCredentialUser(c, token.UserID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权管理该成员的凭证"})
			return
		}
	}

	if err := store.DeleteToken(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
