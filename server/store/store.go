package store

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"sync"

	"fileflow/server/config"

	"github.com/google/uuid"
)

var (
	data     *Data
	dataLock sync.RWMutex
	backend  Backend
)

// Init 初始化存储
func Init() error {
	cfg := config.Get()

	// 确保数据目录存在
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 创建后端
	var err error
	backend, err = NewBackend()
	if err != nil {
		return fmt.Errorf("创建数据库后端失败: %w", err)
	}

	// 初始化后端
	if err := backend.Init(); err != nil {
		return fmt.Errorf("初始化数据库后端失败: %w", err)
	}

	// 获取后端类型用于日志
	backendType, _ := ParseDatabaseURL(cfg.DatabaseURL)
	log.Printf("使用数据库后端: %s", backendType)

	// 加载数据
	return load()
}

// Close 关闭存储
func Close() error {
	if backend != nil {
		return backend.Close()
	}
	return nil
}

// load 从后端加载数据到内存
func load() error {
	dataLock.Lock()
	defer dataLock.Unlock()

	var err error
	data, err = backend.Load()
	if err != nil {
		return fmt.Errorf("加载数据失败: %w", err)
	}

	return nil
}

// save 保存数据到后端（内部使用，需要在锁内调用）
func save() error {
	if err := backend.Save(data); err != nil {
		return fmt.Errorf("保存数据失败: %w", err)
	}
	return nil
}

// GetAccounts 获取所有账户
func GetAccounts() []Account {
	dataLock.RLock()
	defer dataLock.RUnlock()

	result := make([]Account, len(data.Accounts))
	copy(result, data.Accounts)
	return result
}

// AccountsPage 账户分页结果
type AccountsPage struct {
	Items      []Account `json:"items"`
	Total      int       `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
	TotalPages int       `json:"totalPages"`
}

// GetAccountsPaged 分页获取账户
func GetAccountsPaged(page, pageSize int) AccountsPage {
	dataLock.RLock()
	defer dataLock.RUnlock()

	total := len(data.Accounts)
	if pageSize <= 0 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		return AccountsPage{
			Items:      []Account{},
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		}
	}

	if end > total {
		end = total
	}

	result := make([]Account, end-start)
	copy(result, data.Accounts[start:end])

	return AccountsPage{
		Items:      result,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// GetAccountsStats 获取账户统计信息（不含详细数据）
type AccountsStats struct {
	TotalAccounts   int   `json:"totalAccounts"`
	AvailableCount  int   `json:"availableCount"`
	TotalSizeBytes  int64 `json:"totalSizeBytes"`
	TotalQuotaBytes int64 `json:"totalQuotaBytes"`
	TotalWriteOps   int64 `json:"totalWriteOps"`
	TotalReadOps    int64 `json:"totalReadOps"`
}

// GetAccountsStats 获取账户统计信息
func GetAccountsStats() AccountsStats {
	dataLock.RLock()
	defer dataLock.RUnlock()

	stats := AccountsStats{
		TotalAccounts: len(data.Accounts),
	}

	for _, acc := range data.Accounts {
		if acc.IsAvailable() {
			stats.AvailableCount++
		}
		stats.TotalSizeBytes += acc.Usage.SizeBytes
		stats.TotalQuotaBytes += acc.Quota.MaxSizeBytes
		stats.TotalWriteOps += acc.Usage.ClassAOps
		stats.TotalReadOps += acc.Usage.ClassBOps
	}

	return stats
}

// GetActiveAccounts 获取所有激活的账户
func GetActiveAccounts() []Account {
	dataLock.RLock()
	defer dataLock.RUnlock()

	var result []Account
	for _, acc := range data.Accounts {
		if acc.IsActive {
			result = append(result, acc)
		}
	}
	return result
}

// GetAvailableAccounts 获取所有可用于上传的账户
func GetAvailableAccounts() []Account {
	dataLock.RLock()
	defer dataLock.RUnlock()

	var result []Account
	for _, acc := range data.Accounts {
		if acc.IsAvailable() {
			result = append(result, acc)
		}
	}
	return result
}

// GetAvailableAccountsForAutoUpload 获取所有可用于自动上传的账户
func GetAvailableAccountsForAutoUpload() []Account {
	dataLock.RLock()
	defer dataLock.RUnlock()

	var result []Account
	for _, acc := range data.Accounts {
		if acc.IsAvailableForAutoUpload() {
			result = append(result, acc)
		}
	}
	return result
}

// GetAvailableAccountsForClientUpload 获取所有可用于前端上传的账户
func GetAvailableAccountsForClientUpload() []Account {
	dataLock.RLock()
	defer dataLock.RUnlock()

	var result []Account
	for _, acc := range data.Accounts {
		if acc.IsAvailableForClientUpload() {
			result = append(result, acc)
		}
	}
	return result
}

// GetAccountByID 根据 ID 获取账户
func GetAccountByID(id string) (*Account, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	for _, acc := range data.Accounts {
		if acc.ID == id {
			result := acc
			return &result, nil
		}
	}
	return nil, fmt.Errorf("账户不存在: %s", id)
}

// CreateAccount 创建账户
func CreateAccount(acc *Account) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	acc.ID = uuid.New().String()
	acc.CreatedAt = NowString()
	acc.UpdatedAt = NowString()

	data.Accounts = append(data.Accounts, *acc)
	return save()
}

// UpdateAccount 更新账户
func UpdateAccount(acc *Account) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, a := range data.Accounts {
		if a.ID == acc.ID {
			acc.UpdatedAt = NowString()
			acc.CreatedAt = a.CreatedAt // 保留创建时间
			data.Accounts[i] = *acc
			return save()
		}
	}
	return fmt.Errorf("账户不存在: %s", acc.ID)
}

// UpdateAccountUsage 更新账户使用量
func UpdateAccountUsage(id string, usage Usage) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, a := range data.Accounts {
		if a.ID == id {
			data.Accounts[i].Usage = usage
			data.Accounts[i].Usage.LastSyncAt = NowString()
			data.Accounts[i].UpdatedAt = NowString()
			return save()
		}
	}
	return fmt.Errorf("账户不存在: %s", id)
}

// DeleteAccount 删除账户
func DeleteAccount(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, acc := range data.Accounts {
		if acc.ID == id {
			data.Accounts = append(data.Accounts[:i], data.Accounts[i+1:]...)
			return save()
		}
	}
	return fmt.Errorf("账户不存在: %s", id)
}

// GetTokens 获取所有 Token
func GetTokens() []Token {
	dataLock.RLock()
	defer dataLock.RUnlock()

	result := make([]Token, len(data.Tokens))
	copy(result, data.Tokens)
	return result
}

// GetTokenByValue 根据 Token 值获取 Token
func GetTokenByValue(tokenValue string) (*Token, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	for _, t := range data.Tokens {
		if t.Token == tokenValue {
			result := t
			return &result, nil
		}
	}
	return nil, fmt.Errorf("Token 不存在")
}

// CreateToken 创建 Token
func CreateToken(t *Token) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	t.ID = uuid.New().String()
	t.Token = "sk-" + generateRandomString(61)
	t.CreatedAt = NowString()

	data.Tokens = append(data.Tokens, *t)
	return save()
}

// generateRandomString 生成指定长度的随机字符串（大小写字母和数字）
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// DeleteToken 删除 Token
func DeleteToken(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, t := range data.Tokens {
		if t.ID == id {
			data.Tokens = append(data.Tokens[:i], data.Tokens[i+1:]...)
			return save()
		}
	}
	return fmt.Errorf("Token 不存在: %s", id)
}

// ValidateAPIToken 验证 API Token 并返回 Token 对象
func ValidateAPIToken(tokenValue string) (*Token, error) {
	return GetTokenByValue(tokenValue)
}

// GetSettings 获取系统设置
func GetSettings() Settings {
	dataLock.RLock()
	defer dataLock.RUnlock()

	// 返回默认值如果未设置
	settings := data.Settings
	if settings.SyncInterval <= 0 {
		settings.SyncInterval = 5
	}
	return settings
}

// UpdateSettings 更新系统设置
func UpdateSettings(settings Settings) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	// 验证同步间隔
	if settings.SyncInterval < 1 {
		settings.SyncInterval = 1
	}

	// 验证到期检查间隔
	if settings.ExpirationCheckMinutes < 60 {
		settings.ExpirationCheckMinutes = 60
	} else if settings.ExpirationCheckMinutes > 1440 {
		settings.ExpirationCheckMinutes = 1440
	}

	data.Settings = settings
	return save()
}

// GetFileExpirations 获取所有文件到期记录
func GetFileExpirations() []FileExpiration {
	dataLock.RLock()
	defer dataLock.RUnlock()

	if data.FileExpirations == nil {
		return []FileExpiration{}
	}

	result := make([]FileExpiration, len(data.FileExpirations))
	copy(result, data.FileExpirations)
	return result
}

// GetExpiredFiles 获取已过期的文件列表
func GetExpiredFiles() []FileExpiration {
	dataLock.RLock()
	defer dataLock.RUnlock()

	now := NowString()
	var result []FileExpiration
	for _, exp := range data.FileExpirations {
		if exp.ExpiresAt <= now {
			result = append(result, exp)
		}
	}
	return result
}

// CreateFileExpiration 创建文件到期记录
func CreateFileExpiration(exp *FileExpiration) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	exp.ID = uuid.New().String()
	exp.CreatedAt = NowString()

	// 检查是否已存在相同的记录（相同账户和文件）
	for i, e := range data.FileExpirations {
		if e.AccountID == exp.AccountID && e.FileKey == exp.FileKey {
			// 更新现有记录
			data.FileExpirations[i] = *exp
			return save()
		}
	}

	data.FileExpirations = append(data.FileExpirations, *exp)
	return save()
}

// DeleteFileExpiration 删除指定账户和文件的到期记录
func DeleteFileExpiration(accountID, fileKey string) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, exp := range data.FileExpirations {
		if exp.AccountID == accountID && exp.FileKey == fileKey {
			data.FileExpirations = append(data.FileExpirations[:i], data.FileExpirations[i+1:]...)
			return save()
		}
	}
	return nil // 不存在也不报错
}

// DeleteFileExpirationByID 按 ID 删除到期记录
func DeleteFileExpirationByID(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	for i, exp := range data.FileExpirations {
		if exp.ID == id {
			data.FileExpirations = append(data.FileExpirations[:i], data.FileExpirations[i+1:]...)
			return save()
		}
	}
	return nil
}

// DeleteFileExpirationsByAccountID 删除指定账户的所有到期记录
func DeleteFileExpirationsByAccountID(accountID string) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	var remaining []FileExpiration
	for _, exp := range data.FileExpirations {
		if exp.AccountID != accountID {
			remaining = append(remaining, exp)
		}
	}
	data.FileExpirations = remaining
	return save()
}
