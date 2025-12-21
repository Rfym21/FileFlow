package store

import (
	"fmt"
	"path/filepath"
	"strings"

	"fileflow/server/config"
)

// Backend 数据库后端接口
type Backend interface {
	// Init 初始化连接并创建表结构
	Init() error
	// Load 加载全部数据到内存
	Load() (*Data, error)
	// Save 保存全部数据
	Save(data *Data) error
	// Close 关闭连接
	Close() error
}

// BackendType 数据库类型
type BackendType string

const (
	BackendSQLite   BackendType = "sqlite"
	BackendTurso    BackendType = "turso"
	BackendRedis    BackendType = "redis"
	BackendMySQL    BackendType = "mysql"
	BackendMongoDB  BackendType = "mongodb"
	BackendPostgres BackendType = "postgres"
)

// ParseDatabaseURL 解析数据库 URL 并返回对应的后端类型和连接字符串
func ParseDatabaseURL(url string) (BackendType, string) {
	cfg := config.Get()

	// 空 URL 使用默认 SQLite
	if url == "" {
		defaultPath := filepath.Join(cfg.DataDir, "fileflow.db")
		return BackendSQLite, defaultPath
	}

	// sqlite:路径
	if strings.HasPrefix(url, "sqlite:") {
		return BackendSQLite, strings.TrimPrefix(url, "sqlite:")
	}

	// libsql:// Turso
	if strings.HasPrefix(url, "libsql://") {
		return BackendTurso, url
	}

	// redis://
	if strings.HasPrefix(url, "redis://") {
		return BackendRedis, url
	}

	// mysql://
	if strings.HasPrefix(url, "mysql://") {
		return BackendMySQL, url
	}

	// mongodb://
	if strings.HasPrefix(url, "mongodb://") || strings.HasPrefix(url, "mongodb+srv://") {
		return BackendMongoDB, url
	}

	// postgres:// 或 postgresql://
	if strings.HasPrefix(url, "postgres://") || strings.HasPrefix(url, "postgresql://") {
		return BackendPostgres, url
	}

	// 默认当作 SQLite 路径
	return BackendSQLite, url
}

// NewBackend 根据配置创建数据库后端
func NewBackend() (Backend, error) {
	cfg := config.Get()
	backendType, connStr := ParseDatabaseURL(cfg.DatabaseURL)

	switch backendType {
	case BackendSQLite:
		return NewSQLiteBackend(connStr)
	case BackendTurso:
		return NewTursoBackend(connStr)
	case BackendRedis:
		return NewRedisBackend(connStr)
	case BackendMySQL:
		return NewMySQLBackend(connStr)
	case BackendMongoDB:
		return NewMongoBackend(connStr)
	case BackendPostgres:
		return NewPostgresBackend(connStr)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", backendType)
	}
}
