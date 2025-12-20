package service

import (
	"context"
	"log"
	"sort"

	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// GCThreshold GC 阈值（99.5%）
const GCThreshold = 99.5

// RunGC 执行垃圾回收
func RunGC(ctx context.Context, acc *store.Account) error {
	usagePercent := acc.GetUsagePercent()
	if usagePercent <= 100 {
		return nil // 未超限，无需 GC
	}

	log.Printf("[GC] 账户 %s 容量使用率 %.2f%%，开始执行 GC", acc.Name, usagePercent)

	client := getS3Client(acc)

	// 获取所有文件并按时间排序
	files, err := listAllFilesForGC(ctx, client, acc.BucketName)
	if err != nil {
		return err
	}

	// 按 LastModified 升序排列（最旧的在前）
	sort.Slice(files, func(i, j int) bool {
		return files[i].LastModified.Before(files[j].LastModified)
	})

	// 计算需要删除多少容量才能降到 99.5%
	targetSize := int64(float64(acc.Quota.MaxSizeBytes) * GCThreshold / 100)
	currentSize := acc.Usage.SizeBytes
	needToDelete := currentSize - targetSize

	if needToDelete <= 0 {
		return nil
	}

	var deletedSize int64
	var deletedFiles []string

	for _, f := range files {
		if deletedSize >= needToDelete {
			break
		}

		// 删除文件
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(acc.BucketName),
			Key:    aws.String(f.Key),
		})
		if err != nil {
			log.Printf("[GC] 删除文件 %s 失败: %v", f.Key, err)
			continue
		}

		deletedSize += f.Size
		deletedFiles = append(deletedFiles, f.Key)
		log.Printf("[GC] 已删除: %s (%.2f KB)", f.Key, float64(f.Size)/1024)
	}

	log.Printf("[GC] 账户 %s GC 完成，共删除 %d 个文件，释放 %.2f MB",
		acc.Name, len(deletedFiles), float64(deletedSize)/1024/1024)

	return nil
}

// RunGCForAllAccounts 对所有超限账户执行 GC
func RunGCForAllAccounts(ctx context.Context) {
	accounts := store.GetAccounts()

	for _, acc := range accounts {
		if acc.GetUsagePercent() > 100 {
			if err := RunGC(ctx, &acc); err != nil {
				log.Printf("[GC] 账户 %s GC 失败: %v", acc.Name, err)
			}
		}
	}
}

// listAllFilesForGC 获取所有文件用于 GC
func listAllFilesForGC(ctx context.Context, client *s3.Client, bucket string) ([]FileInfo, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}

	var files []FileInfo
	paginator := s3.NewListObjectsV2Paginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, obj := range page.Contents {
			files = append(files, FileInfo{
				Key:          aws.ToString(obj.Key),
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
			})
		}
	}

	return files, nil
}
