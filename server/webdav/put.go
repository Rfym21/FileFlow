package webdav

import (
	"context"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

/**
 * PUT 处理函数
 * 上传或更新文件
 */
func handlePut(w http.ResponseWriter, r *http.Request) {
	cred, ok := GetCredentialFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !HasPermission(cred, "write") {
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
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	key := strings.TrimPrefix(urlPath, "/")

	// 创建 S3 客户端
	client, err := createS3Client(acc)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 获取 Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = getContentType(key)
	}

	// 上传对象
	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(acc.BucketName),
		Key:         aws.String(key),
		Body:        r.Body,
		ContentType: aws.String(contentType),
	})

	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 返回 201 Created 或 204 No Content
	w.WriteHeader(http.StatusCreated)
}
