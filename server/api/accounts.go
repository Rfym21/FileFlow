package api

import (
	"context"
	"net/http"

	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// AccountRequest 创建/更新账户请求
type AccountRequest struct {
	Name            string      `json:"name" binding:"required"`
	IsActive        bool        `json:"isActive"`
	Description     string      `json:"description"`
	AccountID       string      `json:"accountId" binding:"required"`
	AccessKeyId     string      `json:"accessKeyId"`     // 更新时可选，空则保留原值
	SecretAccessKey string      `json:"secretAccessKey"` // 更新时可选，空则保留原值
	BucketName      string      `json:"bucketName" binding:"required"`
	Endpoint        string      `json:"endpoint" binding:"required"`
	PublicDomain    string      `json:"publicDomain" binding:"required"`
	APIToken        string      `json:"apiToken"`
	Quota           store.Quota `json:"quota" binding:"required"`
}

// AccountResponse 账户响应（隐藏敏感字段）
type AccountResponse struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	IsActive     bool        `json:"isActive"`
	Description  string      `json:"description"`
	AccountID    string      `json:"accountId"`
	BucketName   string      `json:"bucketName"`
	Endpoint     string      `json:"endpoint"`
	PublicDomain string      `json:"publicDomain"`
	HasAPIToken  bool        `json:"hasApiToken"`
	Quota        store.Quota `json:"quota"`
	Usage        store.Usage `json:"usage"`
	UsagePercent float64     `json:"usagePercent"`
	IsOverQuota  bool        `json:"isOverQuota"`
	IsOverOps    bool        `json:"isOverOps"`
	IsAvailable  bool        `json:"isAvailable"`
	CreatedAt    string      `json:"createdAt"`
	UpdatedAt    string      `json:"updatedAt"`
}

// toAccountResponse 转换为响应对象
func toAccountResponse(acc *store.Account) AccountResponse {
	return AccountResponse{
		ID:           acc.ID,
		Name:         acc.Name,
		IsActive:     acc.IsActive,
		Description:  acc.Description,
		AccountID:    acc.AccountID,
		BucketName:   acc.BucketName,
		Endpoint:     acc.Endpoint,
		PublicDomain: acc.PublicDomain,
		HasAPIToken:  acc.APIToken != "",
		Quota:        acc.Quota,
		Usage:        acc.Usage,
		UsagePercent: acc.GetUsagePercent(),
		IsOverQuota:  acc.IsOverQuota(),
		IsOverOps:    acc.IsOverOps(),
		IsAvailable:  acc.IsAvailable(),
		CreatedAt:    acc.CreatedAt,
		UpdatedAt:    acc.UpdatedAt,
	}
}

// GetAccounts 获取所有账户
func GetAccounts(c *gin.Context) {
	accounts := store.GetAccounts()

	var result []AccountResponse
	for _, acc := range accounts {
		result = append(result, toAccountResponse(&acc))
	}

	c.JSON(http.StatusOK, result)
}

// GetAccount 获取单个账户
func GetAccount(c *gin.Context) {
	id := c.Param("id")

	acc, err := store.GetAccountByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toAccountResponse(acc))
}

// CreateAccount 创建账户
func CreateAccount(c *gin.Context) {
	var req AccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 创建时必须提供密钥
	if req.AccessKeyId == "" || req.SecretAccessKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access Key ID 和 Secret Access Key 不能为空"})
		return
	}

	acc := &store.Account{
		Name:            req.Name,
		IsActive:        req.IsActive,
		Description:     req.Description,
		AccountID:       req.AccountID,
		AccessKeyId:     req.AccessKeyId,
		SecretAccessKey: req.SecretAccessKey,
		BucketName:      req.BucketName,
		Endpoint:        req.Endpoint,
		PublicDomain:    req.PublicDomain,
		APIToken:        req.APIToken,
		Quota:           req.Quota,
	}

	if err := store.CreateAccount(acc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toAccountResponse(acc))
}

// UpdateAccount 更新账户
func UpdateAccount(c *gin.Context) {
	id := c.Param("id")

	// 先获取现有账户
	existing, err := store.GetAccountByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req AccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 更新字段
	existing.Name = req.Name
	existing.IsActive = req.IsActive
	existing.Description = req.Description
	existing.AccountID = req.AccountID
	existing.BucketName = req.BucketName
	existing.Endpoint = req.Endpoint
	existing.PublicDomain = req.PublicDomain
	existing.Quota = req.Quota

	// 敏感字段：只有非空时才更新
	if req.AccessKeyId != "" {
		existing.AccessKeyId = req.AccessKeyId
	}
	if req.SecretAccessKey != "" {
		existing.SecretAccessKey = req.SecretAccessKey
	}
	if req.APIToken != "" {
		existing.APIToken = req.APIToken
	}

	if err := store.UpdateAccount(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toAccountResponse(existing))
}

// DeleteAccount 删除账户
func DeleteAccount(c *gin.Context) {
	id := c.Param("id")

	if err := store.DeleteAccount(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// SyncAccounts 同步账户使用量
func SyncAccounts(c *gin.Context) {
	accountID := c.Query("accountId")

	if accountID != "" {
		// 同步单个账户
		acc, err := store.GetAccountByID(accountID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		if err := service.SyncAccountUsage(context.Background(), acc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 重新获取更新后的账户信息
		acc, _ = store.GetAccountByID(accountID)
		c.JSON(http.StatusOK, toAccountResponse(acc))
	} else {
		// 同步所有账户
		go service.SyncAllAccountsUsage(context.Background())
		c.JSON(http.StatusOK, gin.H{"message": "同步任务已启动"})
	}
}
