package service

import (
	"context"
	"log"

	"github.com/robfig/cron/v3"
)

var scheduler *cron.Cron

// StartScheduler 启动定时任务调度器
func StartScheduler() {
	scheduler = cron.New()

	// 每5分钟同步一次使用量
	_, err := scheduler.AddFunc("*/5 * * * *", func() {
		log.Println("[Scheduler] 开始执行定时同步任务")
		SyncAllAccountsUsage(context.Background())
	})
	if err != nil {
		log.Printf("[Scheduler] 添加同步任务失败: %v", err)
	}

	scheduler.Start()
	log.Println("[Scheduler] 定时任务调度器已启动")
}

// StopScheduler 停止定时任务调度器
func StopScheduler() {
	if scheduler != nil {
		scheduler.Stop()
		log.Println("[Scheduler] 定时任务调度器已停止")
	}
}
