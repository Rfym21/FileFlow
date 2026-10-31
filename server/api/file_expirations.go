package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

type updateFileExpirationsRequest struct {
	IDs       []string `json:"ids"`
	ExpiresAt string   `json:"expiresAt"`
}

// UpdateFileExpiration 修改一个文件的到期时间。
func UpdateFileExpiration(c *gin.Context) {
	var req updateFileExpirationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	updateFileExpirations(c, []string{c.Param("id")}, req.ExpiresAt)
}

// UpdateFileExpirations 批量修改到期时间，任一记录不存在则整批不修改。
func UpdateFileExpirations(c *gin.Context) {
	var req updateFileExpirationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	updateFileExpirations(c, req.IDs, req.ExpiresAt)
}

func updateFileExpirations(c *gin.Context, ids []string, expiresAt string) {
	updatedCount, err := service.UpdateFileExpirations(ids, expiresAt)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrInvalidFileExpiration) {
			status = http.StatusBadRequest
		} else if errors.Is(err, store.ErrFileExpirationNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updatedCount": updatedCount})
}

type batchDeleteFileExpirationResult struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// BatchDeleteFileExpirations 逐个删除文件，失败的记录保留以便重试。
func BatchDeleteFileExpirations(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	ids, err := uniqueExpirationIDs(req.IDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results := make([]batchDeleteFileExpirationResult, 0, len(ids))
	successCount := 0
	for _, id := range ids {
		result := batchDeleteFileExpirationResult{ID: id}
		if err := service.DeleteScheduledFile(c.Request.Context(), id); err != nil {
			result.Error = err.Error()
		} else {
			result.Success = true
			successCount++
		}
		results = append(results, result)
	}
	c.JSON(http.StatusOK, gin.H{
		"results":      results,
		"successCount": successCount,
		"failCount":    len(results) - successCount,
	})
}

func uniqueExpirationIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("请选择要删除的文件")
	}
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("记录 ID 不能为空")
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result, nil
}
