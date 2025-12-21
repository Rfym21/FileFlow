package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	mongoDBName                = "fileflow"
	mongoAccountsColl          = "accounts"
	mongoTokensColl            = "tokens"
	mongoSettingsColl          = "settings"
	mongoS3CredentialsColl     = "s3_credentials"
	mongoWebDAVCredentialsColl = "webdav_credentials"
)

// MongoBackend MongoDB 数据库后端
type MongoBackend struct {
	client  *mongo.Client
	db      *mongo.Database
	connStr string
	ctx     context.Context
}

// MongoAccount MongoDB 中的 Account 文档结构
type MongoAccount struct {
	ID              string `bson:"_id"`
	Name            string `bson:"name"`
	IsActive        bool   `bson:"isActive"`
	Description     string `bson:"description"`
	AccountID       string `bson:"accountId"`
	AccessKeyId     string `bson:"accessKeyId"`
	SecretAccessKey string `bson:"secretAccessKey"`
	BucketName      string `bson:"bucketName"`
	Endpoint        string `bson:"endpoint"`
	PublicDomain    string `bson:"publicDomain"`
	APIToken        string `bson:"apiToken"`
	Quota           struct {
		MaxSizeBytes int64 `bson:"maxSizeBytes"`
		MaxClassAOps int64 `bson:"maxClassAOps"`
	} `bson:"quota"`
	Usage struct {
		SizeBytes  int64  `bson:"sizeBytes"`
		ClassAOps  int64  `bson:"classAOps"`
		ClassBOps  int64  `bson:"classBOps"`
		LastSyncAt string `bson:"lastSyncAt"`
	} `bson:"usage"`
	CreatedAt string `bson:"createdAt"`
	UpdatedAt string `bson:"updatedAt"`
}

// MongoToken MongoDB 中的 Token 文档结构
type MongoToken struct {
	ID          string   `bson:"_id"`
	Name        string   `bson:"name"`
	Token       string   `bson:"token"`
	Permissions []string `bson:"permissions"`
	CreatedAt   string   `bson:"createdAt"`
}

// MongoS3Credential MongoDB 中的 S3Credential 文档结构
type MongoS3Credential struct {
	ID              string   `bson:"_id"`
	AccessKeyID     string   `bson:"accessKeyId"`
	SecretAccessKey string   `bson:"secretAccessKey"`
	AccountID       string   `bson:"accountId"`
	Description     string   `bson:"description"`
	Permissions     []string `bson:"permissions"`
	IsActive        bool     `bson:"isActive"`
	CreatedAt       string   `bson:"createdAt"`
	LastUsedAt      string   `bson:"lastUsedAt"`
}

// MongoWebDAVCredential MongoDB 中的 WebDAVCredential 文档结构
type MongoWebDAVCredential struct {
	ID          string   `bson:"_id"`
	Username    string   `bson:"username"`
	Password    string   `bson:"password"`
	AccountID   string   `bson:"accountId"`
	Description string   `bson:"description"`
	Permissions []string `bson:"permissions"`
	IsActive    bool     `bson:"isActive"`
	CreatedAt   string   `bson:"createdAt"`
	LastUsedAt  string   `bson:"lastUsedAt"`
}

// NewMongoBackend 创建 MongoDB 后端
func NewMongoBackend(connStr string) (*MongoBackend, error) {
	return &MongoBackend{
		connStr: connStr,
		ctx:     context.Background(),
	}, nil
}

// Init 初始化 MongoDB 连接
func (b *MongoBackend) Init() error {
	ctx, cancel := context.WithTimeout(b.ctx, 10*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(b.connStr)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return fmt.Errorf("连接 MongoDB 失败: %w", err)
	}

	// 测试连接
	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("MongoDB 连接测试失败: %w", err)
	}

	b.client = client
	b.db = client.Database(mongoDBName)

	// 创建索引
	if err := b.createIndexes(); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	return nil
}

// createIndexes 创建索引
func (b *MongoBackend) createIndexes() error {
	// tokens 集合的 token 字段唯一索引
	tokensColl := b.db.Collection(mongoTokensColl)
	_, err := tokensColl.Indexes().CreateOne(b.ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "token", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// s3_credentials 集合的 accessKeyId 字段唯一索引
	s3CredsColl := b.db.Collection(mongoS3CredentialsColl)
	_, err = s3CredsColl.Indexes().CreateOne(b.ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "accessKeyId", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// webdav_credentials 集合的 username 字段唯一索引
	webdavCredsColl := b.db.Collection(mongoWebDAVCredentialsColl)
	_, err = webdavCredsColl.Indexes().CreateOne(b.ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// Load 从 MongoDB 加载全部数据
func (b *MongoBackend) Load() (*Data, error) {
	data := &Data{
		Accounts:          []Account{},
		Tokens:            []Token{},
		S3Credentials:     []S3Credential{},
		WebDAVCredentials: []WebDAVCredential{},
	}

	// 加载 accounts
	accountsColl := b.db.Collection(mongoAccountsColl)
	cursor, err := accountsColl.Find(b.ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("查询 accounts 失败: %w", err)
	}
	defer cursor.Close(b.ctx)

	for cursor.Next(b.ctx) {
		var doc MongoAccount
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		acc := Account{
			ID:              doc.ID,
			Name:            doc.Name,
			IsActive:        doc.IsActive,
			Description:     doc.Description,
			AccountID:       doc.AccountID,
			AccessKeyId:     doc.AccessKeyId,
			SecretAccessKey: doc.SecretAccessKey,
			BucketName:      doc.BucketName,
			Endpoint:        doc.Endpoint,
			PublicDomain:    doc.PublicDomain,
			APIToken:        doc.APIToken,
			Quota: Quota{
				MaxSizeBytes: doc.Quota.MaxSizeBytes,
				MaxClassAOps: doc.Quota.MaxClassAOps,
			},
			Usage: Usage{
				SizeBytes:  doc.Usage.SizeBytes,
				ClassAOps:  doc.Usage.ClassAOps,
				ClassBOps:  doc.Usage.ClassBOps,
				LastSyncAt: doc.Usage.LastSyncAt,
			},
			CreatedAt: doc.CreatedAt,
			UpdatedAt: doc.UpdatedAt,
		}
		data.Accounts = append(data.Accounts, acc)
	}

	// 加载 tokens
	tokensColl := b.db.Collection(mongoTokensColl)
	cursor, err = tokensColl.Find(b.ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("查询 tokens 失败: %w", err)
	}
	defer cursor.Close(b.ctx)

	for cursor.Next(b.ctx) {
		var doc MongoToken
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		t := Token{
			ID:          doc.ID,
			Name:        doc.Name,
			Token:       doc.Token,
			Permissions: doc.Permissions,
			CreatedAt:   doc.CreatedAt,
		}
		if t.Permissions == nil {
			t.Permissions = []string{}
		}
		data.Tokens = append(data.Tokens, t)
	}

	// 加载 settings
	settingsColl := b.db.Collection(mongoSettingsColl)
	var settingsDoc struct {
		Key   string `bson:"_id"`
		Value int    `bson:"value"`
	}
	err = settingsColl.FindOne(b.ctx, bson.M{"_id": "sync_interval"}).Decode(&settingsDoc)
	if err == nil {
		data.Settings.SyncInterval = settingsDoc.Value
	}
	if data.Settings.SyncInterval <= 0 {
		data.Settings.SyncInterval = 5
	}

	var endpointProxyDoc struct {
		Key   string `bson:"_id"`
		Value bool   `bson:"value"`
	}
	err = settingsColl.FindOne(b.ctx, bson.M{"_id": "endpoint_proxy"}).Decode(&endpointProxyDoc)
	if err == nil {
		data.Settings.EndpointProxy = endpointProxyDoc.Value
	}

	var endpointProxyURLDoc struct {
		Key   string `bson:"_id"`
		Value string `bson:"value"`
	}
	err = settingsColl.FindOne(b.ctx, bson.M{"_id": "endpoint_proxy_url"}).Decode(&endpointProxyURLDoc)
	if err == nil {
		data.Settings.EndpointProxyURL = endpointProxyURLDoc.Value
	}

	// 加载 s3_credentials
	s3CredsColl := b.db.Collection(mongoS3CredentialsColl)
	cursor, err = s3CredsColl.Find(b.ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("查询 s3_credentials 失败: %w", err)
	}
	defer cursor.Close(b.ctx)

	for cursor.Next(b.ctx) {
		var doc MongoS3Credential
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		cred := S3Credential{
			ID:              doc.ID,
			AccessKeyID:     doc.AccessKeyID,
			SecretAccessKey: doc.SecretAccessKey,
			AccountID:       doc.AccountID,
			Description:     doc.Description,
			Permissions:     doc.Permissions,
			IsActive:        doc.IsActive,
			CreatedAt:       doc.CreatedAt,
			LastUsedAt:      doc.LastUsedAt,
		}
		if cred.Permissions == nil {
			cred.Permissions = []string{}
		}
		data.S3Credentials = append(data.S3Credentials, cred)
	}

	// 加载 webdav_credentials
	webdavCredsColl := b.db.Collection(mongoWebDAVCredentialsColl)
	cursor, err = webdavCredsColl.Find(b.ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("查询 webdav_credentials 失败: %w", err)
	}
	defer cursor.Close(b.ctx)

	for cursor.Next(b.ctx) {
		var doc MongoWebDAVCredential
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		cred := WebDAVCredential{
			ID:          doc.ID,
			Username:    doc.Username,
			Password:    doc.Password,
			AccountID:   doc.AccountID,
			Description: doc.Description,
			Permissions: doc.Permissions,
			IsActive:    doc.IsActive,
			CreatedAt:   doc.CreatedAt,
			LastUsedAt:  doc.LastUsedAt,
		}
		if cred.Permissions == nil {
			cred.Permissions = []string{}
		}
		data.WebDAVCredentials = append(data.WebDAVCredentials, cred)
	}

	return data, nil
}

// Save 保存全部数据到 MongoDB
func (b *MongoBackend) Save(data *Data) error {
	// 使用事务（如果 MongoDB 支持）
	session, err := b.client.StartSession()
	if err != nil {
		// 如果不支持事务，直接执行
		return b.saveWithoutTransaction(data)
	}
	defer session.EndSession(b.ctx)

	_, err = session.WithTransaction(b.ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// 清空并重新插入 accounts
		accountsColl := b.db.Collection(mongoAccountsColl)
		if _, err := accountsColl.DeleteMany(sessCtx, bson.M{}); err != nil {
			return nil, fmt.Errorf("清空 accounts 失败: %w", err)
		}

		if len(data.Accounts) > 0 {
			docs := make([]interface{}, len(data.Accounts))
			for i, acc := range data.Accounts {
				docs[i] = MongoAccount{
					ID:              acc.ID,
					Name:            acc.Name,
					IsActive:        acc.IsActive,
					Description:     acc.Description,
					AccountID:       acc.AccountID,
					AccessKeyId:     acc.AccessKeyId,
					SecretAccessKey: acc.SecretAccessKey,
					BucketName:      acc.BucketName,
					Endpoint:        acc.Endpoint,
					PublicDomain:    acc.PublicDomain,
					APIToken:        acc.APIToken,
					Quota: struct {
						MaxSizeBytes int64 `bson:"maxSizeBytes"`
						MaxClassAOps int64 `bson:"maxClassAOps"`
					}{
						MaxSizeBytes: acc.Quota.MaxSizeBytes,
						MaxClassAOps: acc.Quota.MaxClassAOps,
					},
					Usage: struct {
						SizeBytes  int64  `bson:"sizeBytes"`
						ClassAOps  int64  `bson:"classAOps"`
						ClassBOps  int64  `bson:"classBOps"`
						LastSyncAt string `bson:"lastSyncAt"`
					}{
						SizeBytes:  acc.Usage.SizeBytes,
						ClassAOps:  acc.Usage.ClassAOps,
						ClassBOps:  acc.Usage.ClassBOps,
						LastSyncAt: acc.Usage.LastSyncAt,
					},
					CreatedAt: acc.CreatedAt,
					UpdatedAt: acc.UpdatedAt,
				}
			}
			if _, err := accountsColl.InsertMany(sessCtx, docs); err != nil {
				return nil, fmt.Errorf("插入 accounts 失败: %w", err)
			}
		}

		// 清空并重新插入 tokens
		tokensColl := b.db.Collection(mongoTokensColl)
		if _, err := tokensColl.DeleteMany(sessCtx, bson.M{}); err != nil {
			return nil, fmt.Errorf("清空 tokens 失败: %w", err)
		}

		if len(data.Tokens) > 0 {
			docs := make([]interface{}, len(data.Tokens))
			for i, t := range data.Tokens {
				docs[i] = MongoToken{
					ID:          t.ID,
					Name:        t.Name,
					Token:       t.Token,
					Permissions: t.Permissions,
					CreatedAt:   t.CreatedAt,
				}
			}
			if _, err := tokensColl.InsertMany(sessCtx, docs); err != nil {
				return nil, fmt.Errorf("插入 tokens 失败: %w", err)
			}
		}

		// 保存 settings
		settingsColl := b.db.Collection(mongoSettingsColl)
		_, err := settingsColl.UpdateOne(sessCtx,
			bson.M{"_id": "sync_interval"},
			bson.M{"$set": bson.M{"value": data.Settings.SyncInterval}},
			options.Update().SetUpsert(true))
		if err != nil {
			return nil, fmt.Errorf("保存 settings 失败: %w", err)
		}

		_, err = settingsColl.UpdateOne(sessCtx,
			bson.M{"_id": "endpoint_proxy"},
			bson.M{"$set": bson.M{"value": data.Settings.EndpointProxy}},
			options.Update().SetUpsert(true))
		if err != nil {
			return nil, fmt.Errorf("保存 settings 失败: %w", err)
		}

		_, err = settingsColl.UpdateOne(sessCtx,
			bson.M{"_id": "endpoint_proxy_url"},
			bson.M{"$set": bson.M{"value": data.Settings.EndpointProxyURL}},
			options.Update().SetUpsert(true))
		if err != nil {
			return nil, fmt.Errorf("保存 settings 失败: %w", err)
		}

		// 清空并重新插入 s3_credentials
		s3CredsColl := b.db.Collection(mongoS3CredentialsColl)
		if _, err := s3CredsColl.DeleteMany(sessCtx, bson.M{}); err != nil {
			return nil, fmt.Errorf("清空 s3_credentials 失败: %w", err)
		}

		if len(data.S3Credentials) > 0 {
			docs := make([]interface{}, len(data.S3Credentials))
			for i, cred := range data.S3Credentials {
				docs[i] = MongoS3Credential{
					ID:              cred.ID,
					AccessKeyID:     cred.AccessKeyID,
					SecretAccessKey: cred.SecretAccessKey,
					AccountID:       cred.AccountID,
					Description:     cred.Description,
					Permissions:     cred.Permissions,
					IsActive:        cred.IsActive,
					CreatedAt:       cred.CreatedAt,
					LastUsedAt:      cred.LastUsedAt,
				}
			}
			if _, err := s3CredsColl.InsertMany(sessCtx, docs); err != nil {
				return nil, fmt.Errorf("插入 s3_credentials 失败: %w", err)
			}
		}

		// 清空并重新插入 webdav_credentials
		webdavCredsColl := b.db.Collection(mongoWebDAVCredentialsColl)
		if _, err := webdavCredsColl.DeleteMany(sessCtx, bson.M{}); err != nil {
			return nil, fmt.Errorf("清空 webdav_credentials 失败: %w", err)
		}

		if len(data.WebDAVCredentials) > 0 {
			docs := make([]interface{}, len(data.WebDAVCredentials))
			for i, cred := range data.WebDAVCredentials {
				docs[i] = MongoWebDAVCredential{
					ID:          cred.ID,
					Username:    cred.Username,
					Password:    cred.Password,
					AccountID:   cred.AccountID,
					Description: cred.Description,
					Permissions: cred.Permissions,
					IsActive:    cred.IsActive,
					CreatedAt:   cred.CreatedAt,
					LastUsedAt:  cred.LastUsedAt,
				}
			}
			if _, err := webdavCredsColl.InsertMany(sessCtx, docs); err != nil {
				return nil, fmt.Errorf("插入 webdav_credentials 失败: %w", err)
			}
		}

		return nil, nil
	})

	return err
}

// saveWithoutTransaction 不使用事务保存数据
func (b *MongoBackend) saveWithoutTransaction(data *Data) error {
	// 清空并重新插入 accounts
	accountsColl := b.db.Collection(mongoAccountsColl)
	if _, err := accountsColl.DeleteMany(b.ctx, bson.M{}); err != nil {
		return fmt.Errorf("清空 accounts 失败: %w", err)
	}

	if len(data.Accounts) > 0 {
		docs := make([]interface{}, len(data.Accounts))
		for i, acc := range data.Accounts {
			docs[i] = MongoAccount{
				ID:              acc.ID,
				Name:            acc.Name,
				IsActive:        acc.IsActive,
				Description:     acc.Description,
				AccountID:       acc.AccountID,
				AccessKeyId:     acc.AccessKeyId,
				SecretAccessKey: acc.SecretAccessKey,
				BucketName:      acc.BucketName,
				Endpoint:        acc.Endpoint,
				PublicDomain:    acc.PublicDomain,
				APIToken:        acc.APIToken,
				Quota: struct {
					MaxSizeBytes int64 `bson:"maxSizeBytes"`
					MaxClassAOps int64 `bson:"maxClassAOps"`
				}{
					MaxSizeBytes: acc.Quota.MaxSizeBytes,
					MaxClassAOps: acc.Quota.MaxClassAOps,
				},
				Usage: struct {
					SizeBytes  int64  `bson:"sizeBytes"`
					ClassAOps  int64  `bson:"classAOps"`
					ClassBOps  int64  `bson:"classBOps"`
					LastSyncAt string `bson:"lastSyncAt"`
				}{
					SizeBytes:  acc.Usage.SizeBytes,
					ClassAOps:  acc.Usage.ClassAOps,
					ClassBOps:  acc.Usage.ClassBOps,
					LastSyncAt: acc.Usage.LastSyncAt,
				},
				CreatedAt: acc.CreatedAt,
				UpdatedAt: acc.UpdatedAt,
			}
		}
		if _, err := accountsColl.InsertMany(b.ctx, docs); err != nil {
			return fmt.Errorf("插入 accounts 失败: %w", err)
		}
	}

	// 清空并重新插入 tokens
	tokensColl := b.db.Collection(mongoTokensColl)
	if _, err := tokensColl.DeleteMany(b.ctx, bson.M{}); err != nil {
		return fmt.Errorf("清空 tokens 失败: %w", err)
	}

	if len(data.Tokens) > 0 {
		docs := make([]interface{}, len(data.Tokens))
		for i, t := range data.Tokens {
			docs[i] = MongoToken{
				ID:          t.ID,
				Name:        t.Name,
				Token:       t.Token,
				Permissions: t.Permissions,
				CreatedAt:   t.CreatedAt,
			}
		}
		if _, err := tokensColl.InsertMany(b.ctx, docs); err != nil {
			return fmt.Errorf("插入 tokens 失败: %w", err)
		}
	}

	// 保存 settings
	settingsColl := b.db.Collection(mongoSettingsColl)
	_, err := settingsColl.UpdateOne(b.ctx,
		bson.M{"_id": "sync_interval"},
		bson.M{"$set": bson.M{"value": data.Settings.SyncInterval}},
		options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	_, err = settingsColl.UpdateOne(b.ctx,
		bson.M{"_id": "endpoint_proxy"},
		bson.M{"$set": bson.M{"value": data.Settings.EndpointProxy}},
		options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	_, err = settingsColl.UpdateOne(b.ctx,
		bson.M{"_id": "endpoint_proxy_url"},
		bson.M{"$set": bson.M{"value": data.Settings.EndpointProxyURL}},
		options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("保存 settings 失败: %w", err)
	}

	// 清空并重新插入 s3_credentials
	s3CredsColl := b.db.Collection(mongoS3CredentialsColl)
	if _, err := s3CredsColl.DeleteMany(b.ctx, bson.M{}); err != nil {
		return fmt.Errorf("清空 s3_credentials 失败: %w", err)
	}

	if len(data.S3Credentials) > 0 {
		docs := make([]interface{}, len(data.S3Credentials))
		for i, cred := range data.S3Credentials {
			docs[i] = MongoS3Credential{
				ID:              cred.ID,
				AccessKeyID:     cred.AccessKeyID,
				SecretAccessKey: cred.SecretAccessKey,
				AccountID:       cred.AccountID,
				Description:     cred.Description,
				Permissions:     cred.Permissions,
				IsActive:        cred.IsActive,
				CreatedAt:       cred.CreatedAt,
				LastUsedAt:      cred.LastUsedAt,
			}
		}
		if _, err := s3CredsColl.InsertMany(b.ctx, docs); err != nil {
			return fmt.Errorf("插入 s3_credentials 失败: %w", err)
		}
	}

	// 清空并重新插入 webdav_credentials
	webdavCredsColl := b.db.Collection(mongoWebDAVCredentialsColl)
	if _, err := webdavCredsColl.DeleteMany(b.ctx, bson.M{}); err != nil {
		return fmt.Errorf("清空 webdav_credentials 失败: %w", err)
	}

	if len(data.WebDAVCredentials) > 0 {
		docs := make([]interface{}, len(data.WebDAVCredentials))
		for i, cred := range data.WebDAVCredentials {
			docs[i] = MongoWebDAVCredential{
				ID:          cred.ID,
				Username:    cred.Username,
				Password:    cred.Password,
				AccountID:   cred.AccountID,
				Description: cred.Description,
				Permissions: cred.Permissions,
				IsActive:    cred.IsActive,
				CreatedAt:   cred.CreatedAt,
				LastUsedAt:  cred.LastUsedAt,
			}
		}
		if _, err := webdavCredsColl.InsertMany(b.ctx, docs); err != nil {
			return fmt.Errorf("插入 webdav_credentials 失败: %w", err)
		}
	}

	return nil
}

// Close 关闭 MongoDB 连接
func (b *MongoBackend) Close() error {
	if b.client != nil {
		return b.client.Disconnect(b.ctx)
	}
	return nil
}
