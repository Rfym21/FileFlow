package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fileflow/server/service"

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
		// 上传到指定账户
		result, err = service.UploadToAccount(c.Request.Context(), accountID, key, file, contentType)
	} else {
		// 智能上传（自动选择账户）
		result, err = service.SmartUpload(c.Request.Context(), key, file, header.Size, contentType)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
