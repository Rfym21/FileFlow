package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"fileflow/server/store"
)

// 到期修改与计划删除共用此锁，避免后台使用旧扫描结果删除已延期的文件。
var expirationOperationLock sync.Mutex

// UpdateFileExpirations 修改单个或多个文件的到期时间。
func UpdateFileExpirations(ids []string, expiresAt string) (int, error) {
	expirationOperationLock.Lock()
	defer expirationOperationLock.Unlock()
	return store.UpdateFileExpirations(ids, expiresAt)
}

// DeleteScheduledFile 删除云端文件并移除计划记录。
func DeleteScheduledFile(ctx context.Context, id string) error {
	expirationOperationLock.Lock()
	defer expirationOperationLock.Unlock()

	exp, err := store.GetFileExpirationByID(id)
	if err != nil {
		return err
	}
	return deleteScheduledFile(ctx, exp)
}

func deleteScheduledFile(ctx context.Context, exp *store.FileExpiration) error {
	if err := DeleteFile(ctx, exp.AccountID, exp.FileKey); err != nil {
		return err
	}
	if err := store.DeleteFileExpirationByID(exp.ID); err != nil {
		return fmt.Errorf("文件已删除，但删除计划保存失败: %w", err)
	}
	return nil
}

// deleteExpiredFile 在删除前重新检查最新时间，扫描后已延期的记录会跳过。
func deleteExpiredFile(ctx context.Context, id string) (bool, error) {
	expirationOperationLock.Lock()
	defer expirationOperationLock.Unlock()

	exp, err := store.GetFileExpirationByID(id)
	if errors.Is(err, store.ErrFileExpirationNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	expiresAt, err := time.Parse(time.RFC3339, exp.ExpiresAt)
	if err != nil {
		return false, fmt.Errorf("到期时间无效: %w", err)
	}
	if expiresAt.After(time.Now()) {
		return false, nil
	}
	if err := deleteScheduledFile(ctx, exp); err != nil {
		return false, err
	}
	return true, nil
}

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
		deleted, err := deleteExpiredFile(ctx, exp.ID)
		if err != nil {
			log.Printf("[Expiration] 删除文件失败 (accountId=%s, key=%s): %v", exp.AccountID, exp.FileKey, err)
			failCount++
			continue
		}

		if !deleted {
			continue
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
