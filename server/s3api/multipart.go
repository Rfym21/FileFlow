package s3api

import (
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gin-gonic/gin"
)

// CreateMultipartUpload 初始化分片上传
func CreateMultipartUpload(c *gin.Context) {
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

	// 获取 Content-Type
	contentType := c.GetHeader("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 调用 R2 初始化分片上传
	client := getS3ClientForAccount(acc)

	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(acc.BucketName),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}

	output, err := client.CreateMultipartUpload(c.Request.Context(), input)
	if err != nil {
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	result := InitiateMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:   bucket,
		Key:      key,
		UploadId: aws.ToString(output.UploadId),
	}

	WriteS3XMLResponse(c, http.StatusOK, result)
}

// UploadPart 上传分片
func UploadPart(c *gin.Context) {
	bucket, key := getBucketAndKey(c)
	uploadID := c.Query("uploadId")
	partNumberStr := c.Query("partNumber")

	// 验证权限
	cred := GetS3CredentialFromContext(c)
	if !cred.HasPermission("write") {
		WriteS3Error(c, ErrAccessDenied)
		return
	}

	// 验证分片号
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		WriteS3Error(c, ErrInvalidPartNumber)
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

	// 调用 R2 上传分片
	client := getS3ClientForAccount(acc)

	input := &s3.UploadPartInput{
		Bucket:     aws.String(acc.BucketName),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)),
		Body:       body,
	}

	output, err := client.UploadPart(c.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchUpload") {
			WriteS3Error(c, ErrNoSuchUpload)
			return
		}
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	c.Header("ETag", aws.ToString(output.ETag))
	c.Status(http.StatusOK)
}

// UploadPartCopy 复制分片
func UploadPartCopy(c *gin.Context) {
	bucket, key := getBucketAndKey(c)
	uploadID := c.Query("uploadId")
	partNumberStr := c.Query("partNumber")

	// 验证权限
	cred := GetS3CredentialFromContext(c)
	if !cred.HasPermission("write") || !cred.HasPermission("read") {
		WriteS3Error(c, ErrAccessDenied)
		return
	}

	// 验证分片号
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		WriteS3Error(c, ErrInvalidPartNumber)
		return
	}

	// 获取账户
	acc, err := getAccountForBucket(c, bucket)
	if err != nil {
		return
	}

	// 解析源对象
	copySource := c.GetHeader("x-amz-copy-source")

	// 调用 R2 复制分片
	client := getS3ClientForAccount(acc)

	input := &s3.UploadPartCopyInput{
		Bucket:     aws.String(acc.BucketName),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)),
		CopySource: aws.String(copySource),
	}

	// 处理 Range
	if copyRange := c.GetHeader("x-amz-copy-source-range"); copyRange != "" {
		input.CopySourceRange = aws.String(copyRange)
	}

	output, err := client.UploadPartCopy(c.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchUpload") {
			WriteS3Error(c, ErrNoSuchUpload)
			return
		}
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	etag := aws.ToString(output.CopyPartResult.ETag)
	etag = strings.Trim(etag, `"`)

	result := CopyObjectResult{
		LastModified: output.CopyPartResult.LastModified.Format(time.RFC3339),
		ETag:         etag,
	}

	WriteS3XMLResponse(c, http.StatusOK, result)
}

// CompleteMultipartUpload 完成分片上传
func CompleteMultipartUpload(c *gin.Context) {
	bucket, key := getBucketAndKey(c)
	uploadID := c.Query("uploadId")

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

	// 解析请求体
	var completeReq CompleteMultipartUploadRequest
	if err := xml.NewDecoder(c.Request.Body).Decode(&completeReq); err != nil {
		WriteS3Error(c, ErrMalformedXML)
		return
	}

	// 转换分片信息
	var completedParts []types.CompletedPart
	for _, p := range completeReq.Parts {
		completedParts = append(completedParts, types.CompletedPart{
			PartNumber: aws.Int32(int32(p.PartNumber)),
			ETag:       aws.String(p.ETag),
		})
	}

	// 调用 R2 完成分片上传
	client := getS3ClientForAccount(acc)

	input := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(acc.BucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}

	output, err := client.CompleteMultipartUpload(c.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchUpload") {
			WriteS3Error(c, ErrNoSuchUpload)
			return
		}
		if strings.Contains(err.Error(), "InvalidPart") {
			WriteS3Error(c, ErrInvalidPart)
			return
		}
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	etag := aws.ToString(output.ETag)
	etag = strings.Trim(etag, `"`)

	result := CompleteMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: aws.ToString(output.Location),
		Bucket:   bucket,
		Key:      key,
		ETag:     etag,
	}

	WriteS3XMLResponse(c, http.StatusOK, result)
}

// AbortMultipartUpload 取消分片上传
func AbortMultipartUpload(c *gin.Context) {
	bucket, key := getBucketAndKey(c)
	uploadID := c.Query("uploadId")

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

	// 调用 R2 取消分片上传
	client := getS3ClientForAccount(acc)

	input := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(acc.BucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	}

	_, err = client.AbortMultipartUpload(c.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchUpload") {
			WriteS3Error(c, ErrNoSuchUpload)
			return
		}
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// ListParts 列出已上传的分片
func ListParts(c *gin.Context) {
	bucket, key := getBucketAndKey(c)
	uploadID := c.Query("uploadId")
	maxPartsStr := c.DefaultQuery("max-parts", "1000")
	partNumberMarkerStr := c.Query("part-number-marker")

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

	maxParts, _ := strconv.Atoi(maxPartsStr)
	if maxParts <= 0 || maxParts > 1000 {
		maxParts = 1000
	}

	// 调用 R2 列出分片
	client := getS3ClientForAccount(acc)

	input := &s3.ListPartsInput{
		Bucket:   aws.String(acc.BucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MaxParts: aws.Int32(int32(maxParts)),
	}

	if partNumberMarkerStr != "" {
		partNumberMarker, _ := strconv.Atoi(partNumberMarkerStr)
		input.PartNumberMarker = aws.String(strconv.Itoa(partNumberMarker))
	}

	output, err := client.ListParts(c.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchUpload") {
			WriteS3Error(c, ErrNoSuchUpload)
			return
		}
		WriteS3ErrorWithMessage(c, ErrInternalError, err.Error())
		return
	}

	result := ListPartsResult{
		Xmlns:            "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:           bucket,
		Key:              key,
		UploadId:         uploadID,
		PartNumberMarker: int(aws.ToInt32((*int32)(nil))),
		MaxParts:         maxParts,
		IsTruncated:      aws.ToBool(output.IsTruncated),
	}

	if output.NextPartNumberMarker != nil {
		nextMarker, _ := strconv.Atoi(*output.NextPartNumberMarker)
		result.NextPartNumberMarker = nextMarker
	}

	for _, p := range output.Parts {
		etag := aws.ToString(p.ETag)
		etag = strings.Trim(etag, `"`)

		result.Parts = append(result.Parts, PartInfo{
			PartNumber:   int(aws.ToInt32(p.PartNumber)),
			LastModified: aws.ToTime(p.LastModified).Format(time.RFC3339),
			ETag:         etag,
			Size:         aws.ToInt64(p.Size),
		})
	}

	WriteS3XMLResponse(c, http.StatusOK, result)
}
