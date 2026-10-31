package store

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SetFilesExpiration updates file deadlines in one persisted snapshot after the
// caller verifies cloud objects and acquires the service expiration lock.
// nil makes selected files permanent. It returns the number of affected records.
func SetFilesExpiration(accountID string, keys []string, expiresAt *string) (int, error) {
	if strings.TrimSpace(accountID) == "" || len(keys) == 0 {
		return 0, ErrInvalidFileExpiration
	}
	selected := make(map[string]bool, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) == "" || strings.HasSuffix(key, "/") {
			return 0, ErrInvalidFileExpiration
		}
		selected[key] = true
	}
	deadline := ""
	if expiresAt != nil {
		parsed, err := time.Parse(time.RFC3339, *expiresAt)
		if err != nil || !parsed.After(time.Now()) {
			return 0, ErrInvalidFileExpiration
		}
		deadline = parsed.UTC().Format(time.RFC3339)
	}
	dataLock.Lock()
	defer dataLock.Unlock()
	if data == nil {
		return 0, ErrInvalidFileExpiration
	}
	previous := data.FileExpirations
	updated := make([]FileExpiration, 0, len(previous)+len(selected))
	affected := make(map[string]bool, len(selected))
	for _, record := range previous {
		if record.AccountID != accountID || !selected[record.FileKey] {
			updated = append(updated, record)
			continue
		}
		affected[record.FileKey] = true
		if expiresAt != nil {
			record.ExpiresAt = deadline
			updated = append(updated, record)
		}
	}
	if expiresAt != nil {
		now := NowString()
		for key := range selected {
			if !affected[key] {
				updated = append(updated, FileExpiration{ID: uuid.New().String(), AccountID: accountID, FileKey: key, ExpiresAt: deadline, CreatedAt: now})
				affected[key] = true
			}
		}
	}
	if len(affected) == 0 {
		return 0, nil
	}
	data.FileExpirations = updated
	if err := save(); err != nil {
		data.FileExpirations = previous
		return 0, err
	}
	return len(affected), nil
}
