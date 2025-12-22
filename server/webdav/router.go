package webdav

import (
	"context"
	"net/http"
	"sync"

	"fileflow/server/store"
)

var (
	// 全局锁系统实例
	globalLockSystem LockSystem
	lockSystemOnce   sync.Once
)

// getLockSystem 获取全局锁系统实例
func getLockSystem() LockSystem {
	lockSystemOnce.Do(func() {
		globalLockSystem = NewMemLS()
	})
	return globalLockSystem
}

/**
 * NewRouter 创建 WebDAV 路由器
 * 使用完整的 RFC 4918 实现
 */
func NewRouter() http.Handler {
	h := &Handler{
		Prefix:     "/webdav",
		LockSystem: getLockSystem(),
	}

	// 包装 Handler，注入存储和用户
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从上下文获取凭证和账户
		cred, ok := GetCredentialFromContext(r.Context())
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		acc, ok := GetAccountFromContext(r.Context())
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 创建存储适配器
		storage, err := NewS3Storage(acc)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// 创建用户包装器
		user := NewWebDAVUser(cred, acc)

		// 注入到上下文
		ctx := context.WithValue(r.Context(), storageKey, storage)
		ctx = context.WithValue(ctx, userKey, user)

		h.ServeHTTP(w, r.WithContext(ctx))
	})

	// 应用认证中间件
	return AuthMiddleware()(handler)
}

// NewRouterWithAccount 为指定账户创建 WebDAV 路由器（用于测试）
func NewRouterWithAccount(acc *store.Account, cred *store.WebDAVCredential) http.Handler {
	h := &Handler{
		Prefix:     "",
		LockSystem: getLockSystem(),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storage, err := NewS3Storage(acc)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		user := NewWebDAVUser(cred, acc)

		ctx := context.WithValue(r.Context(), storageKey, storage)
		ctx = context.WithValue(ctx, userKey, user)

		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
