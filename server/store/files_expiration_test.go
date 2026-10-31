package store

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSetFilesExpirationAtomicAndPermanent(t *testing.T) {
	b := useExpirationTestData(t, []FileExpiration{{ID: "old", AccountID: "storage", FileKey: "old.txt", CreatedAt: "created", ExpiresAt: "2030-01-01T00:00:00Z"}})
	deadline := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	initial := append([]FileExpiration(nil), data.FileExpirations...)
	if _, err := SetFilesExpiration("storage", []string{"old.txt", "folder/"}, &deadline); !errors.Is(err, ErrInvalidFileExpiration) || !reflect.DeepEqual(data.FileExpirations, initial) {
		t.Fatal("invalid batch partially changed deadline")
	}
	b.saveError = errors.New("disk failure")
	if _, err := SetFilesExpiration("storage", []string{"old.txt", "new.txt"}, &deadline); err == nil || !reflect.DeepEqual(data.FileExpirations, initial) {
		t.Fatal("failed save modified deadline batch")
	}
	b.saveError = nil
	count, err := SetFilesExpiration("storage", []string{"old.txt", "new.txt", "new.txt"}, &deadline)
	if err != nil || count != 2 || len(data.FileExpirations) != 2 || data.FileExpirations[0].CreatedAt != "created" {
		t.Fatal("deadline batch did not preserve identity or deduplicate")
	}
	count, err = SetFilesExpiration("storage", []string{"old.txt", "new.txt"}, nil)
	if err != nil || count != 2 || len(data.FileExpirations) != 0 {
		t.Fatal("permanent batch left expiration records")
	}
}
