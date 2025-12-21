package webdav

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

/**
 * GET/HEAD 处理函数
 * 下载文件或获取文件信息
 */
func handleGet(w http.ResponseWriter, r *http.Request) {
	cred, ok := GetCredentialFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !HasPermission(cred, "read") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	acc, ok := GetAccountFromContext(r.Context())
	if !ok {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 解析路径
	urlPath := strings.TrimPrefix(r.URL.Path, "/webdav")
	urlPath = strings.TrimPrefix(urlPath, "/")
	if urlPath == "" || strings.HasSuffix(urlPath, "/") {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	key := strings.TrimPrefix(urlPath, "/")

	// 创建 S3 客户端
	client, err := createS3Client(acc)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 获取对象
	getResp, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(acc.BucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer getResp.Body.Close()

	// 设置响应头
	if getResp.ContentType != nil {
		w.Header().Set("Content-Type", *getResp.ContentType)
	} else {
		w.Header().Set("Content-Type", getContentType(key))
	}

	if getResp.ContentLength != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(*getResp.ContentLength, 10))
	}

	if getResp.LastModified != nil {
		w.Header().Set("Last-Modified", getResp.LastModified.UTC().Format(http.TimeFormat))
	}

	if getResp.ETag != nil {
		w.Header().Set("ETag", *getResp.ETag)
	}

	// HEAD 请求只返回头部
	if r.Method == "HEAD" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// GET 请求返回内容
	w.WriteHeader(http.StatusOK)
	io.Copy(w, getResp.Body)
}
