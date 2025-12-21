package webdav

import (
	"context"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

/**
 * MKCOL 处理函数
 * 创建目录（集合）
 */
func handleMkcol(w http.ResponseWriter, r *http.Request) {
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
	if urlPath == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 确保路径以 / 结尾
	if !strings.HasSuffix(urlPath, "/") {
		urlPath += "/"
	}

	key := urlPath

	// 创建 S3 客户端
	client, err := createS3Client(acc)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 检查目录是否已存在
	_, err = client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(acc.BucketName),
		Key:    aws.String(key),
	})

	if err == nil {
		// 目录已存在
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 创建空对象作为目录标记
	// 注意：S3 本身没有目录概念，这里创建一个以 / 结尾的空对象
	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(acc.BucketName),
		Key:         aws.String(key),
		Body:        strings.NewReader(""),
		ContentType: aws.String("application/x-directory"),
	})

	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
