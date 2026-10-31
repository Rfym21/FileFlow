package service

import (
	"testing"
)

func TestSchedulerReloadPreservesDirectUploadCleanup(t *testing.T) {
	setupPreviewAccount(t, "https://example.invalid")
	StartScheduler()
	defer StopScheduler()
	before := len(scheduler.Entries())
	if before < 3 {
		t.Fatal("initial scheduler lacks upload cleanup")
	}
	ReloadScheduler()
	if len(scheduler.Entries()) != before {
		t.Fatal("settings reload discarded direct upload cleanup")
	}
}
