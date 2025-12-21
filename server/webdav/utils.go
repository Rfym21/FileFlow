package webdav

import (
	"context"
	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

/**
 * 创建 S3 客户端
 * 使用账户配置信息
 */
func createS3Client(acc *store.Account) (*s3.Client, error) {
	cfg := aws.Config{
		Region: "auto",
		Credentials: credentials.NewStaticCredentialsProvider(
			acc.AccessKeyId,
			acc.SecretAccessKey,
			"",
		),
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(acc.Endpoint)
		o.UsePathStyle = true
	})

	return client, nil
}

/**
 * 解析 WebDAV 路径为 S3 Key
 */
func pathToKey(urlPath string) string {
	key := urlPath
	if key != "" && key[0] == '/' {
		key = key[1:]
	}
	return key
}

/**
 * 检查对象是否存在
 */
func objectExists(ctx context.Context, client *s3.Client, bucket, key string) bool {
	_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err == nil
}
