package s3api

import (
	"strings"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

/**
 * VirtualHostedStyleMiddleware S3 虚拟主机风格中间件
 * 支持 bucket.s3.example.com/key 格式访问
 * 同时兼容 Path Style (/s3/bucket/key) 格式
 */
func VirtualHostedStyleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 排除非 S3 请求路径
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") ||
		   strings.HasPrefix(path, "/s3") ||
		   strings.HasPrefix(path, "/webdav") ||
		   strings.HasPrefix(path, "/assets") ||
		   strings.HasPrefix(path, "/guide") ||
		   path == "/favicon.svg" ||
		   path == "/favicon.ico" {
			c.Next()
			return
		}

		settings := store.GetSettings()

		// 如果启用了虚拟主机风格且配置了基础域名
		if settings.S3VirtualHostedStyle && settings.S3BaseDomain != "" {
			host := c.Request.Host

			// 移除端口号
			if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
				host = host[:colonIndex]
			}

			// 提取 bucket 名称
			bucket := extractBucketFromHost(host, settings.S3BaseDomain)

			// 如果成功提取 bucket，说明是虚拟主机风格请求
			if bucket != "" {
				// 注入 bucket 参数
				c.Params = append(c.Params, gin.Param{
					Key:   "bucket",
					Value: bucket,
				})

				// 将整个路径作为 key
				key := strings.TrimPrefix(c.Request.URL.Path, "/")
				if key != "" {
					c.Params = append(c.Params, gin.Param{
						Key:   "key",
						Value: "/" + key,
					})
				}

				// 标记为虚拟主机风格请求
				c.Set("s3_virtual_hosted_style", true)

				// 执行 S3 认证
				S3AuthMiddleware()(c)
				if c.IsAborted() {
					return
				}

				// 直接处理请求，不继续匹配其他路由
				handleVirtualHostedRequestDirect(c)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

/**
 * handleVirtualHostedRequestDirect 直接处理虚拟主机风格请求
 * 在中间件中调用，避免路由冲突
 */
func handleVirtualHostedRequestDirect(c *gin.Context) {
	// 根据 HTTP 方法和查询参数分发请求
	method := c.Request.Method
	key := c.Param("key")

	switch method {
	case "GET":
		if key == "" || key == "/" {
			// Bucket 级别操作
			if c.Query("uploadId") != "" {
				ListParts(c)
			} else {
				ListObjectsV2(c)
			}
		} else {
			// Object 级别操作
			handleGetObject(c)
		}
	case "HEAD":
		if key == "" || key == "/" {
			HeadBucket(c)
		} else {
			HeadObject(c)
		}
	case "PUT":
		handlePutObject(c)
	case "DELETE":
		handleDeleteObject(c)
	case "POST":
		handlePostObject(c)
	default:
		WriteS3Error(c, ErrMethodNotAllowed)
	}
}

/**
 * extractBucketFromHost 从 Host 头中提取 bucket 名称
 * 例如: my-bucket.s3.example.com -> my-bucket
 *       s3.example.com -> "" (基础域名，无 bucket)
 *       other-domain.com -> "" (非 S3 域名)
 */
func extractBucketFromHost(host, baseDomain string) string {
	// 标准化域名（转小写）
	host = strings.ToLower(host)
	baseDomain = strings.ToLower(baseDomain)

	// 如果 host 就是 baseDomain，没有 bucket
	if host == baseDomain {
		return ""
	}

	// 检查是否以 .baseDomain 结尾
	suffix := "." + baseDomain
	if !strings.HasSuffix(host, suffix) {
		return ""
	}

	// 提取 bucket 名称
	bucket := strings.TrimSuffix(host, suffix)

	// 验证 bucket 名称是否合法（DNS 规范）
	if !isValidBucketNameForVirtualHosted(bucket) {
		return ""
	}

	return bucket
}

/**
 * isValidBucketNameForVirtualHosted 验证 bucket 名称是否符合虚拟主机风格要求
 * 虚拟主机风格要求 bucket 名称符合 DNS 规范
 */
func isValidBucketNameForVirtualHosted(bucket string) bool {
	// bucket 名称不能为空
	if bucket == "" {
		return false
	}

	// bucket 名称长度限制 3-63 字符
	if len(bucket) < 3 || len(bucket) > 63 {
		return false
	}

	// 只能包含小写字母、数字、连字符
	for _, ch := range bucket {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return false
		}
	}

	// 不能以连字符开头或结尾
	if bucket[0] == '-' || bucket[len(bucket)-1] == '-' {
		return false
	}

	return true
}
