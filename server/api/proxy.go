package api

import (
	"io"
	"net/http"
	"time"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

var proxyClient = &http.Client{
	Timeout: 60 * time.Second,
}

// Proxy 反向代理 R2 文件
func Proxy(c *gin.Context) {
	settings := store.GetSettings()
	if !settings.EndpointProxy {
		c.JSON(http.StatusForbidden, gin.H{"error": "代理未启用"})
		return
	}

	subdomain := c.Param("subdomain")
	path := c.Param("path")

	if subdomain == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 构建原始 R2 URL
	targetURL := "https://" + subdomain + ".r2.dev" + path

	// 创建代理请求
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", targetURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建请求失败"})
		return
	}

	// 转发部分请求头
	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	if ifNoneMatch := c.GetHeader("If-None-Match"); ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	if ifModifiedSince := c.GetHeader("If-Modified-Since"); ifModifiedSince != "" {
		req.Header.Set("If-Modified-Since", ifModifiedSince)
	}

	// 发送请求
	resp, err := proxyClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "请求上游失败"})
		return
	}
	defer resp.Body.Close()

	// 转发响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 添加缓存头
	c.Header("Cache-Control", "public, max-age=31536000")

	// 流式传输响应
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
