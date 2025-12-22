package webdav

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// FileInfo 文件信息接口（兼容 OpenList model.Obj）
type FileInfo interface {
	GetName() string
	GetSize() int64
	GetPath() string
	ModTime() time.Time
	CreateTime() time.Time
	IsDir() bool
	GetETag() string
	GetContentType() string
}

// Storage 存储操作接口
type Storage interface {
	// List 列出目录内容
	List(ctx context.Context, path string) ([]FileInfo, error)
	// Get 获取文件/目录信息
	Get(ctx context.Context, path string) (FileInfo, error)
	// Open 打开文件获取读取流
	Open(ctx context.Context, path string) (io.ReadCloser, int64, error)
	// Put 上传文件
	Put(ctx context.Context, path string, reader io.Reader, size int64, contentType string) error
	// MakeDir 创建目录
	MakeDir(ctx context.Context, path string) error
	// Remove 删除文件或目录
	Remove(ctx context.Context, path string) error
	// Move 移动文件或目录
	Move(ctx context.Context, src, dst string) error
	// Copy 复制文件或目录
	Copy(ctx context.Context, src, dst string) error
}

// S3FileInfo S3 文件信息实现
type S3FileInfo struct {
	name        string
	size        int64
	path        string
	modTime     time.Time
	isDir       bool
	etag        string
	contentType string
}

func (f *S3FileInfo) GetName() string        { return f.name }
func (f *S3FileInfo) GetSize() int64         { return f.size }
func (f *S3FileInfo) GetPath() string        { return f.path }
func (f *S3FileInfo) ModTime() time.Time     { return f.modTime }
func (f *S3FileInfo) CreateTime() time.Time  { return f.modTime }
func (f *S3FileInfo) IsDir() bool            { return f.isDir }
func (f *S3FileInfo) GetETag() string        { return f.etag }
func (f *S3FileInfo) GetContentType() string { return f.contentType }

// S3Storage S3 存储实现
type S3Storage struct {
	client     *s3.Client
	bucketName string
}

// NewS3Storage 创建 S3 存储适配器
func NewS3Storage(acc *store.Account) (*S3Storage, error) {
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
	})

	return &S3Storage{
		client:     client,
		bucketName: acc.BucketName,
	}, nil
}

// pathToKey 将路径转换为 S3 key
func pathToKey(p string) string {
	p = strings.TrimPrefix(p, "/")
	return p
}

// keyToPath 将 S3 key 转换为路径
func keyToPath(key string) string {
	if !strings.HasPrefix(key, "/") {
		return "/" + key
	}
	return key
}

// List 列出目录内容
func (s *S3Storage) List(ctx context.Context, dirPath string) ([]FileInfo, error) {
	prefix := pathToKey(dirPath)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var files []FileInfo
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucketName),
			Prefix:            aws.String(prefix),
			Delimiter:         aws.String("/"),
			ContinuationToken: continuationToken,
		}

		output, err := s.client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("list objects failed: %w", err)
		}

		// 处理目录（CommonPrefixes）
		for _, cp := range output.CommonPrefixes {
			name := strings.TrimPrefix(*cp.Prefix, prefix)
			name = strings.TrimSuffix(name, "/")
			if name == "" {
				continue
			}
			files = append(files, &S3FileInfo{
				name:    name,
				path:    keyToPath(*cp.Prefix),
				isDir:   true,
				modTime: time.Now(),
			})
		}

		// 处理文件
		for _, obj := range output.Contents {
			key := *obj.Key
			// 跳过目录占位符
			if strings.HasSuffix(key, "/") || key == prefix {
				continue
			}
			name := strings.TrimPrefix(key, prefix)
			if name == "" || strings.Contains(name, "/") {
				continue
			}

			etag := ""
			if obj.ETag != nil {
				etag = strings.Trim(*obj.ETag, "\"")
			}

			modTime := time.Now()
			if obj.LastModified != nil {
				modTime = *obj.LastModified
			}

			files = append(files, &S3FileInfo{
				name:    name,
				size:    *obj.Size,
				path:    keyToPath(key),
				modTime: modTime,
				isDir:   false,
				etag:    etag,
			})
		}

		if !*output.IsTruncated {
			break
		}
		continuationToken = output.NextContinuationToken
	}

	return files, nil
}

// Get 获取文件/目录信息
func (s *S3Storage) Get(ctx context.Context, filePath string) (FileInfo, error) {
	key := pathToKey(filePath)

	// 根目录特殊处理
	if key == "" {
		return &S3FileInfo{
			name:    "",
			path:    "/",
			isDir:   true,
			modTime: time.Now(),
		}, nil
	}

	// 先尝试作为文件获取
	headOutput, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err == nil {
		// 如果 key 以 "/" 结尾，说明是目录占位符
		if strings.HasSuffix(key, "/") {
			return &S3FileInfo{
				name:    path.Base(strings.TrimSuffix(filePath, "/")),
				path:    keyToPath(key),
				isDir:   true,
				modTime: time.Now(),
			}, nil
		}

		etag := ""
		if headOutput.ETag != nil {
			etag = strings.Trim(*headOutput.ETag, "\"")
		}
		contentType := "application/octet-stream"
		if headOutput.ContentType != nil {
			contentType = *headOutput.ContentType
		}
		modTime := time.Now()
		if headOutput.LastModified != nil {
			modTime = *headOutput.LastModified
		}
		size := int64(0)
		if headOutput.ContentLength != nil {
			size = *headOutput.ContentLength
		}

		return &S3FileInfo{
			name:        path.Base(filePath),
			size:        size,
			path:        keyToPath(key),
			modTime:     modTime,
			isDir:       false,
			etag:        etag,
			contentType: contentType,
		}, nil
	}

	// 检查是否为目录
	prefix := key
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	listOutput, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})

	if err != nil {
		return nil, fmt.Errorf("get object failed: %w", err)
	}

	if len(listOutput.Contents) > 0 || len(listOutput.CommonPrefixes) > 0 {
		return &S3FileInfo{
			name:    path.Base(filePath),
			path:    keyToPath(key),
			isDir:   true,
			modTime: time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("not found: %s", filePath)
}

// Open 打开文件获取读取流
func (s *S3Storage) Open(ctx context.Context, filePath string) (io.ReadCloser, int64, error) {
	key := pathToKey(filePath)

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, 0, fmt.Errorf("get object failed: %w", err)
	}

	size := int64(0)
	if output.ContentLength != nil {
		size = *output.ContentLength
	}

	return output.Body, size, nil
}

// Put 上传文件
func (s *S3Storage) Put(ctx context.Context, filePath string, reader io.Reader, size int64, contentType string) error {
	key := pathToKey(filePath)

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	}

	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("put object failed: %w", err)
	}

	return nil
}

// MakeDir 创建目录
func (s *S3Storage) MakeDir(ctx context.Context, dirPath string) error {
	key := pathToKey(dirPath)
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        strings.NewReader(""),
		ContentType: aws.String("application/x-directory"),
	})

	if err != nil {
		return fmt.Errorf("make dir failed: %w", err)
	}

	return nil
}

// Remove 删除文件或目录
func (s *S3Storage) Remove(ctx context.Context, filePath string) error {
	key := pathToKey(filePath)

	// 检查是否为目录
	info, err := s.Get(ctx, filePath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// 删除目录下所有内容
		return s.removeDir(ctx, key)
	}

	// 删除单个文件
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("delete object failed: %w", err)
	}

	return nil
}

// removeDir 递归删除目录
func (s *S3Storage) removeDir(ctx context.Context, prefix string) error {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var continuationToken *string
	var objects []types.ObjectIdentifier

	for {
		listOutput, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucketName),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})

		if err != nil {
			return fmt.Errorf("list objects for delete failed: %w", err)
		}

		for _, obj := range listOutput.Contents {
			objects = append(objects, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		if !*listOutput.IsTruncated {
			break
		}
		continuationToken = listOutput.NextContinuationToken
	}

	if len(objects) == 0 {
		return nil
	}

	// 批量删除（每次最多 1000 个）
	for i := 0; i < len(objects); i += 1000 {
		end := i + 1000
		if end > len(objects) {
			end = len(objects)
		}

		_, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucketName),
			Delete: &types.Delete{
				Objects: objects[i:end],
				Quiet:   aws.Bool(true),
			},
		})

		if err != nil {
			return fmt.Errorf("batch delete failed: %w", err)
		}
	}

	return nil
}

// Move 移动文件或目录
func (s *S3Storage) Move(ctx context.Context, src, dst string) error {
	// 先复制
	if err := s.Copy(ctx, src, dst); err != nil {
		return err
	}
	// 再删除源
	return s.Remove(ctx, src)
}

// Copy 复制文件或目录
func (s *S3Storage) Copy(ctx context.Context, src, dst string) error {
	srcKey := pathToKey(src)
	dstKey := pathToKey(dst)

	// 检查源是否为目录
	info, err := s.Get(ctx, src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return s.copyDir(ctx, srcKey, dstKey)
	}

	// 复制单个文件
	_, err = s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucketName),
		Key:        aws.String(dstKey),
		CopySource: aws.String(s.bucketName + "/" + srcKey),
	})

	if err != nil {
		return fmt.Errorf("copy object failed: %w", err)
	}

	return nil
}

// copyDir 递归复制目录
func (s *S3Storage) copyDir(ctx context.Context, srcPrefix, dstPrefix string) error {
	if !strings.HasSuffix(srcPrefix, "/") {
		srcPrefix += "/"
	}
	if !strings.HasSuffix(dstPrefix, "/") {
		dstPrefix += "/"
	}

	var continuationToken *string

	for {
		listOutput, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucketName),
			Prefix:            aws.String(srcPrefix),
			ContinuationToken: continuationToken,
		})

		if err != nil {
			return fmt.Errorf("list objects for copy failed: %w", err)
		}

		for _, obj := range listOutput.Contents {
			srcKey := *obj.Key
			relPath := strings.TrimPrefix(srcKey, srcPrefix)
			dstKey := dstPrefix + relPath

			_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
				Bucket:     aws.String(s.bucketName),
				Key:        aws.String(dstKey),
				CopySource: aws.String(s.bucketName + "/" + srcKey),
			})

			if err != nil {
				return fmt.Errorf("copy object %s failed: %w", srcKey, err)
			}
		}

		if !*listOutput.IsTruncated {
			break
		}
		continuationToken = listOutput.NextContinuationToken
	}

	return nil
}
