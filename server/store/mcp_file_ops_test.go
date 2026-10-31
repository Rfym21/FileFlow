package store

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestCopiedFileExpirationPreservesDeadlineAndRollsBack(t *testing.T) {
	backend := useExpirationTestData(t, nil)
	deadline := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	data.FileExpirations = []FileExpiration{
		{ID: "source-expiry", AccountID: "account", FileKey: "source.txt", ExpiresAt: deadline},
		{ID: "destination-expiry", AccountID: "account", FileKey: "copy.txt", ExpiresAt: "2030-01-01T00:00:00Z"},
		{ID: "other", AccountID: "other-account", FileKey: "copy.txt", ExpiresAt: "2030-01-01T00:00:00Z"},
	}
	before := append([]FileExpiration(nil), data.FileExpirations...)
	backend.saveError = errors.New("disk full")
	if err := SetCopiedFileExpiration("account", "source.txt", "copy.txt"); err == nil || !reflect.DeepEqual(data.FileExpirations, before) {
		t.Fatal("failed expiration copy changed persisted memory")
	}
	backend.saveError = nil
	if err := SetCopiedFileExpiration("account", "source.txt", "copy.txt"); err != nil {
		t.Fatal(err)
	}
	if len(data.FileExpirations) != 3 {
		t.Fatal("expiration copy duplicated records")
	}
	found := false
	for _, expiration := range data.FileExpirations {
		if expiration.AccountID == "account" && expiration.FileKey == "copy.txt" {
			found = expiration.ExpiresAt == deadline
		}
	}
	if !found {
		t.Fatal("deadline elapsed during copy was not preserved exactly")
	}
	if err := SetCopiedFileExpiration("account", "permanent.txt", "copy.txt"); err != nil {
		t.Fatal(err)
	}
	if len(data.FileExpirations) != 2 {
		t.Fatal("permanent source did not remove only destination schedule")
	}
	for _, expiration := range data.FileExpirations {
		if expiration.AccountID == "account" && expiration.FileKey == "copy.txt" {
			t.Fatal("copied permanent file retained destination deadline")
		}
	}
}
