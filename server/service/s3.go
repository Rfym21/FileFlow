package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"fileflow/server/config"
	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// FileInfo 文件信息
type FileInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
	IsDir        bool      `json:"isDir"`
}

// FileNode 文件树节点
type FileNode struct {
	Key          string      `json:"key"`
	Name         string      `json:"name"`
	Size         int64       `json:"size,omitempty"`
	LastModified *time.Time  `json:"lastModified,omitempty"`
	IsDir        bool        `json:"isDir"`
	Children     []*FileNode `json:"children,omitempty"`
}

// TreeNode 构建文件树时的辅助结构
type TreeNode struct {
	node     *FileNode
	children map[string]*TreeNode
}

// AccountFiles 账户文件列表
type AccountFiles struct {
	ID          string      `json:"id"`
	AccountName string      `json:"accountName"`
	Files       []*FileNode `json:"files"`
	SizeBytes   int64       `json:"sizeBytes"`
	MaxSize     int64       `json:"maxSize"`
	NextCursor  string      `json:"nextCursor,omitempty"`
}

// ListFilesResult 文件列表结果
type ListFilesResult struct {
	Files      []*FileNode `json:"files"`
	NextCursor string      `json:"nextCursor,omitempty"`
}

// UploadResult 上传结果
type UploadResult struct {
	ID          string `json:"id"`
	AccountName string `json:"accountName"`
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
}

// getS3Client 获取账户的 S3 客户端
func getS3Client(acc *store.Account) *s3.Client {
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

// SmartUpload 智能上传文件（自动选择可用账户，失败自动重试其他账户）
func SmartUpload(ctx context.Context, key string, body io.Reader, size int64, contentType string) (*UploadResult, error) {
	accounts := store.GetAvailableAccounts()
	if len(accounts) == 0 {
		return nil, fmt.Errorf("没有可用的存储账户")
	}

	// 按使用率排序，优先使用使用率低的账户
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].GetUsagePercent() < accounts[j].GetUsagePercent()
	})

	// 需要将 body 读取到内存，以便重试
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	var lastErr error
	for _, acc := range accounts {
		result, err := doUpload(ctx, &acc, key, bodyBytes, contentType)
		if err == nil {
			return result, nil
		}
		lastErr = err
		log.Printf("上传到账户 %s 失败: %v，尝试下一个账户", acc.Name, err)
	}

	return nil, fmt.Errorf("所有账户上传均失败: %w", lastErr)
}

// UploadToAccount 上传文件到指定账户
func UploadToAccount(ctx context.Context, accountID string, key string, body io.Reader, contentType string) (*UploadResult, error) {
	acc, err := store.GetAccountByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("账户不存在: %w", err)
	}

	if !acc.IsActive {
		return nil, fmt.Errorf("账户已停用")
	}

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	return doUpload(ctx, acc, key, bodyBytes, contentType)
}

// doUpload 上传文件到指定账户（内部函数）
func doUpload(ctx context.Context, acc *store.Account, key string, body []byte, contentType string) (*UploadResult, error) {
	client := getS3Client(acc)

	input := &s3.PutObjectInput{
		Bucket:      aws.String(acc.BucketName),
		Key:         aws.String(key),
		Body:        strings.NewReader(string(body)),
		ContentType: aws.String(contentType),
	}

	_, err := client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("上传失败: %w", err)
	}

	url := buildPublicURL(acc.PublicDomain, key)

	return &UploadResult{
		ID:          acc.ID,
		AccountName: acc.Name,
		Key:         key,
		Size:        int64(len(body)),
		URL:         url,
	}, nil
}

// ListFiles 列出账户指定前缀下的文件（懒加载+分页）
func ListFiles(ctx context.Context, acc *store.Account, prefix string, cursor string, limit int32) (*ListFilesResult, error) {
	client := getS3Client(acc)

	if limit <= 0 {
		limit = 50
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(acc.BucketName),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int32(limit),
	}

	if cursor != "" {
		input.ContinuationToken = aws.String(cursor)
	}

	output, err := client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("列出文件失败: %w", err)
	}

	var files []*FileNode

	// 添加目录（CommonPrefixes）
	for _, cp := range output.CommonPrefixes {
		dirKey := aws.ToString(cp.Prefix)
		name := strings.TrimSuffix(strings.TrimPrefix(dirKey, prefix), "/")
		if name != "" {
			files = append(files, &FileNode{
				Key:   dirKey,
				Name:  name,
				IsDir: true,
			})
		}
	}

	// 添加文件（Contents）
	for _, obj := range output.Contents {
		key := aws.ToString(obj.Key)
		// 跳过目录本身
		if key == prefix || strings.HasSuffix(key, "/") {
			continue
		}
		name := strings.TrimPrefix(key, prefix)
		lastMod := aws.ToTime(obj.LastModified)
		files = append(files, &FileNode{
			Key:          key,
			Name:         name,
			Size:         aws.ToInt64(obj.Size),
			LastModified: &lastMod,
			IsDir:        false,
		})
	}

	// 排序：目录优先，然后按名称
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	result := &ListFilesResult{
		Files: files,
	}

	if aws.ToBool(output.IsTruncated) {
		result.NextCursor = aws.ToString(output.NextContinuationToken)
	}

	return result, nil
}

// ListAllAccountsFiles 列出所有激活账户的文件（懒加载+分页）
func ListAllAccountsFiles(ctx context.Context, prefix string, cursor string, limit int32) ([]AccountFiles, error) {
	accounts := store.GetActiveAccounts()
	var result []AccountFiles

	for _, acc := range accounts {
		listResult, err := ListFiles(ctx, &acc, prefix, cursor, limit)
		if err != nil {
			log.Printf("列出账户 %s 文件失败: %v", acc.Name, err)
			continue
		}

		result = append(result, AccountFiles{
			ID:          acc.ID,
			AccountName: acc.Name,
			Files:       listResult.Files,
			SizeBytes:   acc.Usage.SizeBytes,
			MaxSize:     acc.Quota.MaxSizeBytes,
			NextCursor:  listResult.NextCursor,
		})
	}

	return result, nil
}

// ListAccountsFilesByIDs 列出指定账户组的文件（懒加载+分页）
func ListAccountsFilesByIDs(ctx context.Context, ids []string, prefix string, cursor string, limit int32) ([]AccountFiles, error) {
	var result []AccountFiles

	for _, id := range ids {
		acc, err := store.GetAccountByID(id)
		if err != nil {
			log.Printf("账户 %s 不存在: %v", id, err)
			continue
		}

		if !acc.IsActive {
			continue
		}

		listResult, err := ListFiles(ctx, acc, prefix, cursor, limit)
		if err != nil {
			log.Printf("列出账户 %s 文件失败: %v", acc.Name, err)
			continue
		}

		result = append(result, AccountFiles{
			ID:          acc.ID,
			AccountName: acc.Name,
			Files:       listResult.Files,
			SizeBytes:   acc.Usage.SizeBytes,
			MaxSize:     acc.Quota.MaxSizeBytes,
			NextCursor:  listResult.NextCursor,
		})
	}

	return result, nil
}

// DeleteFile 删除指定账户的文件
func DeleteFile(ctx context.Context, accountID, key string) error {
	acc, err := store.GetAccountByID(accountID)
	if err != nil {
		return err
	}

	client := getS3Client(acc)

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(acc.BucketName),
		Key:    aws.String(key),
	}

	_, err = client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}

	return nil
}

// GetFileLink 获取文件直链
func GetFileLink(accountID, key string) (string, error) {
	acc, err := store.GetAccountByID(accountID)
	if err != nil {
		return "", err
	}

	return buildPublicURL(acc.PublicDomain, key), nil
}

// buildPublicURL 构建公开访问 URL，处理 publicDomain 可能包含协议前缀的情况
func buildPublicURL(publicDomain, key string) string {
	// 去除 publicDomain 中的协议前缀（包括畸形格式）
	domain := strings.TrimPrefix(publicDomain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https//") // 处理缺少冒号的情况
	domain = strings.TrimPrefix(domain, "http//")

	// 去除可能的尾部斜杠
	domain = strings.TrimSuffix(domain, "/")

	// 去除 key 可能的开头斜杠
	key = strings.TrimPrefix(key, "/")

	// 检查是否启用代理
	cfg := config.Get()
	if cfg.EndpointProxy && cfg.EndpointProxyURL != "" {
		// 提取子域名（如 pub-xxx.r2.dev -> pub-xxx）
		subdomain := domain
		if idx := strings.Index(domain, "."); idx > 0 {
			subdomain = domain[:idx]
		}

		proxyURL := strings.TrimSuffix(cfg.EndpointProxyURL, "/")
		return fmt.Sprintf("%s/%s/%s", proxyURL, subdomain, key)
	}

	return fmt.Sprintf("https://%s/%s", domain, key)
}

// ClearBucket 清空指定账户的存储桶
func ClearBucket(ctx context.Context, accountID string) error {
	acc, err := store.GetAccountByID(accountID)
	if err != nil {
		return fmt.Errorf("账户不存在: %w", err)
	}

	client := getS3Client(acc)

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(acc.BucketName),
	}

	paginator := s3.NewListObjectsV2Paginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("列出文件失败: %w", err)
		}

		if len(page.Contents) == 0 {
			continue
		}

		// 构建删除列表（每次最多 1000 个）
		var objects []types.ObjectIdentifier
		for _, obj := range page.Contents {
			objects = append(objects, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		// 批量删除
		deleteInput := &s3.DeleteObjectsInput{
			Bucket: aws.String(acc.BucketName),
			Delete: &types.Delete{
				Objects: objects,
				Quiet:   aws.Bool(true),
			},
		}

		_, err = client.DeleteObjects(ctx, deleteInput)
		if err != nil {
			return fmt.Errorf("删除文件失败: %w", err)
		}

		log.Printf("已删除 %d 个文件", len(objects))
	}

	return nil
}

// GetAccountStorageSize 获取账户存储使用量
func GetAccountStorageSize(ctx context.Context, acc *store.Account) (int64, error) {
	client := getS3Client(acc)

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(acc.BucketName),
	}

	var totalSize int64
	paginator := s3.NewListObjectsV2Paginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, fmt.Errorf("获取存储使用量失败: %w", err)
		}

		for _, obj := range page.Contents {
			totalSize += aws.ToInt64(obj.Size)
		}
	}

	return totalSize, nil
}

// buildFileTree 构建文件树
func buildFileTree(files []FileInfo, prefix string) []*FileNode {
	root := make(map[string]*TreeNode)

	for _, f := range files {
		// 移除前缀
		relPath := strings.TrimPrefix(f.Key, prefix)
		if relPath == "" {
			continue
		}

		parts := strings.Split(relPath, "/")
		current := root

		for i, part := range parts {
			if part == "" {
				continue
			}

			isLast := i == len(parts)-1
			fullKey := prefix + strings.Join(parts[:i+1], "/")

			if isLast && !f.IsDir {
				// 文件节点
				current[part] = &TreeNode{
					node: &FileNode{
						Key:          fullKey,
						Name:         part,
						Size:         f.Size,
						LastModified: &f.LastModified,
						IsDir:        false,
					},
					children: nil,
				}
			} else {
				// 目录节点
				if _, exists := current[part]; !exists {
					current[part] = &TreeNode{
						node: &FileNode{
							Key:   fullKey + "/",
							Name:  part,
							IsDir: true,
						},
						children: make(map[string]*TreeNode),
					}
				}
				current = current[part].children
			}
		}
	}

	return treeNodesToSlice(root)
}

// treeNodesToSlice 将 TreeNode map 转换为 FileNode 切片
func treeNodesToSlice(treeMap map[string]*TreeNode) []*FileNode {
	result := make([]*FileNode, 0, len(treeMap))

	for _, treeNode := range treeMap {
		node := treeNode.node

		// 递归处理子节点
		if treeNode.children != nil && len(treeNode.children) > 0 {
			node.Children = treeNodesToSlice(treeNode.children)
		}

		result = append(result, node)
	}

	// 按名称排序，目录优先
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})

	return result
}
