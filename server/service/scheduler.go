package service

import (
	"context"
	"fmt"
	"log"
	"sync"

	"fileflow/server/store"

	"github.com/robfig/cron/v3"
)

var (
	scheduler     *cron.Cron
	schedulerLock sync.Mutex
)

// StartScheduler 启动定时任务调度器
func StartScheduler() {
	schedulerLock.Lock()
	defer schedulerLock.Unlock()

	settings := store.GetSettings()
	scheduler = cron.New()

	// 同步使用量任务，将分钟数转换为 cron 表达式
	interval := settings.SyncInterval
	if interval <= 0 {
		interval = 5
	}
	cronExpr := fmt.Sprintf("*/%d * * * *", interval)

	_, err := scheduler.AddFunc(cronExpr, func() {
		log.Println("[Scheduler] 开始执行定时同步任务")
		SyncAllAccountsUsage(context.Background())
	})
	if err != nil {
		log.Printf("[Scheduler] 添加同步任务失败: %v", err)
	}

	scheduler.Start()
	log.Printf("[Scheduler] 定时任务调度器已启动 (间隔: %d 分钟)", interval)
}

// StopScheduler 停止定时任务调度器
func StopScheduler() {
	schedulerLock.Lock()
	defer schedulerLock.Unlock()

	if scheduler != nil {
		scheduler.Stop()
		log.Println("[Scheduler] 定时任务调度器已停止")
	}
}

// ReloadScheduler 重载定时任务调度器
func ReloadScheduler() {
	schedulerLock.Lock()
	defer schedulerLock.Unlock()

	// 停止现有调度器
	if scheduler != nil {
		scheduler.Stop()
	}

	// 重新创建调度器
	settings := store.GetSettings()
	scheduler = cron.New()

	interval := settings.SyncInterval
	if interval <= 0 {
		interval = 5
	}
	cronExpr := fmt.Sprintf("*/%d * * * *", interval)

	_, err := scheduler.AddFunc(cronExpr, func() {
		log.Println("[Scheduler] 开始执行定时同步任务")
		SyncAllAccountsUsage(context.Background())
	})
	if err != nil {
		log.Printf("[Scheduler] 添加同步任务失败: %v", err)
		return
	}

	scheduler.Start()
	log.Printf("[Scheduler] 定时任务调度器已重载 (间隔: %d 分钟)", interval)
}
