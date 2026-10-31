package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"fileflow/server/store"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var ErrInvalidFileBrowser = errors.New("文件浏览参数无效")

type flatFileCursor struct {
	Version   int    `json:"v"`
	AccountID string `json:"a"`
	Prefix    string `json:"p"`
	Query     string `json:"q"`
	After     string `json:"k"`
}

// ListFlatAccountFiles scans at most one R2 listing page per request. The cursor
// records the last processed key, including when a search page contains no hits.
func ListFlatAccountFiles(ctx context.Context, accountID, prefix, query, cursor string, limit int32) (AccountFiles, error) {
	result := AccountFiles{Files: []*FileNode{}}
	account, err := store.GetAccountByID(accountID)
	if err != nil {
		return result, err
	}
	if limit < 1 || limit > 100 || len(query) > 256 || len(prefix) > 1024 {
		return result, ErrInvalidFileBrowser
	}
	query = strings.ToLower(strings.TrimSpace(query))
	after := ""
	if cursor != "" {
		encoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || len(encoded) > 4096 {
			return result, ErrInvalidFileBrowser
		}
		var state flatFileCursor
		if json.Unmarshal(encoded, &state) != nil || state.Version != 1 || state.AccountID != accountID || state.Prefix != prefix || state.Query != query {
			return result, ErrInvalidFileBrowser
		}
		after = state.After
	}
	input := &s3.ListObjectsV2Input{Bucket: aws.String(account.BucketName), Prefix: aws.String(prefix), MaxKeys: aws.Int32(1000)}
	if after != "" {
		input.StartAfter = aws.String(after)
	}
	output, err := getS3Client(account).ListObjectsV2(ctx, input)
	if err != nil {
		return result, fmt.Errorf("搜索或列出文件失败: %w", err)
	}
	result.ID, result.AccountName, result.SizeBytes, result.MaxSize = account.ID, account.Name, account.Usage.SizeBytes, account.Quota.MaxSizeBytes
	lastKey := after
	more := aws.ToBool(output.IsTruncated)
	for index, object := range output.Contents {
		key := aws.ToString(object.Key)
		lastKey = key
		if strings.HasSuffix(key, "/") || key == prefix || IsPendingUploadKey(account.ID, key) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(key), query) {
			continue
		}
		result.Files = append(result.Files, &FileNode{Key: key, Name: key, Size: aws.ToInt64(object.Size), LastModified: object.LastModified})
		if len(result.Files) >= int(limit) {
			more = index < len(output.Contents)-1 || more
			break
		}
	}
	if more && lastKey != "" && lastKey != after {
		encoded, _ := json.Marshal(flatFileCursor{Version: 1, AccountID: account.ID, Prefix: prefix, Query: query, After: lastKey})
		result.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	return result, nil
}

type BatchFileDeleteResult struct {
	Key     string `json:"key"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func validateBatchFileKeys(keys []string) ([]string, error) {
	if len(keys) == 0 || len(keys) > 100 {
		return nil, fmt.Errorf("%w: 每批需选择 1 到 100 个文件", ErrInvalidFileBrowser)
	}
	unique := make([]string, 0, len(keys))
	seen := map[string]bool{}
	for _, key := range keys {
		if key == "" || len(key) > 1024 || strings.HasSuffix(key, "/") {
			return nil, fmt.Errorf("%w: 请选择文件，目录不支持该批量操作", ErrInvalidFileBrowser)
		}
		if !seen[key] {
			unique = append(unique, key)
			seen[key] = true
		}
	}
	return unique, nil
}

func BatchDeleteAccountFiles(ctx context.Context, accountID string, keys []string) ([]BatchFileDeleteResult, error) {
	keys, err := validateBatchFileKeys(keys)
	if err != nil {
		return nil, err
	}
	account, err := store.GetAccountByID(accountID)
	if err != nil {
		return nil, err
	}
	if !account.IsActive {
		return nil, fmt.Errorf("存储账号已停用")
	}
	results := make([]BatchFileDeleteResult, 0, len(keys))
	for _, key := range keys {
		entry := BatchFileDeleteResult{Key: key}
		if ctx.Err() != nil {
			entry.Error = "操作已取消"
		} else if IsPendingUploadKey(account.ID, key) {
			entry.Error = "文件尚未完成上传"
		} else {
			expirationOperationLock.Lock()
			deleteErr := DeleteFile(ctx, accountID, key)
			if deleteErr == nil {
				if err := store.DeleteFileExpiration(accountID, key); err != nil {
					deleteErr = fmt.Errorf("文件已删除，但到期记录保存失败: %w", err)
				}
			}
			expirationOperationLock.Unlock()
			if deleteErr != nil {
				entry.Error = deleteErr.Error()
			} else {
				entry.Success = true
			}
		}
		results = append(results, entry)
	}
	return results, nil
}

func SetAccountFilesExpiration(ctx context.Context, accountID string, keys []string, expiresAt *string) (int, error) {
	keys, err := validateBatchFileKeys(keys)
	if err != nil {
		return 0, err
	}
	if expiresAt != nil {
		parsed, err := time.Parse(time.RFC3339, *expiresAt)
		if err != nil || !parsed.After(time.Now()) {
			return 0, fmt.Errorf("%w: 到期时间必须是未来时间", ErrInvalidFileBrowser)
		}
	}
	account, err := store.GetAccountByID(accountID)
	if err != nil {
		return 0, err
	}
	if !account.IsActive {
		return 0, fmt.Errorf("存储账号已停用")
	}
	expirationOperationLock.Lock()
	defer expirationOperationLock.Unlock()
	client := getS3Client(account)
	for _, key := range keys {
		if IsPendingUploadKey(account.ID, key) {
			return 0, fmt.Errorf("文件尚未完成上传")
		}
		if _, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(key)}); err != nil {
			return 0, fmt.Errorf("验证文件 %s 失败: %w", key, err)
		}
	}
	return store.SetFilesExpiration(accountID, keys, expiresAt)
}
