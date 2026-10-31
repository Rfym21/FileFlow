package api

import (
	"net/http"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// WebDAVCredentialRequest 创建 WebDAV 凭证请求
type WebDAVCredentialRequest struct {
	UserID      string   `json:"userId"`
	AccountID   string   `json:"accountId" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	Username    string   `json:"username"`
	Password    string   `json:"password"`
}

// WebDAVCredentialUpdateRequest 更新 WebDAV 凭证请求
type WebDAVCredentialUpdateRequest struct {
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	IsActive    bool     `json:"isActive"`
}

/**
 * 获取所有 WebDAV 凭证
 */
func GetWebDAVCredentials(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	credentials := store.GetWebDAVCredentials()
	visible := make([]store.WebDAVCredential, 0, len(credentials))
	for _, credential := range credentials {
		if canManageCredentialUser(c, credential.UserID) {
			visible = append(visible, credential)
		}
	}
	c.JSON(http.StatusOK, gin.H{"credentials": visible})
}

/**
 * 创建 WebDAV 凭证
 */
func CreateWebDAVCredential(c *gin.Context) {
	var req WebDAVCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 验证账户是否存在且具有 WebDAV 权限
	acc, err := store.GetAccountByID(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账户不存在"})
		return
	}

	if !acc.CanWebDAV() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该账户未启用 WebDAV 权限，无法创建 WebDAV 凭证"})
		return
	}

	// 验证权限列表
	validPerms := map[string]bool{"read": true, "write": true, "delete": true}
	for _, perm := range req.Permissions {
		if !validPerms[perm] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的权限: " + perm})
			return
		}
	}

	user, ok := credentialMember(c, req.UserID, req.Permissions)
	if !ok {
		return
	}
	cred := &store.WebDAVCredential{
		UserID:      user.ID,
		AccountID:   req.AccountID,
		Description: req.Description,
		Permissions: req.Permissions,
		Username:    req.Username,
		Password:    req.Password,
	}

	if err := store.CreateWebDAVCredential(cred); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Set("audit_target", cred.ID)
	c.JSON(http.StatusOK, gin.H{
		"message":    "创建成功",
		"credential": cred,
	})
}

/**
 * 更新 WebDAV 凭证
 */
func UpdateWebDAVCredential(c *gin.Context) {
	id := c.Param("id")

	var req WebDAVCredentialUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 验证权限列表
	validPerms := map[string]bool{"read": true, "write": true, "delete": true}
	for _, perm := range req.Permissions {
		if !validPerms[perm] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的权限: " + perm})
			return
		}
	}

	cred, err := store.GetWebDAVCredentialByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "WebDAV 凭证不存在"})
		return
	}
	if !canManageCredentialUser(c, cred.UserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权管理该成员的凭证"})
		return
	}
	if req.IsActive {
		if _, ok := credentialMember(c, cred.UserID, req.Permissions); !ok {
			return
		}
	}
	updates := &store.WebDAVCredential{
		Description: req.Description,
		Permissions: req.Permissions,
		IsActive:    req.IsActive,
	}

	if err := store.UpdateWebDAVCredential(id, updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

/**
 * 删除 WebDAV 凭证
 */
func DeleteWebDAVCredential(c *gin.Context) {
	id := c.Param("id")
	if credential, err := store.GetWebDAVCredentialByID(id); err == nil && !canManageCredentialUser(c, credential.UserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权管理该成员的凭证"})
		return
	}

	if err := store.DeleteWebDAVCredential(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
