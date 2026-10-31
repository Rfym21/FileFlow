package webdav

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"fileflow/server/middleware"
	"fileflow/server/store"
)

type contextKey string

const (
	credentialContextKey contextKey = "webdav_credential"
	accountContextKey    contextKey = "webdav_account"
)

/**
 * WebDAV HTTP Basic Auth 认证中间件
 * 解析 Authorization 头，验证用户名和密码
 */
func AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// 解析 Basic Auth
			const prefix = "Basic "
			if !strings.HasPrefix(auth, prefix) {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Base64 解码
			payload, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// 分割用户名和密码
			pair := strings.SplitN(string(payload), ":", 2)
			if len(pair) != 2 {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			username, password := pair[0], pair[1]

			// 验证凭证
			cred, err := store.GetWebDAVCredentialByUsername(username)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if cred.Password != password || !cred.IsActive {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			member, err := store.GetUserByID(cred.UserID)
			if err != nil || !member.IsActive {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Member is disabled", http.StatusUnauthorized)
				return
			}
			cred.Permissions = middleware.EffectivePermissions(member.Role, cred.Permissions)
			if len(cred.Permissions) == 0 {
				http.Error(w, "Permission denied", http.StatusForbidden)
				return
			}

			// 获取关联的账户
			acc, err := store.GetAccountByID(cred.AccountID)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// 检查账户是否启用
			if !acc.IsActive {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Account is disabled", http.StatusForbidden)
				return
			}

			// 检查账户是否具有 WebDAV 访问权限
			if !acc.CanWebDAV() {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Account does not have WebDAV permission", http.StatusForbidden)
				return
			}

			// 更新最后使用时间
			go store.UpdateWebDAVCredentialLastUsed(cred.ID)

			// 将凭证和账户信息存入上下文
			ctx := context.WithValue(r.Context(), credentialContextKey, cred)
			ctx = context.WithValue(ctx, accountContextKey, acc)

			response := &webDAVResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(response, r.WithContext(ctx))
			switch r.Method {
			case "PUT", "DELETE", "MKCOL", "COPY", "MOVE", "PROPPATCH", "LOCK", "UNLOCK":
				_ = store.AppendAuditEntry(&store.AuditEntry{UserID: member.ID, Username: member.Username, Action: "webdav." + strings.ToLower(r.Method), Target: acc.ID + ":" + r.URL.Path, Status: response.status})
			}
		})
	}
}

type webDAVResponseWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *webDAVResponseWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.written = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *webDAVResponseWriter) Write(body []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

/**
 * 从请求上下文获取 WebDAV 凭证
 */
func GetCredentialFromContext(ctx context.Context) (*store.WebDAVCredential, bool) {
	cred, ok := ctx.Value(credentialContextKey).(*store.WebDAVCredential)
	return cred, ok
}

/**
 * 从请求上下文获取关联账户
 */
func GetAccountFromContext(ctx context.Context) (*store.Account, bool) {
	acc, ok := ctx.Value(accountContextKey).(*store.Account)
	return acc, ok
}

/**
 * 检查凭证是否具有指定权限
 */
func HasPermission(cred *store.WebDAVCredential, permission string) bool {
	if cred == nil {
		return false
	}
	return cred.HasPermission(permission)
}
