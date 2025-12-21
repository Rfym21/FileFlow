package store

import (
	"crypto/rand"
	"fmt"

	"github.com/google/uuid"
)

/**
 * 获取所有 WebDAV 凭证
 */
func GetWebDAVCredentials() []WebDAVCredential {
	dataLock.RLock()
	defer dataLock.RUnlock()

	if data == nil || data.WebDAVCredentials == nil {
		return []WebDAVCredential{}
	}

	result := make([]WebDAVCredential, len(data.WebDAVCredentials))
	copy(result, data.WebDAVCredentials)
	return result
}

/**
 * 根据 ID 获取 WebDAV 凭证
 */
func GetWebDAVCredentialByID(id string) (*WebDAVCredential, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	for _, c := range data.WebDAVCredentials {
		if c.ID == id {
			result := c
			return &result, nil
		}
	}
	return nil, fmt.Errorf("WebDAV 凭证不存在")
}

/**
 * 根据用户名获取 WebDAV 凭证
 */
func GetWebDAVCredentialByUsername(username string) (*WebDAVCredential, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	for _, c := range data.WebDAVCredentials {
		if c.Username == username {
			result := c
			return &result, nil
		}
	}
	return nil, fmt.Errorf("WebDAV 凭证不存在")
}

/**
 * 创建 WebDAV 凭证
 */
func CreateWebDAVCredential(cred *WebDAVCredential) error {
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

	// 如果未提供用户名，则自动生成
	if cred.Username == "" {
		cred.Username = generateWebDAVUsername()
	} else {
		// 检查用户名是否已存在
		for _, c := range data.WebDAVCredentials {
			if c.Username == cred.Username {
				return fmt.Errorf("用户名已存在")
			}
		}
	}

	// 如果未提供密码，则自动生成
	if cred.Password == "" {
		cred.Password = generateWebDAVPassword()
	}

	// 生成凭证信息
	cred.ID = uuid.New().String()
	cred.IsActive = true
	cred.CreatedAt = NowString()
	cred.LastUsedAt = ""

	data.WebDAVCredentials = append(data.WebDAVCredentials, *cred)

	return save()
}

/**
 * 更新 WebDAV 凭证
 */
func UpdateWebDAVCredential(id string, updates *WebDAVCredential) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, c := range data.WebDAVCredentials {
		if c.ID == id {
			// 只更新允许更新的字段
			if updates.Description != "" {
				data.WebDAVCredentials[i].Description = updates.Description
			}
			if updates.Permissions != nil {
				data.WebDAVCredentials[i].Permissions = updates.Permissions
			}
			data.WebDAVCredentials[i].IsActive = updates.IsActive
			return save()
		}
	}
	return fmt.Errorf("WebDAV 凭证不存在")
}

/**
 * 更新 WebDAV 凭证最后使用时间
 */
func UpdateWebDAVCredentialLastUsed(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, c := range data.WebDAVCredentials {
		if c.ID == id {
			data.WebDAVCredentials[i].LastUsedAt = NowString()
			return save()
		}
	}
	return nil
}

/**
 * 删除 WebDAV 凭证
 */
func DeleteWebDAVCredential(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, c := range data.WebDAVCredentials {
		if c.ID == id {
			data.WebDAVCredentials = append(data.WebDAVCredentials[:i], data.WebDAVCredentials[i+1:]...)
			return save()
		}
	}
	return fmt.Errorf("WebDAV 凭证不存在")
}

/**
 * 生成 WebDAV 用户名
 * 格式：FFLW_WebDAV_XXXXXXXX（8 位随机字符）
 */
func generateWebDAVUsername() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return "FFLW_WebDAV_" + string(b)
}

/**
 * 生成 WebDAV 密码（32 字符）
 */
func generateWebDAVPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}
