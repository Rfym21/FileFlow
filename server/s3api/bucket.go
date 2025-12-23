package s3api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

// HeadBucket 检查 Bucket 是否存在
func HeadBucket(c *gin.Context) {
	bucketName := c.Param("bucket")

	// 验证 Bucket 权限
	cred := GetS3CredentialFromContext(c)
	acc := GetS3AccountFromContext(c)

	if acc == nil || acc.BucketName != bucketName {
		// 尝试通过 bucket 名称查找
		account, err := store.GetAccountByBucketName(bucketName)
		if err != nil {
			WriteS3Error(c, ErrNoSuchBucket)
			return
		}
		// 检查凭证是否有权访问该账户
		if cred.AccountID != account.ID {
			WriteS3Error(c, ErrAccessDenied)
			return
		}
	}

	c.Status(http.StatusOK)
}

// ListObjectsV2 列出对象
func ListObjectsV2(c *gin.Context) {
	bucketName := c.Param("bucket")
	prefix := c.Query("prefix")
	delimiter := c.Query("delimiter")
	maxKeysStr := c.DefaultQuery("max-keys", "1000")
	continuationToken := c.Query("continuation-token")
	startAfter := c.Query("start-after")

	maxKeys, err := strconv.Atoi(maxKeysStr)
	if err != nil || maxKeys < 1 {
		maxKeys = 1000
	}
	if maxKeys > 1000 {
		maxKeys = 1000
	}

	// 验证权限
	cred := GetS3CredentialFromContext(c)
	if !cred.HasPermission("read") {
		WriteS3Error(c, ErrAccessDenied)
		return
	}

	// 获取账户
	acc := GetS3AccountFromContext(c)
	if acc == nil || acc.BucketName != bucketName {
		account, err := store.GetAccountByBucketName(bucketName)
		if err != nil {
			WriteS3Error(c, ErrNoSuchBucket)
			return
		}
		if cred.AccountID != account.ID {
			WriteS3Error(c, ErrAccessDenied)
			return
		}
		acc = account
	}

	// 调用 R2 列出对象
	client := getS3ClientForAccount(acc)

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(acc.BucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(maxKeys)),
	}

	if delimiter != "" {
		input.Delimiter = aws.String(delimiter)
	}
	if continuationToken != "" {
		input.ContinuationToken = aws.String(continuationToken)
	}
	if startAfter != "" {
		input.StartAfter = aws.String(startAfter)
	}

	output, err := client.ListObjectsV2(c.Request.Context(), input)
	if err != nil {
		WriteS3Error(c, ErrInternalError)
		return
	}

	// 构建响应
	result := ListBucketResult{
		Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:        bucketName,
		Prefix:      prefix,
		Delimiter:   delimiter,
		MaxKeys:     maxKeys,
		IsTruncated: aws.ToBool(output.IsTruncated),
		KeyCount:    int(aws.ToInt32(output.KeyCount)),
	}

	if continuationToken != "" {
		result.ContinuationToken = continuationToken
	}
	if output.NextContinuationToken != nil {
		result.NextContinuationToken = aws.ToString(output.NextContinuationToken)
	}
	if startAfter != "" {
		result.StartAfter = startAfter
	}

	// 转换对象列表
	for _, obj := range output.Contents {
		etag := aws.ToString(obj.ETag)
		// 移除 ETag 两端的引号（R2 返回的 ETag 包含引号，但 XML 中不应该有）
		etag = strings.Trim(etag, `"`)

		result.Contents = append(result.Contents, ObjectInfo{
			Key:          aws.ToString(obj.Key),
			LastModified: aws.ToTime(obj.LastModified).Format(time.RFC3339),
			ETag:         etag,
			Size:         aws.ToInt64(obj.Size),
			StorageClass: "STANDARD",
		})
	}

	// 转换公共前缀
	for _, cp := range output.CommonPrefixes {
		result.CommonPrefixes = append(result.CommonPrefixes, CommonPrefix{
			Prefix: aws.ToString(cp.Prefix),
		})
	}

	WriteS3XMLResponse(c, http.StatusOK, result)
}

// getS3ClientForAccount 获取账户的 S3 客户端
func getS3ClientForAccount(acc *store.Account) *s3.Client {
	cfg := aws.Config{
		Region: "auto",
		Credentials: credentials.NewStaticCredentialsProvider(
			acc.AccessKeyId,
			acc.SecretAccessKey,
			"",
		),
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(acc.Endpoint)
	})
}

// getBucketAndKey 解析请求中的 bucket 和 key
func getBucketAndKey(c *gin.Context) (bucket string, key string) {
	bucket = c.Param("bucket")
	key = c.Param("key")
	// 去除 key 开头的斜杠
	key = strings.TrimPrefix(key, "/")
	return
}
