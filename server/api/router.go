package api

import (
	"fileflow/server/middleware"

	"github.com/gin-gonic/gin"
)

// SetupRouter 配置路由
func SetupRouter(r *gin.Engine) {
	// 公开接口
	r.GET("/api/health", Health)
	r.POST("/api/auth/login", Login)

	// 反向代理（公开，用于代理 R2 文件）
	r.GET("/p/:subdomain/*path", Proxy)

	// 需要认证的接口
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	{
		// 通用认证检查
		protected.GET("/check", Check)

		// 文件操作（需要相应权限）
		protected.GET("/files", middleware.RequirePermission("read"), GetFiles)
		protected.POST("/upload", middleware.RequirePermission("write"), Upload)
		protected.DELETE("/file", middleware.RequirePermission("delete"), DeleteFile)
		protected.GET("/link", middleware.RequirePermission("read"), GetLink)
	}

	// 管理员专用接口（仅 JWT）
	admin := r.Group("/api")
	admin.Use(middleware.JWTOnlyMiddleware())
	{
		// 账户管理
		admin.GET("/accounts", GetAccounts)
		admin.GET("/accounts/stats", GetAccountsStats)
		admin.GET("/accounts/:id", GetAccount)
		admin.POST("/accounts", CreateAccount)
		admin.PUT("/accounts/:id", UpdateAccount)
		admin.DELETE("/accounts/:id", DeleteAccount)
		admin.POST("/accounts/sync", SyncAccounts)
		admin.POST("/accounts/:id/clear", ClearBucket)
		admin.POST("/accounts/delete-old-files", DeleteOldFiles)

		// Token 管理
		admin.GET("/tokens", GetTokens)
		admin.POST("/tokens", CreateToken)
		admin.DELETE("/tokens/:id", DeleteToken)

		// S3 凭证管理
		admin.GET("/s3-credentials", GetS3Credentials)
		admin.POST("/s3-credentials", CreateS3Credential)
		admin.PUT("/s3-credentials/:id", UpdateS3Credential)
		admin.DELETE("/s3-credentials/:id", DeleteS3Credential)

		// WebDAV 凭证管理
		admin.GET("/webdav-credentials", GetWebDAVCredentials)
		admin.POST("/webdav-credentials", CreateWebDAVCredential)
		admin.PUT("/webdav-credentials/:id", UpdateWebDAVCredential)
		admin.DELETE("/webdav-credentials/:id", DeleteWebDAVCredential)

		// 系统设置
		admin.GET("/settings", GetSettings)
		admin.PUT("/settings", UpdateSettings)
	}
}
