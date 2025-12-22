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

	// 验证默认文件到期天数（0 表示永久，最大 3650 天 = 10 年）
	if settings.DefaultExpirationDays < 0 {
		settings.DefaultExpirationDays = 0
	}
	if settings.DefaultExpirationDays > 3650 {
		settings.DefaultExpirationDays = 3650
	}

	// 验证过期检查间隔（60-1440 分钟，即 1-24 小时）
	if settings.ExpirationCheckMinutes < 60 {
		settings.ExpirationCheckMinutes = 60
	}
	if settings.ExpirationCheckMinutes > 1440 {
		settings.ExpirationCheckMinutes = 1440
	}

	if err := store.UpdateSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 重载调度器
	service.ReloadScheduler()

	c.JSON(http.StatusOK, settings)
}
