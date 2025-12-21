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
	}
	if data.Settings.SyncInterval <= 0 {
		data.Settings.SyncInterval = 5
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
