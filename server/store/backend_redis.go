package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	redisAccountsKey          = "fileflow:accounts"
	redisTokensKey            = "fileflow:tokens"
	redisSettingsKey          = "fileflow:settings"
	redisS3CredentialsKey     = "fileflow:s3_credentials"
	redisWebDAVCredentialsKey = "fileflow:webdav_credentials"
	redisFileExpirationsKey   = "fileflow:file_expirations"
)

// RedisBackend Redis 数据库后端
type RedisBackend struct {
	client  *redis.Client
	connStr string
	ctx     context.Context
}

// NewRedisBackend 创建 Redis 后端
func NewRedisBackend(connStr string) (*RedisBackend, error) {
	return &RedisBackend{
		connStr: connStr,
		ctx:     context.Background(),
	}, nil
}

// Init 初始化 Redis 连接
func (b *RedisBackend) Init() error {
	opt, err := redis.ParseURL(b.connStr)
	if err != nil {
		return fmt.Errorf("解析 Redis URL 失败: %w", err)
	}

	b.client = redis.NewClient(opt)

	// 测试连接
	if err := b.client.Ping(b.ctx).Err(); err != nil {
		return fmt.Errorf("Redis 连接测试失败: %w", err)
	}

	return nil
}

// Load 从 Redis 加载全部数据
func (b *RedisBackend) Load() (*Data, error) {
	data := &Data{
		Accounts:          []Account{},
		Tokens:            []Token{},
		S3Credentials:     []S3Credential{},
		WebDAVCredentials: []WebDAVCredential{},
		FileExpirations:   []FileExpiration{},
	}

	// 加载 accounts
	accountsMap, err := b.client.HGetAll(b.ctx, redisAccountsKey).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("加载 accounts 失败: %w", err)
	}

	for _, jsonStr := range accountsMap {
		var acc Account
		if err := json.Unmarshal([]byte(jsonStr), &acc); err != nil {
			continue
		}
		data.Accounts = append(data.Accounts, acc)
	}

	// 加载 tokens
	tokensMap, err := b.client.HGetAll(b.ctx, redisTokensKey).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("加载 tokens 失败: %w", err)
	}

	for _, jsonStr := range tokensMap {
		var t Token
		if err := json.Unmarshal([]byte(jsonStr), &t); err != nil {
			continue
		}
		data.Tokens = append(data.Tokens, t)
	}

	// 加载 settings
	settingsMap, err := b.client.HGetAll(b.ctx, redisSettingsKey).Result()
	if err == nil {
		if v, ok := settingsMap["sync_interval"]; ok {
			fmt.Sscanf(v, "%d", &data.Settings.SyncInterval)
		}
		if v, ok := settingsMap["endpoint_proxy"]; ok {
			data.Settings.EndpointProxy = v == "true"
		}
		if v, ok := settingsMap["endpoint_proxy_url"]; ok {
			data.Settings.EndpointProxyURL = v
		}
		if v, ok := settingsMap["default_expiration_days"]; ok {
			fmt.Sscanf(v, "%d", &data.Settings.DefaultExpirationDays)
		}
		if v, ok := settingsMap["expiration_check_minutes"]; ok {
			fmt.Sscanf(v, "%d", &data.Settings.ExpirationCheckMinutes)
		}
		if v, ok := settingsMap["s3_virtual_hosted_style"]; ok {
			data.Settings.S3VirtualHostedStyle = (v == "true" || v == "1")
		}
		if v, ok := settingsMap["s3_base_domain"]; ok {
			data.Settings.S3BaseDomain = v
		}
	}
	if data.Settings.SyncInterval <= 0 {
		data.Settings.SyncInterval = 5
	}
	if data.Settings.DefaultExpirationDays <= 0 {
		data.Settings.DefaultExpirationDays = 30
	}
	if data.Settings.ExpirationCheckMinutes <= 0 {
		data.Settings.ExpirationCheckMinutes = 720
	}

	// 加载 s3_credentials
	s3CredsMap, err := b.client.HGetAll(b.ctx, redisS3CredentialsKey).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("加载 s3_credentials 失败: %w", err)
	}

	for _, jsonStr := range s3CredsMap {
		var cred S3Credential
		if err := json.Unmarshal([]byte(jsonStr), &cred); err != nil {
			continue
		}
		data.S3Credentials = append(data.S3Credentials, cred)
	}

	// 加载 webdav_credentials
	webdavCredsMap, err := b.client.HGetAll(b.ctx, redisWebDAVCredentialsKey).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("加载 webdav_credentials 失败: %w", err)
	}

	for _, jsonStr := range webdavCredsMap {
		var cred WebDAVCredential
		if err := json.Unmarshal([]byte(jsonStr), &cred); err != nil {
			continue
		}
		data.WebDAVCredentials = append(data.WebDAVCredentials, cred)
	}

	// 加载 file_expirations
	fileExpMap, err := b.client.HGetAll(b.ctx, redisFileExpirationsKey).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("加载 file_expirations 失败: %w", err)
	}

	for _, jsonStr := range fileExpMap {
		var exp FileExpiration
		if err := json.Unmarshal([]byte(jsonStr), &exp); err != nil {
			continue
		}
		data.FileExpirations = append(data.FileExpirations, exp)
	}

	return data, nil
}

// Save 保存全部数据到 Redis
func (b *RedisBackend) Save(data *Data) error {
	pipe := b.client.Pipeline()

	// 删除旧数据
	pipe.Del(b.ctx, redisAccountsKey)
	pipe.Del(b.ctx, redisTokensKey)
	pipe.Del(b.ctx, redisS3CredentialsKey)
	pipe.Del(b.ctx, redisWebDAVCredentialsKey)
	pipe.Del(b.ctx, redisFileExpirationsKey)

	// 保存 accounts
	if len(data.Accounts) > 0 {
		accountsMap := make(map[string]string)
		for _, acc := range data.Accounts {
			jsonBytes, err := json.Marshal(acc)
			if err != nil {
				return fmt.Errorf("序列化 account 失败: %w", err)
			}
			accountsMap[acc.ID] = string(jsonBytes)
		}
		pipe.HSet(b.ctx, redisAccountsKey, accountsMap)
	}

	// 保存 tokens
	if len(data.Tokens) > 0 {
		tokensMap := make(map[string]string)
		for _, t := range data.Tokens {
			jsonBytes, err := json.Marshal(t)
			if err != nil {
				return fmt.Errorf("序列化 token 失败: %w", err)
			}
			tokensMap[t.ID] = string(jsonBytes)
		}
		pipe.HSet(b.ctx, redisTokensKey, tokensMap)
	}

	// 保存 settings
	pipe.HSet(b.ctx, redisSettingsKey, "sync_interval", fmt.Sprintf("%d", data.Settings.SyncInterval))
	endpointProxyVal := "false"
	if data.Settings.EndpointProxy {
		endpointProxyVal = "true"
	}
	pipe.HSet(b.ctx, redisSettingsKey, "endpoint_proxy", endpointProxyVal)
	pipe.HSet(b.ctx, redisSettingsKey, "endpoint_proxy_url", data.Settings.EndpointProxyURL)
	pipe.HSet(b.ctx, redisSettingsKey, "default_expiration_days", fmt.Sprintf("%d", data.Settings.DefaultExpirationDays))
	pipe.HSet(b.ctx, redisSettingsKey, "expiration_check_minutes", fmt.Sprintf("%d", data.Settings.ExpirationCheckMinutes))
	s3VirtualHostedStyleVal := "false"
	if data.Settings.S3VirtualHostedStyle {
		s3VirtualHostedStyleVal = "true"
	}
	pipe.HSet(b.ctx, redisSettingsKey, "s3_virtual_hosted_style", s3VirtualHostedStyleVal)
	pipe.HSet(b.ctx, redisSettingsKey, "s3_base_domain", data.Settings.S3BaseDomain)

	// 保存 s3_credentials
	if len(data.S3Credentials) > 0 {
		s3CredsMap := make(map[string]string)
		for _, cred := range data.S3Credentials {
			jsonBytes, err := json.Marshal(cred)
			if err != nil {
				return fmt.Errorf("序列化 s3_credential 失败: %w", err)
			}
			s3CredsMap[cred.ID] = string(jsonBytes)
		}
		pipe.HSet(b.ctx, redisS3CredentialsKey, s3CredsMap)
	}

	// 保存 webdav_credentials
	if len(data.WebDAVCredentials) > 0 {
		webdavCredsMap := make(map[string]string)
		for _, cred := range data.WebDAVCredentials {
			jsonBytes, err := json.Marshal(cred)
			if err != nil {
				return fmt.Errorf("序列化 webdav_credential 失败: %w", err)
			}
			webdavCredsMap[cred.ID] = string(jsonBytes)
		}
		pipe.HSet(b.ctx, redisWebDAVCredentialsKey, webdavCredsMap)
	}

	// 保存 file_expirations
	if len(data.FileExpirations) > 0 {
		fileExpMap := make(map[string]string)
		for _, exp := range data.FileExpirations {
			jsonBytes, err := json.Marshal(exp)
			if err != nil {
				return fmt.Errorf("序列化 file_expiration 失败: %w", err)
			}
			fileExpMap[exp.ID] = string(jsonBytes)
		}
		pipe.HSet(b.ctx, redisFileExpirationsKey, fileExpMap)
	}

	_, err := pipe.Exec(b.ctx)
	if err != nil {
		return fmt.Errorf("保存到 Redis 失败: %w", err)
	}

	return nil
}

// Close 关闭 Redis 连接
func (b *RedisBackend) Close() error {
	if b.client != nil {
		return b.client.Close()
	}
	return nil
}
