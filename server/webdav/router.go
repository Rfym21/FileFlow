package webdav

import (
	"net/http"
)

/**
 * 创建 WebDAV 路由器
 * 处理所有 WebDAV 协议请求
 */
func NewRouter() http.Handler {
	// 直接创建处理函数，不使用 ServeMux
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "OPTIONS":
			handleOptions(w, r)
		case "PROPFIND":
			handlePropfind(w, r)
		case "GET", "HEAD":
			handleGet(w, r)
		case "PUT":
			handlePut(w, r)
		case "DELETE":
			handleDelete(w, r)
		case "MKCOL":
			handleMkcol(w, r)
		case "COPY":
			handleCopy(w, r)
		case "MOVE":
			handleMove(w, r)
		case "LOCK":
			handleLock(w, r)
		case "UNLOCK":
			handleUnlock(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// 应用认证中间件
	return AuthMiddleware()(handler)
}

/**
 * OPTIONS 请求处理
 * 返回支持的 WebDAV 方法
 */
func handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1, 2")
	w.Header().Set("Allow", "OPTIONS, GET, HEAD, PUT, DELETE, PROPFIND, MKCOL, COPY, MOVE, LOCK, UNLOCK")
	w.Header().Set("MS-Author-Via", "DAV")
	w.WriteHeader(http.StatusNoContent)
}
