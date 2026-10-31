package store

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SetCopiedFileExpiration preserves the source deadline exactly in one snapshot,
// including a deadline that elapsed during a remote copy. The service holds its
// expiration operation lock so scheduled cleanup and edits cannot race copying.
// A permanent source removes any stale destination expiration record.
func SetCopiedFileExpiration(accountID, sourceKey, destinationKey string) error {
	if accountID == "" || sourceKey == "" || destinationKey == "" || sourceKey == destinationKey || strings.HasSuffix(sourceKey, "/") || strings.HasSuffix(destinationKey, "/") {
		return ErrInvalidFileExpiration
	}
	dataLock.Lock()
	defer dataLock.Unlock()
	if data == nil {
		return ErrInvalidFileExpiration
	}
	deadline := ""
	for _, expiration := range data.FileExpirations {
		if expiration.AccountID == accountID && expiration.FileKey == sourceKey {
			if _, err := time.Parse(time.RFC3339, expiration.ExpiresAt); err != nil {
				return ErrInvalidFileExpiration
			}
			deadline = expiration.ExpiresAt
			break
		}
	}
	previous := data.FileExpirations
	updated := make([]FileExpiration, 0, len(previous)+1)
	for _, expiration := range previous {
		if expiration.AccountID != accountID || expiration.FileKey != destinationKey {
			updated = append(updated, expiration)
		}
	}
	if deadline != "" {
		updated = append(updated, FileExpiration{ID: uuid.NewString(), AccountID: accountID, FileKey: destinationKey, ExpiresAt: deadline, CreatedAt: NowString()})
	}
	data.FileExpirations = updated
	if err := save(); err != nil {
		data.FileExpirations = previous
		return err
	}
	return nil
}
