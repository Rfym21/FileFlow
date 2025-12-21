package webdav

import (
	"context"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

/**
 * DELETE 处理函数
 * 删除文件或目录
 */
func handleDelete(w http.ResponseWriter, r *http.Request) {
	cred, ok := GetCredentialFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !HasPermission(cred, "delete") {
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

	// 创建 S3 客户端
	client, err := createS3Client(acc)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	isDir := strings.HasSuffix(urlPath, "/")

	if isDir {
		// 删除目录（删除所有以此前缀开头的对象）
		prefix := strings.TrimPrefix(urlPath, "/")
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}

		// 列出所有对象
		listResp, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
			Bucket: aws.String(acc.BucketName),
			Prefix: aws.String(prefix),
		})

		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// 批量删除
		for _, obj := range listResp.Contents {
			_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: aws.String(acc.BucketName),
				Key:    obj.Key,
			})
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		}
	} else {
		// 删除单个文件
		key := strings.TrimPrefix(urlPath, "/")
		_, err = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(acc.BucketName),
			Key:    aws.String(key),
		})

		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
