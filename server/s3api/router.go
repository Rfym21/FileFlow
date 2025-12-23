package s3api

import (
	"github.com/gin-gonic/gin"
)

/**
 * SetupS3Router 配置 S3 API 路由
 * 支持双模式:
 * 1. Path Style: /s3/:bucket/*key
 * 2. Virtual Hosted Style: bucket.s3.example.com/*key（通过中间件处理）
 */
func SetupS3Router(r *gin.Engine) {
	// 虚拟主机风格中间件（全局）
	// 在所有路由之前检查，如果是虚拟主机请求则直接处理并中止
	r.Use(VirtualHostedStyleMiddleware())

	// Path Style 路由组: /s3/:bucket/*key
	s3Group := r.Group("/s3")
	s3Group.Use(S3AuthMiddleware())

	// Bucket 级别操作
	s3Group.GET("/:bucket", ListObjectsV2)
	s3Group.HEAD("/:bucket", HeadBucket)

	// Object 级别操作
	s3Group.GET("/:bucket/*key", handleGetObject)
	s3Group.HEAD("/:bucket/*key", HeadObject)
	s3Group.PUT("/:bucket/*key", handlePutObject)
	s3Group.DELETE("/:bucket/*key", handleDeleteObject)
	s3Group.POST("/:bucket/*key", handlePostObject)
}


// handleGetObject 处理 GET 请求（可能是 GetObject 或 ListParts）
func handleGetObject(c *gin.Context) {
	// 检查是否是 ListParts 请求
	if c.Query("uploadId") != "" {
		ListParts(c)
		return
	}
	GetObject(c)
}

// handlePutObject 处理 PUT 请求（可能是 PutObject、CopyObject 或 UploadPart）
func handlePutObject(c *gin.Context) {
	// 检查是否是 CopyObject 请求
	if c.GetHeader("x-amz-copy-source") != "" {
		// 检查是否是 UploadPartCopy
		if c.Query("uploadId") != "" && c.Query("partNumber") != "" {
			UploadPartCopy(c)
			return
		}
		CopyObject(c)
		return
	}

	// 检查是否是 UploadPart 请求
	if c.Query("uploadId") != "" && c.Query("partNumber") != "" {
		UploadPart(c)
		return
	}

	PutObject(c)
}

// handleDeleteObject 处理 DELETE 请求（可能是 DeleteObject 或 AbortMultipartUpload）
func handleDeleteObject(c *gin.Context) {
	// 检查是否是 AbortMultipartUpload 请求
	if c.Query("uploadId") != "" {
		AbortMultipartUpload(c)
		return
	}
	DeleteObject(c)
}

// handlePostObject 处理 POST 请求（Multipart 操作）
func handlePostObject(c *gin.Context) {
	// 初始化分片上传
	if c.Query("uploads") != "" {
		CreateMultipartUpload(c)
		return
	}

	// 完成分片上传
	if c.Query("uploadId") != "" {
		CompleteMultipartUpload(c)
		return
	}

	// 批量删除
	if c.Query("delete") != "" {
		DeleteObjects(c)
		return
	}

	WriteS3Error(c, ErrInvalidRequest)
}
