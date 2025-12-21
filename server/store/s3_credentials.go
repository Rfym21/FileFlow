package store

import (
	"crypto/rand"
	"fmt"

	"github.com/google/uuid"
)

// GetS3Credentials 获取所有 S3 凭证
func GetS3Credentials() []S3Credential {
	dataLock.RLock()
	defer dataLock.RUnlock()

	if data == nil || data.S3Credentials == nil {
		return []S3Credential{}
	}

	result := make([]S3Credential, len(data.S3Credentials))
	copy(result, data.S3Credentials)
	return result
}

// GetS3CredentialByID 根据 ID 获取 S3 凭证
func GetS3CredentialByID(id string) (*S3Credential, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	for _, c := range data.S3Credentials {
		if c.ID == id {
			result := c
			return &result, nil
		}
	}
	return nil, fmt.Errorf("S3 凭证不存在")
}

// GetS3CredentialByAccessKey 根据 Access Key ID 获取 S3 凭证
func GetS3CredentialByAccessKey(accessKeyID string) (*S3Credential, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	for _, c := range data.S3Credentials {
		if c.AccessKeyID == accessKeyID {
			result := c
			return &result, nil
		}
	}
	return nil, fmt.Errorf("S3 凭证不存在")
}

// CreateS3Credential 创建 S3 凭证
func CreateS3Credential(cred *S3Credential) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	// 验证账户存在
	found := false
	for _, acc := range data.Accounts {
		if acc.ID == cred.AccountID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("关联的账户不存在")
	}

	// 生成凭证信息
	cred.ID = uuid.New().String()
	cred.AccessKeyID = generateS3AccessKey()
	cred.SecretAccessKey = generateS3SecretKey()
	cred.IsActive = true
	cred.CreatedAt = NowString()
	cred.LastUsedAt = ""

	data.S3Credentials = append(data.S3Credentials, *cred)

	return save()
}

// UpdateS3Credential 更新 S3 凭证
func UpdateS3Credential(id string, updates *S3Credential) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, c := range data.S3Credentials {
		if c.ID == id {
			// 只更新允许更新的字段
			if updates.Description != "" {
				data.S3Credentials[i].Description = updates.Description
			}
			if updates.Permissions != nil {
				data.S3Credentials[i].Permissions = updates.Permissions
			}
			data.S3Credentials[i].IsActive = updates.IsActive
			return save()
		}
	}
	return fmt.Errorf("S3 凭证不存在")
}

// UpdateS3CredentialLastUsed 更新 S3 凭证最后使用时间
func UpdateS3CredentialLastUsed(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, c := range data.S3Credentials {
		if c.ID == id {
			data.S3Credentials[i].LastUsedAt = NowString()
			return save()
		}
	}
	return nil
}

// DeleteS3Credential 删除 S3 凭证
func DeleteS3Credential(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, c := range data.S3Credentials {
		if c.ID == id {
			data.S3Credentials = append(data.S3Credentials[:i], data.S3Credentials[i+1:]...)
			return save()
		}
	}
	return fmt.Errorf("S3 凭证不存在")
}

// generateS3AccessKey 生成 S3 Access Key ID（20 字符）
// 格式：FFLW + 16 位随机字符
func generateS3AccessKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return "FFLW" + string(b)
}

// generateS3SecretKey 生成 S3 Secret Access Key（40 字符）
func generateS3SecretKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 40)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// GetAccountByBucketName 根据 Bucket 名称获取账户
func GetAccountByBucketName(bucketName string) (*Account, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	for _, acc := range data.Accounts {
		if acc.BucketName == bucketName && acc.IsActive {
			result := acc
			return &result, nil
		}
	}
	return nil, fmt.Errorf("bucket not found: %s", bucketName)
}
