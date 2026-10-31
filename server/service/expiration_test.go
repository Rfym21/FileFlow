package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"fileflow/server/config"
	"fileflow/server/store"
)

func TestExpiredSnapshotDoesNotDeletePostponedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FILEFLOW_ADMIN_PASSWORD", "test-password")
	t.Setenv("FILEFLOW_JWT_SECRET", "test-secret")
	t.Setenv("FILEFLOW_DATA_DIR", dir)
	t.Setenv("FILEFLOW_DATABASE_URL", "sqlite:"+filepath.Join(dir, "test.db"))
	cfg := config.Get()
	previous := *cfg
	cfg.DataDir = dir
	cfg.DatabaseURL = "sqlite:" + filepath.Join(dir, "test.db")
	t.Cleanup(func() { store.Close(); *cfg = previous })
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	record := &store.FileExpiration{AccountID: "no-account", FileKey: "do-not-delete.jpg", ExpiresAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}
	if err := store.CreateFileExpiration(record); err != nil {
		t.Fatal(err)
	}
	snapshot := store.GetExpiredFiles()
	if len(snapshot) != 1 {
		t.Fatal("expected one expired record in the scan")
	}
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := UpdateFileExpirations([]string{record.ID}, future); err != nil {
		t.Fatal(err)
	}
	deleted, err := deleteExpiredFile(context.Background(), snapshot[0].ID)
	if err != nil || deleted {
		t.Fatalf("postponed file deleted = %v, error = %v", deleted, err)
	}
	current, err := store.GetFileExpirationByID(record.ID)
	if err != nil || current.ExpiresAt != future {
		t.Fatalf("postponed record missing or changed: %#v, %v", current, err)
	}
}
