package api

import (
	"context"
	"net/http"
	"strconv"

	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// AccountRequest 创建/更新账户请求
type AccountRequest struct {
	Name            string                   `json:"name" binding:"required"`
	IsActive        bool                     `json:"isActive"`
	Description     string                   `json:"description"`
	AccountID       string                   `json:"accountId" binding:"required"`
	AccessKeyId     string                   `json:"accessKeyId"`     // 更新时可选，空则保留原值
	SecretAccessKey string                   `json:"secretAccessKey"` // 更新时可选，空则保留原值
	BucketName      string                   `json:"bucketName" binding:"required"`
	Endpoint        string                   `json:"endpoint" binding:"required"`
	PublicDomain    string                   `json:"publicDomain" binding:"required"`
	APIToken        string                   `json:"apiToken"`
	Quota           store.Quota              `json:"quota" binding:"required"`
	Permissions     store.AccountPermissions `json:"permissions"`
}

// AccountResponse 账户响应（隐藏敏感字段）
type AccountResponse struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	IsActive     bool                     `json:"isActive"`
	Description  string                   `json:"description"`
	AccountID    string                   `json:"accountId"`
	BucketName   string                   `json:"bucketName"`
	Endpoint     string                   `json:"endpoint"`
	PublicDomain string                   `json:"publicDomain"`
	HasAPIToken  bool                     `json:"hasApiToken"`
	Quota        store.Quota              `json:"quota"`
	Usage        store.Usage              `json:"usage"`
	Permissions  store.AccountPermissions `json:"permissions"`
	UsagePercent float64                  `json:"usagePercent"`
	IsOverQuota  bool                     `json:"isOverQuota"`
	IsOverOps    bool                     `json:"isOverOps"`
	IsAvailable  bool                     `json:"isAvailable"`
	CreatedAt    string                   `json:"createdAt"`
	UpdatedAt    string                   `json:"updatedAt"`
}

// AccountFullResponse 账户完整响应（包含敏感字段，用于编辑）
type AccountFullResponse struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	IsActive        bool                     `json:"isActive"`
	Description     string                   `json:"description"`
	AccountID       string                   `json:"accountId"`
	AccessKeyId     string                   `json:"accessKeyId"`
	SecretAccessKey string                   `json:"secretAccessKey"`
	BucketName      string                   `json:"bucketName"`
	Endpoint        string                   `json:"endpoint"`
	PublicDomain    string                   `json:"publicDomain"`
	APIToken        string                   `json:"apiToken"`
	Quota           store.Quota              `json:"quota"`
	Usage           store.Usage              `json:"usage"`
	Permissions     store.AccountPermissions `json:"permissions"`
	UsagePercent    float64                  `json:"usagePercent"`
	IsOverQuota     bool                     `json:"isOverQuota"`
	IsOverOps       bool                     `json:"isOverOps"`
	IsAvailable     bool                     `json:"isAvailable"`
	CreatedAt       string                   `json:"createdAt"`
	UpdatedAt       string                   `json:"updatedAt"`
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
		Permissions:  acc.Permissions,
		UsagePercent: acc.GetUsagePercent(),
		IsOverQuota:  acc.IsOverQuota(),
		IsOverOps:    acc.IsOverOps(),
		IsAvailable:  acc.IsAvailable(),
		CreatedAt:    acc.CreatedAt,
		UpdatedAt:    acc.UpdatedAt,
	}
}

// toAccountFullResponse 转换为完整响应对象（包含敏感字段）
func toAccountFullResponse(acc *store.Account) AccountFullResponse {
	return AccountFullResponse{
		ID:              acc.ID,
		Name:            acc.Name,
		IsActive:        acc.IsActive,
		Description:     acc.Description,
		AccountID:       acc.AccountID,
		AccessKeyId:     acc.AccessKeyId,
		SecretAccessKey: acc.SecretAccessKey,
		BucketName:      acc.BucketName,
		Endpoint:        acc.Endpoint,
		PublicDomain:    acc.PublicDomain,
		APIToken:        acc.APIToken,
		Quota:           acc.Quota,
		Usage:           acc.Usage,
		Permissions:     acc.Permissions,
		UsagePercent:    acc.GetUsagePercent(),
		IsOverQuota:     acc.IsOverQuota(),
		IsOverOps:       acc.IsOverOps(),
		IsAvailable:     acc.IsAvailable(),
		CreatedAt:       acc.CreatedAt,
		UpdatedAt:       acc.UpdatedAt,
	}
}

// GetAccounts 获取账户列表（支持分页）
func GetAccounts(c *gin.Context) {
	pageStr := c.Query("page")
	pageSizeStr := c.Query("pageSize")

	// 如果没有分页参数，返回所有账户（兼容旧接口）
	if pageStr == "" && pageSizeStr == "" {
		accounts := store.GetAccounts()
		var result []AccountFullResponse
		for _, acc := range accounts {
			result = append(result, toAccountFullResponse(&acc))
		}
		c.JSON(http.StatusOK, result)
		return
	}

	// 分页获取
	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	pagedResult := store.GetAccountsPaged(page, pageSize)

	var items []AccountFullResponse
	for _, acc := range pagedResult.Items {
		items = append(items, toAccountFullResponse(&acc))
	}

	c.JSON(http.StatusOK, gin.H{
		"items":      items,
		"total":      pagedResult.Total,
		"page":       pagedResult.Page,
		"pageSize":   pagedResult.PageSize,
		"totalPages": pagedResult.TotalPages,
	})
}

// GetAccount 获取单个账户
func GetAccount(c *gin.Context) {
	id := c.Param("id")

	acc, err := store.GetAccountByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toAccountFullResponse(acc))
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

	// 如果权限未设置（全为false），使用默认权限
	permissions := req.Permissions
	if !permissions.WebDAV && !permissions.AutoUpload &&
		!permissions.APIUpload && !permissions.ClientUpload {
		permissions = store.DefaultAccountPermissions()
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
		Permissions:     permissions,
	}

	if err := store.CreateAccount(acc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建后返回完整信息（包含敏感字段）
	c.JSON(http.StatusCreated, toAccountFullResponse(acc))
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
	existing.Permissions = req.Permissions

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

	// 更新后返回完整信息（包含敏感字段）
	c.JSON(http.StatusOK, toAccountFullResponse(existing))
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
		// 同步所有账户（同步执行，等待完成）
		service.SyncAllAccountsUsage(context.Background())

		// 返回更新后的所有账户信息
		accounts := store.GetAccounts()
		var result []AccountFullResponse
		for _, acc := range accounts {
			result = append(result, toAccountFullResponse(&acc))
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetAccountsStats 获取账户统计信息
func GetAccountsStats(c *gin.Context) {
	stats := store.GetAccountsStats()
	c.JSON(http.StatusOK, stats)
}
