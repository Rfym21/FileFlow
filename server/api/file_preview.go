package api

import (
	"errors"
	"net/http"

	"fileflow/server/service"

	"github.com/aws/smithy-go"
	"github.com/gin-gonic/gin"
)

// GetFilePreview 返回文件预览类型、短期签名链接以及有限的文本内容。
func GetFilePreview(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	result, err := service.GetFilePreview(c.Request.Context(), getFirstID(c.Query("idGroup")), c.Query("key"))
	if err != nil {
		status := http.StatusInternalServerError
		var apiError smithy.APIError
		if errors.Is(err, service.ErrInvalidFilePreview) {
			status = http.StatusBadRequest
		} else if errors.Is(err, service.ErrPreviewAccountNotFound) || (errors.As(err, &apiError) && (apiError.ErrorCode() == "NoSuchKey" || apiError.ErrorCode() == "NotFound")) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
