package api

import (
	"net/http"

	"fileflow/server/config"
	"fileflow/server/middleware"

	"github.com/gin-gonic/gin"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string `json:"token"`
}

// Login 管理员登录
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	cfg := config.Get()

	// 验证用户名和密码（明文比较）
	if req.Username != cfg.AdminUser || req.Password != cfg.AdminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 生成 JWT
	token, err := middleware.GenerateJWT(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{Token: token})
}

// Check 验证当前认证状态
func Check(c *gin.Context) {
	authType := c.GetString(middleware.ContextKeyAuthType)

	response := gin.H{
		"valid":    true,
		"authType": authType,
	}

	if authType == middleware.AuthTypeJWT {
		response["user"] = c.GetString(middleware.ContextKeyUser)
	} else {
		response["tokenId"] = c.GetString(middleware.ContextKeyTokenID)
	}

	c.JSON(http.StatusOK, response)
}

// Health 健康检查端点
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}
