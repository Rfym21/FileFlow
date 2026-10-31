package api

import (
	"fileflow/server/middleware"
	"fileflow/server/store"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetUsers(c *gin.Context) {
	profiles := make([]UserProfile, 0)
	for _, user := range store.GetUsers() {
		profiles = append(profiles, userProfile(&user))
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusOK, gin.H{"users": profiles})
}
func validManagedRole(actor *store.User, role string) bool {
	if role == "member" || role == "viewer" {
		return true
	}
	return actor.Role == "owner" && role == "admin"
}
func CreateUser(c *gin.Context) {
	var request struct {
		Username    string `json:"username" binding:"required"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password" binding:"required"`
		Role        string `json:"role" binding:"required"`
	}
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if !validManagedRole(middleware.CurrentUser(c), request.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权创建该角色的成员"})
		return
	}
	user := &store.User{Username: request.Username, DisplayName: request.DisplayName, Role: request.Role, IsActive: true}
	if err := store.CreateUser(user, request.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Set("audit_target", user.ID+":"+user.Username)
	c.JSON(http.StatusCreated, userProfile(user))
}
func UpdateUser(c *gin.Context) {
	var request struct {
		DisplayName string `json:"displayName"`
		Role        string `json:"role" binding:"required"`
		IsActive    *bool  `json:"isActive" binding:"required"`
		Password    string `json:"password"`
	}
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	target, err := store.GetUserByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
		return
	}
	actor := middleware.CurrentUser(c)
	ownerSelfEdit := target.Role == "owner" && actor.ID == target.ID && request.Role == "owner" && *request.IsActive && request.Password == ""
	if !ownerSelfEdit && (target.Role == "owner" || !validManagedRole(actor, request.Role) || (actor.Role != "owner" && target.Role == "admin")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改该成员；所有者请通过个人信息修改密码"})
		return
	}
	if err := store.UpdateUser(target.ID, store.UserUpdate{DisplayName: request.DisplayName, Role: request.Role, IsActive: *request.IsActive, Password: request.Password}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := store.GetUserByID(target.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取成员失败"})
		return
	}
	c.JSON(http.StatusOK, userProfile(updated))
}
