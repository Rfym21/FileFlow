package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetFiles 获取文件列表（懒加载+分页）
func GetFiles(c *gin.Context) {
	idGroupStr := c.Query("idGroup")
	prefix := c.Query("prefix")
	cursor := c.Query("cursor")
	limitStr := c.DefaultQuery("limit", "50")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// 解析 idGroup（逗号分隔）
	var idGroup []string
	if idGroupStr != "" {
		for _, id := range strings.Split(idGroupStr, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				idGroup = append(idGroup, id)
			}
		}
	}

	if len(idGroup) > 0 {
		// 获取指定账户组的文件
		result, err := service.ListAccountsFilesByIDs(c.Request.Context(), idGroup, prefix, cursor, int32(limit))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	} else {
		// 获取所有账户的文件
		result, err := service.ListAllAccountsFiles(c.Request.Context(), prefix, cursor, int32(limit))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

// Upload 上传文件（可指定账户，不指定则智能选择）
func Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传文件"})
		return
	}
	defer file.Close()

	// 指定账户ID（可选，取第一个）
	idGroup := c.PostForm("idGroup")
	accountID := getFirstID(idGroup)

	// 解析到期天数（可选，默认使用系统设置）
	expirationDaysStr := c.PostForm("expirationDays")
	var expirationDays int
	if expirationDaysStr != "" {
		expirationDays, _ = strconv.Atoi(expirationDaysStr)
		// -1 表示使用默认设置，0 表示永久
		if expirationDays < -1 {
			expirationDays = -1
		}
	} else {
		expirationDays = -1 // 使用默认设置
	}

	// 生成文件路径
	customPath := c.PostForm("path")
	var key string
	if customPath != "" {
		// 自定义路径 + 原始文件名
		customPath = strings.TrimPrefix(customPath, "/")
		customPath = strings.TrimSuffix(customPath, "/")
		key = fmt.Sprintf("%s/%s", customPath, header.Filename)
	} else {
		// 默认按日期组织
		ext := filepath.Ext(header.Filename)
		key = fmt.Sprintf("%s/%s%s",
			time.Now().Format("2006/01/02"),
			uuid.New().String(),
			ext,
		)
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var result *service.UploadResult
	if accountID != "" {
		// 上传到指定账户（前端上传检查 client_upload 权限）
		result, err = service.UploadToAccountForClient(c.Request.Context(), accountID, key, file, contentType)
	} else {
		// 智能上传（自动选择具有 client_upload 权限的账户）
		result, err = service.SmartUploadForClient(c.Request.Context(), key, file, header.Size, contentType)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建文件到期记录
	if expirationDays == -1 {
		// 使用系统默认设置
		settings := store.GetSettings()
		expirationDays = settings.DefaultExpirationDays
	}
	if expirationDays > 0 {
		// expirationDays > 0 才创建到期记录，0 表示永久不过期
		if err := service.CreateFileExpirationRecord(result.ID, result.Key, expirationDays); err != nil {
			// 到期记录创建失败不影响上传结果，仅记录日志
			fmt.Printf("[Upload] 创建文件到期记录失败: %v\n", err)
		}
	}

	c.JSON(http.StatusOK, result)
}

// DeleteFile 删除文件
func DeleteFile(c *gin.Context) {
	idGroup := c.Query("idGroup")
	accountID := getFirstID(idGroup)
	key := c.Query("key")

	if accountID == "" || key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 idGroup 或 key 参数"})
		return
	}

	if err := service.DeleteFile(c.Request.Context(), accountID, key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 删除对应的到期记录（如果存在）
	service.DeleteFileExpirationRecord(accountID, key)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetLink 获取文件直链
func GetLink(c *gin.Context) {
	idGroup := c.Query("idGroup")
	accountID := getFirstID(idGroup)
	key := c.Query("key")

	if accountID == "" || key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 idGroup 或 key 参数"})
		return
	}

	url, err := service.GetFileLink(accountID, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

// getFirstID 从逗号分隔的 ID 列表中获取第一个 ID
func getFirstID(idGroup string) string {
	if idGroup == "" {
		return ""
	}
	ids := strings.Split(idGroup, ",")
	if len(ids) > 0 {
		return strings.TrimSpace(ids[0])
	}
	return ""
}

// ClearBucket 清空账户的存储桶
func ClearBucket(c *gin.Context) {
	accountID := c.Param("id")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少账户 ID"})
		return
	}

	if err := service.ClearBucket(c.Request.Context(), accountID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "清空成功"})
}

// DeleteOldFilesRequest 删除旧文件请求
type DeleteOldFilesRequest struct {
	AccountIDs []string `json:"accountIds" binding:"required"`
	BeforeDate string   `json:"beforeDate" binding:"required"`
}

// DeleteOldFiles 批量删除指定账户中早于指定时间的文件
func DeleteOldFiles(c *gin.Context) {
	var req DeleteOldFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if len(req.AccountIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择至少一个账户"})
		return
	}

	// 解析日期
	beforeDate, err := time.Parse("2006-01-02", req.BeforeDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "日期格式错误，请使用 YYYY-MM-DD 格式"})
		return
	}

	// 设置为当天结束时间（23:59:59）
	beforeDate = beforeDate.Add(24*time.Hour - time.Second)

	results := service.DeleteOldFilesMultiple(c.Request.Context(), req.AccountIDs, beforeDate)

	c.JSON(http.StatusOK, gin.H{
		"results": results,
	})
}

// FileExpirationResponse 文件到期记录响应（包含账户名）
type FileExpirationResponse struct {
	ID          string `json:"id"`
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	FileKey     string `json:"fileKey"`
	ExpiresAt   string `json:"expiresAt"`
	CreatedAt   string `json:"createdAt"`
}

// GetFileExpirations 获取文件到期列表
func GetFileExpirations(c *gin.Context) {
	expirations := store.GetFileExpirations()

	// 构建账户 ID -> 名称映射
	accounts := store.GetAccounts()
	accountMap := make(map[string]string)
	for _, acc := range accounts {
		accountMap[acc.ID] = acc.Name
	}

	// 转换为响应格式
	result := make([]FileExpirationResponse, 0, len(expirations))
	for _, exp := range expirations {
		accountName := accountMap[exp.AccountID]
		if accountName == "" {
			accountName = "未知账户"
		}
		result = append(result, FileExpirationResponse{
			ID:          exp.ID,
			AccountID:   exp.AccountID,
			AccountName: accountName,
			FileKey:     exp.FileKey,
			ExpiresAt:   exp.ExpiresAt,
			CreatedAt:   exp.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"expirations": result,
		"total":       len(result),
	})
}

// DeleteFileExpirationByID 删除单个到期记录（同时删除文件）
func DeleteFileExpirationByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少记录 ID"})
		return
	}

	// 查找到期记录
	expirations := store.GetFileExpirations()
	var target *store.FileExpiration
	for _, exp := range expirations {
		if exp.ID == id {
			target = &exp
			break
		}
	}

	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}

	// 删除 S3 文件
	if err := service.DeleteFile(c.Request.Context(), target.AccountID, target.FileKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文件失败: " + err.Error()})
		return
	}

	// 删除到期记录
	if err := store.DeleteFileExpirationByID(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除记录失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
