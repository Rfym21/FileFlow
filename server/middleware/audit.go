package middleware

import (
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

// AuditMiddleware records mutations without copying request bodies or credentials.
func AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if skip, _ := c.Get("audit_skip"); skip == true {
			return
		}
		method := c.Request.Method
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			return
		}
		route := c.FullPath()
		if route == "" || !strings.HasPrefix(route, "/api/") {
			return
		}
		// Anonymous attempts must not trigger a full database snapshot save.
		// Successful logins populate this identity in the login handler.
		if c.GetString(ContextKeyUserID) == "" {
			return
		}
		target := c.GetString("audit_target")
		if target == "" {
			target = c.Param("id")
		}
		if key := c.Query("key"); target == "" && key != "" {
			target = c.Query("idGroup") + "/" + key
		}
		// Large object names are bounded while retaining valid UTF-8.
		if len(target) > 512 {
			target = target[:512]
			for !utf8.ValidString(target) {
				target = target[:len(target)-1]
			}
		}
		action := auditAction(method, route)
		if tool := c.GetString("audit_mcp_tool"); tool != "" {
			action = "mcp." + tool
		}
		entry := &store.AuditEntry{
			UserID:   c.GetString(ContextKeyUserID),
			Username: c.GetString(ContextKeyUser),
			Action:   action,
			Target:   target,
			Status:   c.Writer.Status(),
		}
		if err := store.AppendAuditEntry(entry); err != nil {
			// A logging failure cannot undo a completed remote file operation.
			log.Printf("[Audit] 操作记录保存失败: %v", err)
		}
	}
}

func auditAction(method, route string) string {
	labels := map[string]string{
		"POST /api/auth/login":                    "登录",
		"POST /api/auth/logout":                   "退出登录",
		"POST /api/auth/password":                 "修改个人密码",
		"POST /api/users":                         "添加团队成员",
		"PUT /api/users/:id":                      "修改团队成员",
		"POST /api/upload":                        "上传文件",
		"DELETE /api/file":                        "删除文件",
		"POST /api/accounts":                      "添加存储账号",
		"PUT /api/accounts/:id":                   "修改存储账号",
		"DELETE /api/accounts/:id":                "删除存储账号",
		"POST /api/accounts/sync":                 "同步存储用量",
		"POST /api/accounts/:id/clear":            "清空存储桶",
		"POST /api/accounts/delete-old-files":     "批量删除旧文件",
		"POST /api/tokens":                        "创建 API Token",
		"DELETE /api/tokens/:id":                  "删除 API Token",
		"POST /api/webdav-credentials":            "创建 WebDAV 凭证",
		"PUT /api/webdav-credentials/:id":         "修改 WebDAV 凭证",
		"DELETE /api/webdav-credentials/:id":      "删除 WebDAV 凭证",
		"PUT /api/settings":                       "修改系统设置",
		"PATCH /api/file-expirations":             "批量修改到期时间",
		"PATCH /api/file-expirations/:id":         "修改到期时间",
		"POST /api/file-expirations/batch-delete": "批量删除到期文件",
		"DELETE /api/file-expirations/:id":        "删除到期文件",
	}
	if label, ok := labels[method+" "+route]; ok {
		return label
	}
	return method + " " + route
}
