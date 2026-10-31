package api

import (
	"net/http"
	"strconv"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// GetAuditLogs returns recent team activity to authorized administrators.
func GetAuditLogs(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit < 1 || limit > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "记录数量必须在 1 到 2000 之间"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": store.GetAuditEntries(limit)})
}
