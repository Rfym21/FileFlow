package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresBackend PostgreSQL 数据库后端
type PostgresBackend struct {
	db      *sql.DB
	connStr string
}

// NewPostgresBackend 创建 PostgreSQL 后端
func NewPostgresBackend(connStr string) (*PostgresBackend, error) {
	return &PostgresBackend{connStr: connStr}, nil
}

// Init 初始化数据库连接和表结构
func (b *PostgresBackend) Init() error {
	db, err := sql.Open("postgres", b.connStr)
	if err != nil {
		return fmt.Errorf("打开 PostgreSQL 数据库失败: %w", err)
	}
	b.db = db

	// 测试连接
	if err := b.db.Ping(); err != nil {
		return fmt.Errorf("PostgreSQL 连接测试失败: %w", err)
	}

	// 创建表结构
	if err := b.createTables(); err != nil {
		return fmt.Errorf("创建表结构失败: %w", err)
	}

	return nil
}

// createTables 创建数据库表
func (b *PostgresBackend) createTables() error {
	// 创建 accounts 表
	_, err := b.db.Exec(`
		CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			is_active BOOLEAN DEFAULT true,
			description TEXT,
			account_id TEXT,
			access_key_id TEXT,
			secret_access_key TEXT,
			bucket_name TEXT,
			endpoint TEXT,
			public_domain TEXT,
			api_token TEXT,
			quota_max_size_bytes BIGINT DEFAULT 0,
			quota_max_class_a_ops BIGINT DEFAULT 0,
			usage_size_bytes BIGINT DEFAULT 0,
			usage_class_a_ops BIGINT DEFAULT 0,
			usage_class_b_ops BIGINT DEFAULT 0,
			usage_last_sync_at TEXT,
			perm_s3 BOOLEAN DEFAULT true,
			perm_webdav BOOLEAN DEFAULT true,
			perm_auto_upload BOOLEAN DEFAULT true,
			perm_api_upload BOOLEAN DEFAULT true,
			perm_client_upload BOOLEAN DEFAULT true,
			created_at TEXT,
			updated_at TEXT
		)
	`)
	if err != nil {
		return err
	}

	// 创建 tokens 表
	_, err = b.db.Exec(`
		CREATE TABLE IF NOT EXISTS tokens (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			token TEXT UNIQUE NOT NULL,
			permissions TEXT,
			created_at TEXT
		)
	`)
	if err != nil {
		return err
	}

	// 创建 settings 表
	_, err = b.db.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		)
	`)
	if err != nil {
		return err
	}

	// 创建 s3_credentials 表
	_, err = b.db.Exec(`
		CREATE TABLE IF NOT EXISTS s3_credentials (
			id TEXT PRIMARY KEY,
			access_key_id TEXT UNIQUE NOT NULL,
			secret_access_key TEXT NOT NULL,
			account_id TEXT NOT NULL,
			description TEXT,
			permissions TEXT,
			is_active BOOLEAN DEFAULT true,
			created_at TEXT,
			last_used_at TEXT
		)
	`)
	if err != nil {
		return err
	}

	// 创建 webdav_credentials 表
	_, err = b.db.Exec(`
		CREATE TABLE IF NOT EXISTS webdav_credentials (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			account_id TEXT NOT NULL,
			description TEXT,
			permissions TEXT,
			is_active BOOLEAN DEFAULT true,
			created_at TEXT,
			last_used_at TEXT
		)
	`)
	if err != nil {
		return err
	}

	// 创建 file_expirations 表
	_, err = b.db.Exec(`
		CREATE TABLE IF NOT EXISTS file_expirations (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			file_key TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT,
			UNIQUE(account_id, file_key)
		)
	`)
	return err
}

// Load 从数据库加载全部数据
func (b *PostgresBackend) Load() (*Data, error) {
	data := &Data{
		Accounts:          []Account{},
		Tokens:            []Token{},
		S3Credentials:     []S3Credential{},
		WebDAVCredentials: []WebDAVCredential{},
		FileExpirations:   []FileExpiration{},
	}

	// 加载 accounts
	rows, err := b.db.Query(`
		SELECT id, name, is_active, description, account_id, access_key_id,
			secret_access_key, bucket_name, endpoint, public_domain, api_token,
			quota_max_size_bytes, quota_max_class_a_ops,
			usage_size_bytes, usage_class_a_ops, usage_class_b_ops, usage_last_sync_at,
			COALESCE(perm_s3, true), COALESCE(perm_webdav, true), COALESCE(perm_auto_upload, true),
			COALESCE(perm_api_upload, true), COALESCE(perm_client_upload, true),
			created_at, updated_at
		FROM accounts
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 accounts 失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var acc Account
		var description, accountID, accessKeyID, secretAccessKey sql.NullString
		var bucketName, endpoint, publicDomain, apiToken sql.NullString
		var usageLastSyncAt, createdAt, updatedAt sql.NullString

		err := rows.Scan(
			&acc.ID, &acc.Name, &acc.IsActive, &description, &accountID, &accessKeyID,
			&secretAccessKey, &bucketName, &endpoint, &publicDomain, &apiToken,
			&acc.Quota.MaxSizeBytes, &acc.Quota.MaxClassAOps,
			&acc.Usage.SizeBytes, &acc.Usage.ClassAOps, &acc.Usage.ClassBOps, &usageLastSyncAt,
			&acc.Permissions.S3, &acc.Permissions.WebDAV, &acc.Permissions.AutoUpload,
			&acc.Permissions.APIUpload, &acc.Permissions.ClientUpload,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描 account 行失败: %w", err)
		}

		acc.Description = description.String
		acc.AccountID = accountID.String
		acc.AccessKeyId = accessKeyID.String
		acc.SecretAccessKey = secretAccessKey.String
		acc.BucketName = bucketName.String
		acc.Endpoint = endpoint.String
		acc.PublicDomain = publicDomain.String
		acc.APIToken = apiToken.String
		acc.Usage.LastSyncAt = usageLastSyncAt.String
		acc.CreatedAt = createdAt.String
		acc.UpdatedAt = updatedAt.String

		data.Accounts = append(data.Accounts, acc)
	}

	// 加载 tokens
	rows, err = b.db.Query(`SELECT id, name, token, permissions, created_at FROM tokens`)
	if err != nil {
		return nil, fmt.Errorf("查询 tokens 失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t Token
		var permissions sql.NullString
		var createdAt sql.NullString

		err := rows.Scan(&t.ID, &t.Name, &t.Token, &permissions, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("扫描 token 行失败: %w", err)
		}

		if permissions.Valid && permissions.String != "" {
			if err := json.Unmarshal([]byte(permissions.String), &t.Permissions); err != nil {
				t.Permissions = []string{}
			}
		} else {
			t.Permissions = []string{}
		}
		t.CreatedAt = createdAt.String

		data.Tokens = append(data.Tokens, t)
	}

	// 加载 settings
	var syncInterval sql.NullString
	err = b.db.QueryRow(`SELECT value FROM settings WHERE key = 'sync_interval'`).Scan(&syncInterval)
	if err == nil && syncInterval.Valid {
		fmt.Sscanf(syncInterval.String, "%d", &data.Settings.SyncInterval)
	}
	if data.Settings.SyncInterval <= 0 {
		data.Settings.SyncInterval = 5
	}

	var endpointProxy sql.NullString
	err = b.db.QueryRow(`SELECT value FROM settings WHERE key = 'endpoint_proxy'`).Scan(&endpointProxy)
	if err == nil && endpointProxy.Valid {
		data.Settings.EndpointProxy = endpointProxy.String == "true"
	}

	var endpointProxyURL sql.NullString
	err = b.db.QueryRow(`SELECT value FROM settings WHERE key = 'endpoint_proxy_url'`).Scan(&endpointProxyURL)
	if err == nil && endpointProxyURL.Valid {
		data.Settings.EndpointProxyURL = endpointProxyURL.String
	}

	var defaultExpirationDays sql.NullString
	err = b.db.QueryRow(`SELECT value FROM settings WHERE key = 'default_expiration_days'`).Scan(&defaultExpirationDays)
	if err == nil && defaultExpirationDays.Valid {
		fmt.Sscanf(defaultExpirationDays.String, "%d", &data.Settings.DefaultExpirationDays)
	}
	if data.Settings.DefaultExpirationDays <= 0 {
		data.Settings.DefaultExpirationDays = 30
	}

	var expirationCheckMinutes sql.NullString
	err = b.db.QueryRow(`SELECT value FROM settings WHERE key = 'expiration_check_minutes'`).Scan(&expirationCheckMinutes)
	if err == nil && expirationCheckMinutes.Valid {
		fmt.Sscanf(expirationCheckMinutes.String, "%d", &data.Settings.ExpirationCheckMinutes)
	}
	if data.Settings.ExpirationCheckMinutes <= 0 {
		data.Settings.ExpirationCheckMinutes = 720
	}

	var s3VirtualHostedStyle sql.NullString
	err = b.db.QueryRow(`SELECT value FROM settings WHERE key = 's3_virtual_hosted_style'`).Scan(&s3VirtualHostedStyle)
	if err == nil && s3VirtualHostedStyle.Valid {
		data.Settings.S3VirtualHostedStyle = s3VirtualHostedStyle.String == "true"
	}

	var s3BaseDomain sql.NullString
	err = b.db.QueryRow(`SELECT value FROM settings WHERE key = 's3_base_domain'`).Scan(&s3BaseDomain)
	if err == nil && s3BaseDomain.Valid {
		data.Settings.S3BaseDomain = s3BaseDomain.String
	}

	// 加载 s3_credentials
	rows, err = b.db.Query(`
		SELECT id, access_key_id, secret_access_key, account_id, description,
			permissions, is_active, created_at, last_used_at
		FROM s3_credentials
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 s3_credentials 失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cred S3Credential
		var description, permissions, createdAt, lastUsedAt sql.NullString

		err := rows.Scan(
			&cred.ID, &cred.AccessKeyID, &cred.SecretAccessKey, &cred.AccountID,
			&description, &permissions, &cred.IsActive, &createdAt, &lastUsedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描 s3_credential 行失败: %w", err)
		}

		cred.Description = description.String
		if permissions.Valid && permissions.String != "" {
			if err := json.Unmarshal([]byte(permissions.String), &cred.Permissions); err != nil {
				cred.Permissions = []string{}
			}
		} else {
			cred.Permissions = []string{}
		}
		cred.CreatedAt = createdAt.String
		cred.LastUsedAt = lastUsedAt.String

		data.S3Credentials = append(data.S3Credentials, cred)
	}

	// 加载 webdav_credentials
	rows, err = b.db.Query(`
		SELECT id, username, password, account_id, description,
			permissions, is_active, created_at, last_used_at
		FROM webdav_credentials
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 webdav_credentials 失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cred WebDAVCredential
		var description, permissions, createdAt, lastUsedAt sql.NullString

		err := rows.Scan(
			&cred.ID, &cred.Username, &cred.Password, &cred.AccountID,
			&description, &permissions, &cred.IsActive, &createdAt, &lastUsedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描 webdav_credential 行失败: %w", err)
		}

		cred.Description = description.String
		if permissions.Valid && permissions.String != "" {
			if err := json.Unmarshal([]byte(permissions.String), &cred.Permissions); err != nil {
				cred.Permissions = []string{}
			}
		} else {
			cred.Permissions = []string{}
		}
		cred.CreatedAt = createdAt.String
		cred.LastUsedAt = lastUsedAt.String

		data.WebDAVCredentials = append(data.WebDAVCredentials, cred)
	}

	// 加载 file_expirations
	rows, err = b.db.Query(`
		SELECT id, account_id, file_key, expires_at, created_at
		FROM file_expirations
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 file_expirations 失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var exp FileExpiration
		var createdAt sql.NullString

		err := rows.Scan(&exp.ID, &exp.AccountID, &exp.FileKey, &exp.ExpiresAt, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("扫描 file_expiration 行失败: %w", err)
		}

		exp.CreatedAt = createdAt.String
		data.FileExpirations = append(data.FileExpirations, exp)
	}

	return data, nil
}

// Save 保存全部数据到数据库
func (b *PostgresBackend) Save(data *Data) error {
	tx, err := b.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	// 清空并重新插入 accounts
	if _, err := tx.Exec("DELETE FROM accounts"); err != nil {
		return fmt.Errorf("清空 accounts 失败: %w", err)
	}

	for _, acc := range data.Accounts {
		_, err := tx.Exec(`
			INSERT INTO accounts (
				id, name, is_active, description, account_id, access_key_id,
				secret_access_key, bucket_name, endpoint, public_domain, api_token,
				quota_max_size_bytes, quota_max_class_a_ops,
				usage_size_bytes, usage_class_a_ops, usage_class_b_ops, usage_last_sync_at,
				perm_s3, perm_webdav, perm_auto_upload, perm_api_upload, perm_client_upload,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
		`,
			acc.ID, acc.Name, acc.IsActive, acc.Description, acc.AccountID, acc.AccessKeyId,
			acc.SecretAccessKey, acc.BucketName, acc.Endpoint, acc.PublicDomain, acc.APIToken,
			acc.Quota.MaxSizeBytes, acc.Quota.MaxClassAOps,
			acc.Usage.SizeBytes, acc.Usage.ClassAOps, acc.Usage.ClassBOps, acc.Usage.LastSyncAt,
			acc.Permissions.S3, acc.Permissions.WebDAV, acc.Permissions.AutoUpload,
			acc.Permissions.APIUpload, acc.Permissions.ClientUpload,
			acc.CreatedAt, acc.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("插入 account 失败: %w", err)
		}
	}

	// 清空并重新插入 tokens
	if _, err := tx.Exec("DELETE FROM tokens"); err != nil {
		return fmt.Errorf("清空 tokens 失败: %w", err)
	}

	for _, t := range data.Tokens {
		permissions, _ := json.Marshal(t.Permissions)

		_, err := tx.Exec(`
			INSERT INTO tokens (id, name, token, permissions, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, t.ID, t.Name, t.Token, string(permissions), t.CreatedAt)
		if err != nil {
			return fmt.Errorf("插入 token 失败: %w", err)
		}
	}

	// 保存 settings
	_, err = tx.Exec(`
		INSERT INTO settings (key, value) VALUES ('sync_interval', $1)
		ON CONFLICT (key) DO UPDATE SET value = $1
	`, fmt.Sprintf("%d", data.Settings.SyncInterval))
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	endpointProxyVal := "false"
	if data.Settings.EndpointProxy {
		endpointProxyVal = "true"
	}
	_, err = tx.Exec(`
		INSERT INTO settings (key, value) VALUES ('endpoint_proxy', $1)
		ON CONFLICT (key) DO UPDATE SET value = $1
	`, endpointProxyVal)
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO settings (key, value) VALUES ('endpoint_proxy_url', $1)
		ON CONFLICT (key) DO UPDATE SET value = $1
	`, data.Settings.EndpointProxyURL)
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO settings (key, value) VALUES ('default_expiration_days', $1)
		ON CONFLICT (key) DO UPDATE SET value = $1
	`, fmt.Sprintf("%d", data.Settings.DefaultExpirationDays))
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO settings (key, value) VALUES ('expiration_check_minutes', $1)
		ON CONFLICT (key) DO UPDATE SET value = $1
	`, fmt.Sprintf("%d", data.Settings.ExpirationCheckMinutes))
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	s3VirtualHostedStyleVal := "false"
	if data.Settings.S3VirtualHostedStyle {
		s3VirtualHostedStyleVal = "true"
	}
	_, err = tx.Exec(`
		INSERT INTO settings (key, value) VALUES ('s3_virtual_hosted_style', $1)
		ON CONFLICT (key) DO UPDATE SET value = $1
	`, s3VirtualHostedStyleVal)
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO settings (key, value) VALUES ('s3_base_domain', $1)
		ON CONFLICT (key) DO UPDATE SET value = $1
	`, data.Settings.S3BaseDomain)
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	// 清空并重新插入 s3_credentials
	if _, err := tx.Exec("DELETE FROM s3_credentials"); err != nil {
		return fmt.Errorf("清空 s3_credentials 失败: %w", err)
	}

	for _, cred := range data.S3Credentials {
		permissions, _ := json.Marshal(cred.Permissions)

		_, err := tx.Exec(`
			INSERT INTO s3_credentials (
				id, access_key_id, secret_access_key, account_id, description,
				permissions, is_active, created_at, last_used_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			cred.ID, cred.AccessKeyID, cred.SecretAccessKey, cred.AccountID, cred.Description,
			string(permissions), cred.IsActive, cred.CreatedAt, cred.LastUsedAt,
		)
		if err != nil {
			return fmt.Errorf("插入 s3_credential 失败: %w", err)
		}
	}

	// 清空并重新插入 webdav_credentials
	if _, err := tx.Exec("DELETE FROM webdav_credentials"); err != nil {
		return fmt.Errorf("清空 webdav_credentials 失败: %w", err)
	}

	for _, cred := range data.WebDAVCredentials {
		permissions, _ := json.Marshal(cred.Permissions)

		_, err := tx.Exec(`
			INSERT INTO webdav_credentials (
				id, username, password, account_id, description,
				permissions, is_active, created_at, last_used_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			cred.ID, cred.Username, cred.Password, cred.AccountID, cred.Description,
			string(permissions), cred.IsActive, cred.CreatedAt, cred.LastUsedAt,
		)
		if err != nil {
			return fmt.Errorf("插入 webdav_credential 失败: %w", err)
		}
	}

	// 清空并重新插入 file_expirations
	if _, err := tx.Exec("DELETE FROM file_expirations"); err != nil {
		return fmt.Errorf("清空 file_expirations 失败: %w", err)
	}

	for _, exp := range data.FileExpirations {
		_, err := tx.Exec(`
			INSERT INTO file_expirations (id, account_id, file_key, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, exp.ID, exp.AccountID, exp.FileKey, exp.ExpiresAt, exp.CreatedAt)
		if err != nil {
			return fmt.Errorf("插入 file_expiration 失败: %w", err)
		}
	}

	return tx.Commit()
}

// Close 关闭数据库连接
func (b *PostgresBackend) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

