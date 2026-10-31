package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"fileflow/server/middleware"
	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

func uploadActor(c *gin.Context) service.UploadActor {
	user := middleware.CurrentUser(c)
	return service.UploadActor{UserID: user.ID, AuthVersion: user.AuthVersion, AuthType: c.GetString(middleware.ContextKeyAuthType), TokenID: c.GetString(middleware.ContextKeyTokenID)}
}

func decodeUploadRequest(c *gin.Context, destination any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "上传请求 JSON 参数无效", "code": "invalid_upload"})
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "上传请求只能包含一个 JSON 对象", "code": "invalid_upload"})
		return false
	}
	return true
}

func uploadError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "upload_failed"
	switch {
	case errors.Is(err, service.ErrUploadNotUploaded):
		status, code = http.StatusNotFound, "upload_not_uploaded"
	case errors.Is(err, store.ErrUploadSessionNotFound):
		status, code = http.StatusNotFound, "upload_task_not_found"
	case errors.Is(err, store.ErrInvalidUploadSession):
		status, code = http.StatusBadRequest, "invalid_upload"
	case errors.Is(err, store.ErrUploadSessionForbidden):
		status, code = http.StatusForbidden, "permission_denied"
	case errors.Is(err, store.ErrUploadSessionExpired):
		status, code = http.StatusGone, "upload_expired"
	case errors.Is(err, store.ErrUploadSessionConflict):
		status, code = http.StatusConflict, "upload_conflict"
	case errors.Is(err, store.ErrUploadQuotaExceeded):
		status, code = http.StatusTooManyRequests, "quota_exceeded"
	case errors.Is(err, store.ErrUploadSessionLimit):
		status, code = http.StatusTooManyRequests, "upload_task_limit"
	}
	c.JSON(status, gin.H{"error": err.Error(), "code": code})
}
