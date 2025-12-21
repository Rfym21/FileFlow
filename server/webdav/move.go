package webdav

import (
	"context"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

/**
 * MOVE 处理函数
 * 移动文件或目录到新位置
 */
func handleMove(w http.ResponseWriter, r *http.Request) {
	cred, ok := GetCredentialFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !HasPermission(cred, "write") || !HasPermission(cred, "delete") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	acc, ok := GetAccountFromContext(r.Context())
	if !ok {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 解析源路径
	srcPath := strings.TrimPrefix(r.URL.Path, "/webdav")
	srcPath = strings.TrimPrefix(srcPath, "/")
	if srcPath == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 解析目标路径
	destination := r.Header.Get("Destination")
	if destination == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 移除 /webdav 前缀
	destPath := destination
	if strings.Contains(destination, "://") {
		// 处理完整 URL: http://host/webdav/path
		parts := strings.Split(destination, "/webdav/")
		if len(parts) > 1 {
			destPath = parts[1]
		} else if strings.HasSuffix(destination, "/webdav") {
			destPath = ""
		}
	} else {
		// 处理相对路径: /webdav/path
		destPath = strings.TrimPrefix(destPath, "/webdav")
		destPath = strings.TrimPrefix(destPath, "/")
	}

	// 检查是否覆盖
	overwrite := r.Header.Get("Overwrite")
	if overwrite == "" {
		overwrite = "T"
	}

	// 创建 S3 客户端
	client, err := createS3Client(acc)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	srcKey := pathToKey(srcPath)
	destKey := pathToKey(destPath)

	// 检查目标是否存在
	destExists := objectExists(r.Context(), client, acc.BucketName, destKey)
	if destExists && overwrite == "F" {
		http.Error(w, "Precondition Failed", http.StatusPreconditionFailed)
		return
	}

	// 先复制
	_, err = client.CopyObject(context.Background(), &s3.CopyObjectInput{
		Bucket:     aws.String(acc.BucketName),
		Key:        aws.String(destKey),
		CopySource: aws.String(acc.BucketName + "/" + srcKey),
	})

	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 再删除源文件
	_, err = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(acc.BucketName),
		Key:    aws.String(srcKey),
	})

	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if destExists {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}
