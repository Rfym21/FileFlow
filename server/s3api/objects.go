package s3api

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

// PutObject 上传对象
func PutObject(c *gin.Context) {
	bucket, key := getBucketAndKey(c)

	// 验证权限
	cred := GetS3CredentialFromContext(c)
	if !cred.HasPermission("write") {
		WriteS3Error(c, ErrAccessDenied)
		return
	}

	// 获取账户
	acc, err := getAccountForBucket(c, bucket)
	if err != nil {
		return
	}

	// 获取请求体
	body := c.Request.Body
	defer body.Close()

	contentType := c.GetHeader("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 调用 R2 上传
	client := getS3ClientForAccount(acc)

	input := &s3.PutObjectInput{
		Bucket:      aws.String(acc.BucketName),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}

	// 处理可选头部
	if contentLength := c.GetHeader("Content-Length"); contentLength != "" {
		// Content-Length 会被自动处理
	}
	if contentMD5 := c.GetHeader("Content-MD5"); contentMD5 != "" {
		input.ContentMD5 = aws.String(contentMD5)
	}

	output, err := client.PutObject(c.Request.Context(), input)
	if err != nil {
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	// 返回成功响应
	c.Header("ETag", aws.ToString(output.ETag))

	// 转发 checksum 头部（如果存在）
	if output.ChecksumCRC32 != nil {
		c.Header("x-amz-checksum-crc32", aws.ToString(output.ChecksumCRC32))
	}
	if output.ChecksumCRC32C != nil {
		c.Header("x-amz-checksum-crc32c", aws.ToString(output.ChecksumCRC32C))
	}
	if output.ChecksumSHA1 != nil {
		c.Header("x-amz-checksum-sha1", aws.ToString(output.ChecksumSHA1))
	}
	if output.ChecksumSHA256 != nil {
		c.Header("x-amz-checksum-sha256", aws.ToString(output.ChecksumSHA256))
	}

	c.Status(http.StatusOK)
}

// GetObject 获取对象
func GetObject(c *gin.Context) {
	bucket, key := getBucketAndKey(c)

	// 验证权限
	cred := GetS3CredentialFromContext(c)
	if !cred.HasPermission("read") {
		WriteS3Error(c, ErrAccessDenied)
		return
	}

	// 获取账户
	acc, err := getAccountForBucket(c, bucket)
	if err != nil {
		return
	}

	// 调用 R2 获取对象
	client := getS3ClientForAccount(acc)

	input := &s3.GetObjectInput{
		Bucket: aws.String(acc.BucketName),
		Key:    aws.String(key),
	}

	// 处理 Range 请求
	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		input.Range = aws.String(rangeHeader)
	}

	output, err := client.GetObject(c.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") {
			WriteS3Error(c, ErrNoSuchKey)
			return
		}
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}
	defer output.Body.Close()

	// 设置响应头
	if output.ContentType != nil {
		c.Header("Content-Type", aws.ToString(output.ContentType))
	}
	if output.ContentLength != nil {
		c.Header("Content-Length", fmt.Sprintf("%d", *output.ContentLength))
	}
	if output.ETag != nil {
		c.Header("ETag", aws.ToString(output.ETag))
	}
	if output.LastModified != nil {
		c.Header("Last-Modified", output.LastModified.Format(http.TimeFormat))
	}
	if output.ContentRange != nil {
		c.Header("Content-Range", aws.ToString(output.ContentRange))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}

	// 转发 checksum 头部（如果存在）
	if output.ChecksumCRC32 != nil {
		c.Header("x-amz-checksum-crc32", aws.ToString(output.ChecksumCRC32))
	}
	if output.ChecksumCRC32C != nil {
		c.Header("x-amz-checksum-crc32c", aws.ToString(output.ChecksumCRC32C))
	}
	if output.ChecksumSHA1 != nil {
		c.Header("x-amz-checksum-sha1", aws.ToString(output.ChecksumSHA1))
	}
	if output.ChecksumSHA256 != nil {
		c.Header("x-amz-checksum-sha256", aws.ToString(output.ChecksumSHA256))
	}

	// 流式传输响应体
	io.Copy(c.Writer, output.Body)
}

// HeadObject 获取对象元数据
func HeadObject(c *gin.Context) {
	bucket, key := getBucketAndKey(c)

	// 验证权限
	cred := GetS3CredentialFromContext(c)
	if !cred.HasPermission("read") {
		WriteS3Error(c, ErrAccessDenied)
		return
	}

	// 获取账户
	acc, err := getAccountForBucket(c, bucket)
	if err != nil {
		return
	}

	// 调用 R2 获取对象元数据
	client := getS3ClientForAccount(acc)

	input := &s3.HeadObjectInput{
		Bucket: aws.String(acc.BucketName),
		Key:    aws.String(key),
	}

	output, err := client.HeadObject(c.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			WriteS3Error(c, ErrNoSuchKey)
			return
		}
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	// 设置响应头
	if output.ContentType != nil {
		c.Header("Content-Type", aws.ToString(output.ContentType))
	}
	if output.ContentLength != nil {
		c.Header("Content-Length", fmt.Sprintf("%d", *output.ContentLength))
	}
	if output.ETag != nil {
		c.Header("ETag", aws.ToString(output.ETag))
	}
	if output.LastModified != nil {
		c.Header("Last-Modified", output.LastModified.Format(http.TimeFormat))
	}

	// 转发 checksum 头部（如果存在）
	if output.ChecksumCRC32 != nil {
		c.Header("x-amz-checksum-crc32", aws.ToString(output.ChecksumCRC32))
	}
	if output.ChecksumCRC32C != nil {
		c.Header("x-amz-checksum-crc32c", aws.ToString(output.ChecksumCRC32C))
	}
	if output.ChecksumSHA1 != nil {
		c.Header("x-amz-checksum-sha1", aws.ToString(output.ChecksumSHA1))
	}
	if output.ChecksumSHA256 != nil {
		c.Header("x-amz-checksum-sha256", aws.ToString(output.ChecksumSHA256))
	}

	c.Status(http.StatusOK)
}

// DeleteObject 删除对象
func DeleteObject(c *gin.Context) {
	bucket, key := getBucketAndKey(c)

	// 验证权限
	cred := GetS3CredentialFromContext(c)
	if !cred.HasPermission("delete") {
		WriteS3Error(c, ErrAccessDenied)
		return
	}

	// 获取账户
	acc, err := getAccountForBucket(c, bucket)
	if err != nil {
		return
	}

	// 调用 R2 删除对象
	client := getS3ClientForAccount(acc)

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(acc.BucketName),
		Key:    aws.String(key),
	}

	_, err = client.DeleteObject(c.Request.Context(), input)
	if err != nil {
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// DeleteObjects 批量删除对象
func DeleteObjects(c *gin.Context) {
	bucket := c.Param("bucket")

	// 验证权限
	cred := GetS3CredentialFromContext(c)
	if !cred.HasPermission("delete") {
		WriteS3Error(c, ErrAccessDenied)
		return
	}

	// 获取账户
	acc, err := getAccountForBucket(c, bucket)
	if err != nil {
		return
	}

	// 解析请求体
	var deleteReq DeleteRequest
	if err := xml.NewDecoder(c.Request.Body).Decode(&deleteReq); err != nil {
		WriteS3Error(c, ErrMalformedXML)
		return
	}

	// 调用 R2 批量删除
	client := getS3ClientForAccount(acc)

	var objects []string
	for _, obj := range deleteReq.Objects {
		objects = append(objects, obj.Key)
	}

	result := DeleteResult{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
	}

	for _, key := range objects {
		input := &s3.DeleteObjectInput{
			Bucket: aws.String(acc.BucketName),
			Key:    aws.String(key),
		}

		_, err := client.DeleteObject(c.Request.Context(), input)
		if err != nil {
			result.Error = append(result.Error, DeleteError{
				Key:     key,
				Code:    "InternalError",
				Message: err.Error(),
			})
		} else {
			if !deleteReq.Quiet {
				result.Deleted = append(result.Deleted, DeletedObject{Key: key})
			}
		}
	}

	WriteS3XMLResponse(c, http.StatusOK, result)
}

// CopyObject 复制对象
func CopyObject(c *gin.Context) {
	bucket, key := getBucketAndKey(c)

	// 验证权限
	cred := GetS3CredentialFromContext(c)
	if !cred.HasPermission("write") || !cred.HasPermission("read") {
		WriteS3Error(c, ErrAccessDenied)
		return
	}

	// 解析源对象
	copySource := c.GetHeader("x-amz-copy-source")
	copySource, _ = url.QueryUnescape(copySource)
	copySource = strings.TrimPrefix(copySource, "/")

	parts := strings.SplitN(copySource, "/", 2)
	if len(parts) != 2 {
		WriteS3Error(c, ErrInvalidRequest)
		return
	}
	sourceBucket := parts[0]
	sourceKey := parts[1]

	// 获取源账户和目标账户
	sourceAcc, err := getAccountForBucket(c, sourceBucket)
	if err != nil {
		return
	}

	destAcc, err := getAccountForBucket(c, bucket)
	if err != nil {
		return
	}

	// 检查是否同一个账户
	if sourceAcc.ID != destAcc.ID {
		WriteS3ErrorWithMessage(c, ErrInvalidRequest, "Cross-account copy not supported")
		return
	}

	// 调用 R2 复制对象
	client := getS3ClientForAccount(sourceAcc)

	input := &s3.CopyObjectInput{
		Bucket:     aws.String(destAcc.BucketName),
		Key:        aws.String(key),
		CopySource: aws.String(sourceBucket + "/" + sourceKey),
	}

	output, err := client.CopyObject(c.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") {
			WriteS3Error(c, ErrNoSuchKey)
			return
		}
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	etag := aws.ToString(output.CopyObjectResult.ETag)
	etag = strings.Trim(etag, `"`)

	result := CopyObjectResult{
		LastModified: output.CopyObjectResult.LastModified.Format(time.RFC3339),
		ETag:         etag,
	}

	WriteS3XMLResponse(c, http.StatusOK, result)
}

// getAccountForBucket 根据 bucket 名称获取账户，并验证权限
func getAccountForBucket(c *gin.Context, bucketName string) (*store.Account, error) {
	cred := GetS3CredentialFromContext(c)
	acc := GetS3AccountFromContext(c)

	// 如果当前账户的 bucket 名称匹配，直接返回
	if acc != nil && acc.BucketName == bucketName {
		return acc, nil
	}

	// 否则通过 bucket 名称查找
	account, err := store.GetAccountByBucketName(bucketName)
	if err != nil {
		WriteS3Error(c, ErrNoSuchBucket)
		return nil, err
	}

	// 检查凭证是否有权访问该账户
	if cred.AccountID != account.ID {
		WriteS3Error(c, ErrAccessDenied)
		return nil, err
	}

	return account, nil
}
