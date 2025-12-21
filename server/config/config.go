package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config 应用配置
type Config struct {
	AdminUser     string
	AdminPassword string
	JWTSecret     string
	Port          string
	DataDir       string
	DatabaseURL   string
}

var cfg *Config

// Load 加载配置
func Load() *Config {
	if cfg != nil {
		return cfg
	}

	// 加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		log.Println("未找到 .env 文件，使用环境变量")
	}

	cfg = &Config{
		AdminUser:     getEnv("FILEFLOW_ADMIN_USER", "admin"),
		AdminPassword: getEnv("FILEFLOW_ADMIN_PASSWORD", ""),
		JWTSecret:     getEnv("FILEFLOW_JWT_SECRET", ""),
		Port:          getEnv("FILEFLOW_PORT", "8080"),
		DataDir:       getEnv("FILEFLOW_DATA_DIR", "data"),
		DatabaseURL:   getEnv("FILEFLOW_DATABASE_URL", ""),
	}

	// 验证必要配置
	if cfg.AdminPassword == "" {
		log.Fatal("FILEFLOW_ADMIN_PASSWORD 未设置")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("FILEFLOW_JWT_SECRET 未设置")
	}

	return cfg
}

// Get 获取配置实例
func Get() *Config {
	if cfg == nil {
		return Load()
	}
	return cfg
}

// getEnv 获取环境变量，支持默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
