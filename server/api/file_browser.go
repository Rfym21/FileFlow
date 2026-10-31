package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"fileflow/server/service"
	"fileflow/server/store"
	"github.com/gin-gonic/gin"
)

func GetFileBrowser(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 100 || len(c.Query("cursor")) > 6000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分页参数无效"})
		return
	}
	ids := []string{}
	seen := map[string]bool{}
	if raw := c.Query("idGroup"); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id != "" && !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}
	} else {
		for _, account := range store.GetActiveAccounts() {
			ids = append(ids, account.ID)
		}
	}
	if len(ids) > 20 || (len(ids) > 1 && c.Query("cursor") != "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "续页浏览请指定单个存储账号，每次最多查询 20 个账号"})
		return
	}
	result := make([]service.AccountFiles, 0, len(ids))
	for _, id := range ids {
		files, err := service.ListFlatAccountFiles(c.Request.Context(), id, c.Query("prefix"), c.Query("q"), c.Query("cursor"), int32(limit))
		if err != nil {
			fileBrowserError(c, err)
			return
		}
		result = append(result, files)
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusOK, result)
}

func BatchDeleteFiles(c *gin.Context) {
	var request struct {
		AccountID string   `json:"accountId" binding:"required"`
		Keys      []string `json:"keys" binding:"required"`
	}
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	results, err := service.BatchDeleteAccountFiles(c.Request.Context(), request.AccountID, request.Keys)
	if err != nil {
		fileBrowserError(c, err)
		return
	}
	success := 0
	for _, entry := range results {
		if entry.Success {
			success++
		}
	}
	c.Set("audit_target", request.AccountID)
	c.JSON(http.StatusOK, gin.H{"results": results, "successCount": success, "failCount": len(results) - success})
}

func UpdateFilesExpiration(c *gin.Context) {
	var request struct {
		AccountID string          `json:"accountId" binding:"required"`
		Keys      []string        `json:"keys" binding:"required"`
		ExpiresAt json.RawMessage `json:"expiresAt"`
	}
	if c.ShouldBindJSON(&request) != nil || len(request.ExpiresAt) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供账号、文件和 expiresAt（null 表示永久）"})
		return
	}
	var expiresAt *string
	if string(request.ExpiresAt) != "null" {
		var value string
		if json.Unmarshal(request.ExpiresAt, &value) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "到期时间格式无效"})
			return
		}
		expiresAt = &value
	}
	count, err := service.SetAccountFilesExpiration(c.Request.Context(), request.AccountID, request.Keys, expiresAt)
	if err != nil {
		fileBrowserError(c, err)
		return
	}
	c.Set("audit_target", request.AccountID)
	c.JSON(http.StatusOK, gin.H{"updatedCount": count})
}

func fileBrowserError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, store.ErrUploadQuotaExceeded) {
		status = http.StatusTooManyRequests
	}
	if errors.Is(err, service.ErrInvalidFileBrowser) || errors.Is(err, store.ErrInvalidFileExpiration) {
		status = http.StatusBadRequest
	}
	c.JSON(status, gin.H{"error": err.Error()})
}
