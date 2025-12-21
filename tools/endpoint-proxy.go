/**
 * FileFlow R2 Endpoint Proxy - Go 版本
 *
 * 用于反向代理 R2 公开文件，隐藏源站地址
 *
 * URL 格式：
 *   请求: http://localhost:8787/pub-xxx/path/to/file.png
 *   代理: https://pub-xxx.r2.dev/path/to/file.png
 *
 * 编译运行：
 *   go build -o endpoint-proxy endpoint-proxy.go
 *   ./endpoint-proxy
 *
 * 环境变量：
 *   PORT - 监听端口（默认 8787）
 *   ALLOWED_ORIGINS - 允许的 CORS 源（默认 *）
 */

package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

var allowedOrigins = getEnv("ALLOWED_ORIGINS", "*")

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// parsePath 解析请求路径，提取 subdomain 和文件路径
func parsePath(path string) (subdomain, filePath string, ok bool) {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "", "", false
	}

	idx := strings.Index(path, "/")
	if idx == -1 {
		return path, "", true
	}

	return path[:idx], path[idx:], true
}

// setCorsHeaders 设置 CORS 响应头
func setCorsHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}

	if allowedOrigins == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// proxyHandler 代理请求处理器
func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS
	setCorsHeaders(w, r)

	// 处理预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// 只允许 GET 和 HEAD
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// 解析路径
	subdomain, filePath, ok := parsePath(r.URL.Path)
	if !ok || subdomain == "" {
		http.Error(w, `{"error":"Invalid path"}`, http.StatusBadRequest)
		return
	}

	// 构建目标 URL
	targetURL := "https://" + subdomain + ".r2.dev" + filePath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// 创建代理请求
	proxyReq, err := http.NewRequest(r.Method, targetURL, nil)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		http.Error(w, `{"error":"Proxy failed"}`, http.StatusBadGateway)
		return
	}

	// 复制请求头
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("代理请求失败: %v", err)
		http.Error(w, `{"error":"Proxy failed"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("X-Proxy-By", "FileFlow-Go")

	// 写入状态码
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	io.Copy(w, resp.Body)
}

func main() {
	port := getEnv("PORT", "8787")

	http.HandleFunc("/", proxyHandler)

	log.Printf("FileFlow R2 Proxy listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
