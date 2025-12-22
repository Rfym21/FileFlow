package service

import (
	"context"
	"log"
	"time"

	"fileflow/server/store"
)

// CheckAndDeleteExpiredFiles 检查并删除过期文件
func CheckAndDeleteExpiredFiles(ctx context.Context) {
	log.Println("[Expiration] 开始检查过期文件")

	// 获取所有过期的文件记录
	expiredFiles := store.GetExpiredFiles()
	if len(expiredFiles) == 0 {
		log.Println("[Expiration] 没有过期文件需要删除")
		return
	}

	log.Printf("[Expiration] 发现 %d 个过期文件需要删除", len(expiredFiles))

	successCount := 0
	failCount := 0

	for _, exp := range expiredFiles {
		// 删除 S3 中的文件
		err := DeleteFile(ctx, exp.AccountID, exp.FileKey)
		if err != nil {
			log.Printf("[Expiration] 删除文件失败 (accountId=%s, key=%s): %v", exp.AccountID, exp.FileKey, err)
			failCount++
			continue
		}

		// 删除到期记录
		if err := store.DeleteFileExpirationByID(exp.ID); err != nil {
			log.Printf("[Expiration] 删除到期记录失败 (id=%s): %v", exp.ID, err)
			// 文件已删除，记录删除失败也计入成功
		}

		successCount++
		log.Printf("[Expiration] 已删除过期文件: %s/%s", exp.AccountID, exp.FileKey)
	}

	log.Printf("[Expiration] 过期文件清理完成: 成功 %d, 失败 %d", successCount, failCount)
}

// CleanupExpiredFilesByAccount 清理指定账户的所有过期文件记录
func CleanupExpiredFilesByAccount(ctx context.Context, accountID string) {
	expirations := store.GetFileExpirations()
	for _, exp := range expirations {
		if exp.AccountID == accountID {
			store.DeleteFileExpirationByID(exp.ID)
		}
	}
}

// CreateFileExpirationRecord 创建文件到期记录
func CreateFileExpirationRecord(accountID, fileKey string, expirationDays int) error {
	if expirationDays <= 0 {
		// 永久文件，不创建到期记录
		return nil
	}

	expiresAt := time.Now().AddDate(0, 0, expirationDays).Format(time.RFC3339)
	return store.CreateFileExpiration(&store.FileExpiration{
		AccountID: accountID,
		FileKey:   fileKey,
		ExpiresAt: expiresAt,
	})
}

// DeleteFileExpirationRecord 删除文件到期记录
func DeleteFileExpirationRecord(accountID, fileKey string) error {
	return store.DeleteFileExpiration(accountID, fileKey)
}
