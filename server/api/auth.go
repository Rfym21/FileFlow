package api

import (
	"fileflow/server/middleware"
	"fileflow/server/store"
	"github.com/gin-gonic/gin"
	"net/http"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type UserProfile struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	IsActive    bool   `json:"isActive"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func userProfile(user *store.User) UserProfile {
	return UserProfile{user.ID, user.Username, user.DisplayName, user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt}
}

type LoginResponse struct {
	Token       string      `json:"token"`
	Profile     UserProfile `json:"profile"`
	Permissions []string    `json:"permissions"`
}

func Login(c *gin.Context) {
	var request LoginRequest
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	user, err := store.AuthenticateUser(request.Username, request.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误，或成员已停用"})
		return
	}
	token, err := middleware.GenerateJWTForUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}
	c.Set(middleware.ContextKeyUser, user.Username)
	c.Set(middleware.ContextKeyUserID, user.ID)
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusOK, LoginResponse{token, userProfile(user), store.PermissionsForRole(user.Role)})
}
func Check(c *gin.Context) {
	response := gin.H{"valid": true, "authType": c.GetString(middleware.ContextKeyAuthType), "permissions": middleware.Permissions(c)}
	if user := middleware.CurrentUser(c); user != nil {
		response["user"] = user.Username
		response["profile"] = userProfile(user)
	}
	if c.GetString(middleware.ContextKeyAuthType) == middleware.AuthTypeToken {
		response["tokenId"] = c.GetString(middleware.ContextKeyTokenID)
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusOK, response)
}
func ChangePassword(c *gin.Context) {
	var request struct {
		CurrentPassword string `json:"currentPassword" binding:"required"`
		NewPassword     string `json:"newPassword" binding:"required"`
	}
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if err := store.ChangeUserPassword(middleware.CurrentUser(c).ID, request.CurrentPassword, request.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "密码已修改，请重新登录"})
}
func Logout(c *gin.Context) {
	if err := store.RevokeUserSessions(middleware.CurrentUser(c).ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "退出登录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已退出该成员的所有登录会话"})
}
func Health(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "healthy"}) }
