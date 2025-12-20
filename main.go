package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"fileflow/server/api"
	"fileflow/server/config"
	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

//go:embed client/dist/*
var staticFiles embed.FS

func main() {
	// 加载配置
	cfg := config.Load()
	log.Printf("FileFlow 启动中，端口: %s", cfg.Port)

	// 初始化存储
	if err := store.Init(); err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}

	// 启动定时任务
	service.StartScheduler()

	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	// 创建路由
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 配置 CORS
	r.Use(corsMiddleware())

	// 配置 API 路由
	api.SetupRouter(r)

	// 配置静态文件服务
	setupStaticFiles(r)

	// 优雅关闭
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("正在关闭服务...")
		service.StopScheduler()
		os.Exit(0)
	}()

	// 启动服务
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}

// corsMiddleware CORS 中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Token")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// setupStaticFiles 配置静态文件服务
func setupStaticFiles(r *gin.Engine) {
	// 尝试使用嵌入的静态文件
	subFS, err := fs.Sub(staticFiles, "client/dist")
	if err != nil {
		log.Println("未找到嵌入的静态文件，跳过静态文件服务")
		return
	}

	// 检查是否有内容
	entries, err := fs.ReadDir(subFS, ".")
	if err != nil || len(entries) == 0 {
		log.Println("静态文件为空，跳过静态文件服务")
		return
	}

	// assets 子目录
	assetsFS, err := fs.Sub(subFS, "assets")
	if err == nil {
		r.StaticFS("/assets", http.FS(assetsFS))
	}

	// favicon
	r.GET("/favicon.svg", func(c *gin.Context) {
		content, err := fs.ReadFile(subFS, "favicon.svg")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "image/svg+xml", content)
	})

	// SPA 路由处理
	r.NoRoute(func(c *gin.Context) {
		// API 路由返回 404
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}

		// 其他路由返回 index.html
		content, err := fs.ReadFile(subFS, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "页面不存在")
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	log.Println("静态文件服务已启用")
}
