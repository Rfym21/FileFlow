package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"fileflow/server/middleware"
	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

func directUploadActor(c *gin.Context) service.UploadActor {
	user := middleware.CurrentUser(c)
	return service.UploadActor{UserID: user.ID, AuthVersion: user.AuthVersion, AuthType: c.GetString(middleware.ContextKeyAuthType), TokenID: c.GetString(middleware.ContextKeyTokenID)}
}

func decodeDirectUpload(c *gin.Context, destination any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "直传请求 JSON 参数无效", "code": "invalid_upload"})
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "直传请求只能包含一个 JSON 对象", "code": "invalid_upload"})
		return false
	}
	return true
}

func directUploadError(c *gin.Context, err error) {
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

func InitDirectUpload(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	var request service.DirectUploadRequest
	if !decodeDirectUpload(c, &request) {
		return
	}
	result, err := service.InitDirectUpload(c.Request.Context(), directUploadActor(c), request)
	if err != nil {
		directUploadError(c, err)
		return
	}
	c.Set("audit_target", result.AccountID+"/"+result.Key)
	c.JSON(http.StatusOK, result)
}

func CompleteDirectUpload(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	var request struct {
		UploadID string `json:"uploadId"`
	}
	if !decodeDirectUpload(c, &request) {
		return
	}
	if strings.TrimSpace(request.UploadID) == "" || len(request.UploadID) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的直传任务 ID", "code": "invalid_upload"})
		return
	}
	result, changed, err := service.CompleteDirectUpload(c.Request.Context(), directUploadActor(c), request.UploadID)
	if err != nil {
		directUploadError(c, err)
		return
	}
	if !changed {
		c.Set("audit_skip", true)
	}
	c.Set("audit_target", result.ID+"/"+result.Key)
	c.JSON(http.StatusOK, result)
}
