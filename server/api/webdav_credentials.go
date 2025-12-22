package api

import (
	"net/http"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// WebDAVCredentialRequest 创建 WebDAV 凭证请求
type WebDAVCredentialRequest struct {
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
	credentials := store.GetWebDAVCredentials()
	c.JSON(http.StatusOK, gin.H{"credentials": credentials})
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

	cred := &store.WebDAVCredential{
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

	if err := store.DeleteWebDAVCredential(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
