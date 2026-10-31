package store

import "github.com/google/uuid"

const maxAuditEntries = 2000

type AuditEntry struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	Username  string `json:"username"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Status    int    `json:"status"`
	CreatedAt string `json:"createdAt"`
}

func AppendAuditEntry(entry *AuditEntry) error {
	if entry == nil {
		return ErrInvalidUser
	}
	dataLock.Lock()
	defer dataLock.Unlock()
	if data == nil {
		return ErrInvalidUser
	}
	result := *entry
	result.ID, result.CreatedAt = uuid.New().String(), NowString()
	previous := data.AuditEntries
	entries := append(append([]AuditEntry(nil), previous...), result)
	if len(entries) > maxAuditEntries {
		entries = entries[len(entries)-maxAuditEntries:]
	}
	data.AuditEntries = entries
	if err := save(); err != nil {
		data.AuditEntries = previous
		return err
	}
	*entry = result
	return nil
}

func GetAuditEntries(limit int) []AuditEntry {
	dataLock.RLock()
	defer dataLock.RUnlock()
	if data == nil {
		return []AuditEntry{}
	}
	if limit <= 0 || limit > maxAuditEntries {
		limit = maxAuditEntries
	}
	if limit > len(data.AuditEntries) {
		limit = len(data.AuditEntries)
	}
	result := make([]AuditEntry, limit)
	for i := range result {
		result[i] = data.AuditEntries[len(data.AuditEntries)-1-i]
	}
	return result
}
