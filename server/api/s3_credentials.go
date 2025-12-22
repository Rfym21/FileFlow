package api

import (
	"net/http"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// S3CredentialRequest 创建/更新 S3 凭证请求
type S3CredentialRequest struct {
	AccountID   string   `json:"accountId" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// S3CredentialUpdateRequest 更新 S3 凭证请求
type S3CredentialUpdateRequest struct {
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	IsActive    bool     `json:"isActive"`
}

// GetS3Credentials 获取所有 S3 凭证
func GetS3Credentials(c *gin.Context) {
	credentials := store.GetS3Credentials()
	c.JSON(http.StatusOK, gin.H{"credentials": credentials})
}

// CreateS3Credential 创建 S3 凭证
func CreateS3Credential(c *gin.Context) {
	var req S3CredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 验证账户是否存在且具有 S3 权限
	acc, err := store.GetAccountByID(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账户不存在"})
		return
	}

	if !acc.CanS3() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该账户未启用 S3 权限，无法创建 S3 凭证"})
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

	cred := &store.S3Credential{
		AccountID:   req.AccountID,
		Description: req.Description,
		Permissions: req.Permissions,
	}

	if err := store.CreateS3Credential(cred); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "创建成功",
		"credential": cred,
	})
}

// UpdateS3Credential 更新 S3 凭证
func UpdateS3Credential(c *gin.Context) {
	id := c.Param("id")

	var req S3CredentialUpdateRequest
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

	updates := &store.S3Credential{
		Description: req.Description,
		Permissions: req.Permissions,
		IsActive:    req.IsActive,
	}

	if err := store.UpdateS3Credential(id, updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteS3Credential 删除 S3 凭证
func DeleteS3Credential(c *gin.Context) {
	id := c.Param("id")

	if err := store.DeleteS3Credential(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
