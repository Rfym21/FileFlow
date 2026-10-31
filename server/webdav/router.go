package webdav

import (
	"context"
	"net/http"
	"sync"

	"fileflow/server/store"
)

var (
	accountLockSystems = make(map[string]LockSystem)
	accountLockMutex   sync.Mutex
)

// Members sharing an account share its locks; separate buckets do not block each other.
func getAccountLockSystem(accountID string) LockSystem {
	accountLockMutex.Lock()
	defer accountLockMutex.Unlock()
	if locks := accountLockSystems[accountID]; locks != nil {
		return locks
	}
	locks := NewMemLS()
	accountLockSystems[accountID] = locks
	return locks
}

/**
 * NewRouter 创建 WebDAV 路由器
 * 使用完整的 RFC 4918 实现
 */
func NewRouter() http.Handler {
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

		h := &Handler{Prefix: "/webdav", LockSystem: getAccountLockSystem(acc.ID)}
		h.ServeHTTP(w, r.WithContext(ctx))
	})

	// 应用认证中间件
	return AuthMiddleware()(handler)
}

// NewRouterWithAccount 为指定账户创建 WebDAV 路由器（用于测试）
func NewRouterWithAccount(acc *store.Account, cred *store.WebDAVCredential) http.Handler {
	h := &Handler{
		Prefix:     "",
		LockSystem: getAccountLockSystem(acc.ID),
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
