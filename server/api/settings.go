package api

import (
	"net/http"

	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// GetSettings 获取系统设置
func GetSettings(c *gin.Context) {
	settings := store.GetSettings()
	c.JSON(http.StatusOK, settings)
}

// UpdateSettings 更新系统设置
func UpdateSettings(c *gin.Context) {
	var settings store.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 验证同步间隔
	if settings.SyncInterval < 1 {
		settings.SyncInterval = 1
	}
	if settings.SyncInterval > 1440 {
		settings.SyncInterval = 1440 // 最大 24 小时
	}

	if err := store.UpdateSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 重载调度器
	service.ReloadScheduler()

	c.JSON(http.StatusOK, settings)
}
