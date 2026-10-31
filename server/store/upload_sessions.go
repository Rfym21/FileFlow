package store

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	UploadStatusPending                 = "pending"
	UploadStatusCompleting              = "completing"
	UploadStatusCompleted               = "completed"
	DirectUploadReservedClassAOps int64 = 1
	DirectUploadReservedClassBOps int64 = 1
	MaxDirectUploadFileSize       int64 = 5 * 1024 * 1024 * 1024
	MaxPendingUploadsPerUser            = 64
	MaxPendingUploads                   = 1024
)

var (
	ErrUploadSessionNotFound  = errors.New("直传任务不存在")
	ErrInvalidUploadSession   = errors.New("直传任务参数无效")
	ErrUploadSessionExpired   = errors.New("直传任务已过期")
	ErrUploadSessionConflict  = errors.New("直传任务状态冲突")
	ErrUploadSessionForbidden = errors.New("无权使用该直传任务")
	ErrUploadQuotaExceeded    = errors.New("剩余存储容量或操作额度不足")
	ErrUploadSessionLimit     = errors.New("进行中的直传任务过多，请稍后重试")
)

// UploadSession contains metadata only. File bodies never enter the store.
type UploadSession struct {
	ID                string `json:"id"`
	UserID            string `json:"userId"`
	AuthType          string `json:"authType"`
	TokenID           string `json:"tokenId,omitempty"`
	AuthVersion       int    `json:"authVersion"`
	AccountID         string `json:"accountId"`
	StagingKey        string `json:"stagingKey"`
	FileKey           string `json:"fileKey"`
	Filename          string `json:"filename"`
	ExpectedSize      int64  `json:"expectedSize"`
	RequestID         string `json:"requestId,omitempty"`
	RequestedPath     string `json:"requestedPath,omitempty"`
	RandomFilename    bool   `json:"randomFilename"`
	ReservedClassAOps int64  `json:"reservedClassAOps"`
	ConsumedClassAOps int64  `json:"consumedClassAOps"`
	ReservedClassBOps int64  `json:"reservedClassBOps"`
	ConsumedClassBOps int64  `json:"consumedClassBOps"`
	ContentType       string `json:"contentType"`
	ExpirationDays    int    `json:"expirationDays"`
	CreatedAt         string `json:"createdAt"`
	UploadExpiresAt   string `json:"uploadExpiresAt"`
	ExpiresAt         string `json:"expiresAt"`
	Status            string `json:"status"`
	ETag              string `json:"etag,omitempty"`
	CompletedAt       string `json:"completedAt,omitempty"`
}

func validateStoredUploadSession(session UploadSession) error {
	if session.ID == "" || session.UserID == "" || session.AccountID == "" || session.AuthVersion < 1 ||
		session.FileKey == "" || (session.StagingKey != "" && session.StagingKey == session.FileKey) ||
		len(session.StagingKey) > 1024 || len(session.FileKey) > 1024 || len(session.Filename) > 1024 ||
		session.ExpectedSize < 0 || session.ExpectedSize > MaxDirectUploadFileSize ||
		session.ContentType == "" || len(session.ContentType) > 256 || strings.ContainsAny(session.ContentType, "\r\n") ||
		session.ExpirationDays < 0 || session.ExpirationDays > 36500 {
		return ErrInvalidUploadSession
	}
	if session.ReservedClassAOps < 0 || session.ReservedClassAOps > 100 || session.ConsumedClassAOps < 0 || session.ConsumedClassAOps > session.ReservedClassAOps || session.ReservedClassBOps < 0 || session.ReservedClassBOps > 100 || session.ConsumedClassBOps < 0 || len(session.RequestID) > 128 {
		return ErrInvalidUploadSession
	}
	if (session.AuthType != "jwt" && session.AuthType != "api_token") ||
		(session.AuthType == "jwt" && session.TokenID != "") || (session.AuthType == "api_token" && session.TokenID == "") {
		return ErrInvalidUploadSession
	}
	created, err := time.Parse(time.RFC3339, session.CreatedAt)
	if err != nil {
		return ErrInvalidUploadSession
	}
	expires, err := time.Parse(time.RFC3339, session.ExpiresAt)
	if err != nil || !expires.After(created) {
		return ErrInvalidUploadSession
	}
	uploadExpires, err := time.Parse(time.RFC3339, session.UploadExpiresAt)
	if err != nil || !uploadExpires.After(created) || uploadExpires.After(expires) {
		return ErrInvalidUploadSession
	}
	switch session.Status {
	case UploadStatusPending:
		if session.ETag != "" || session.CompletedAt != "" {
			return ErrInvalidUploadSession
		}
	case UploadStatusCompleting:
		if session.ETag == "" || session.CompletedAt != "" {
			return ErrInvalidUploadSession
		}
	case UploadStatusCompleted:
		if session.ETag == "" {
			return ErrInvalidUploadSession
		}
		if _, err := time.Parse(time.RFC3339, session.CompletedAt); err != nil {
			return ErrInvalidUploadSession
		}
	default:
		return ErrInvalidUploadSession
	}
	return nil
}

func uploadSessionIndex(id string) int {
	if data != nil {
		for i := range data.UploadSessions {
			if data.UploadSessions[i].ID == id {
				return i
			}
		}
	}
	return -1
}

func uploadSessionPendingAt(session UploadSession, now time.Time) bool {
	if session.Status != UploadStatusPending && session.Status != UploadStatusCompleting {
		return false
	}
	expires, err := time.Parse(time.RFC3339, session.ExpiresAt)
	return err == nil && expires.After(now)
}

func uploadSessionUncompleted(session UploadSession) bool {
	return session.Status == UploadStatusPending || session.Status == UploadStatusCompleting
}

func uploadReservationsLocked(accountID string, now time.Time) (size, operations int64, count int) {
	if data == nil {
		return
	}
	for _, session := range data.UploadSessions {
		if session.AccountID == accountID && uploadSessionUncompleted(session) {
			size += session.ExpectedSize
			operations += max(session.ReservedClassAOps-session.ConsumedClassAOps, 0)
			count++
		}
	}
	return
}

func GetUploadReservations(accountID string) (size, operations int64, count int) {
	dataLock.RLock()
	defer dataLock.RUnlock()
	return uploadReservationsLocked(accountID, time.Now())
}

func uploadReadReservationsLocked(accountID string, now time.Time) int64 {
	var operations int64
	if data == nil {
		return 0
	}
	for _, session := range data.UploadSessions {
		if session.AccountID == accountID && uploadSessionUncompleted(session) {
			operations += max(session.ReservedClassBOps-session.ConsumedClassBOps, 0)
		}
	}
	return operations
}

func GetUploadReadReservations(accountID string) int64 {
	dataLock.RLock()
	defer dataLock.RUnlock()
	return uploadReadReservationsLocked(accountID, time.Now())
}

// uploadSessionAuthority checks current rights, including token revocation, while locked.
func uploadSessionAuthority(session UploadSession) (*Account, error) {
	userIndex := userIndexByID(session.UserID)
	if userIndex < 0 {
		return nil, ErrUploadSessionForbidden
	}
	user := data.Users[userIndex]
	if !user.IsActive || !user.HasPermission("write") || user.AuthVersion != session.AuthVersion {
		return nil, ErrUploadSessionForbidden
	}
	if session.AuthType == "api_token" {
		allowed := false
		for _, token := range data.Tokens {
			if token.ID == session.TokenID && token.UserID == session.UserID && token.HasPermission("write") {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, ErrUploadSessionForbidden
		}
	}
	for i := range data.Accounts {
		account := &data.Accounts[i]
		if account.ID != session.AccountID {
			continue
		}
		if !account.IsActive || (session.AuthType == "jwt" && !account.CanClientUpload()) || (session.AuthType == "api_token" && !account.CanAPIUpload()) {
			return nil, ErrUploadSessionForbidden
		}
		return account, nil
	}
	return nil, ErrUploadSessionForbidden
}

func uploadCapacityAvailable(account Account, size, operations int64) bool {
	return size >= 0 && operations >= 0 && account.Quota.MaxSizeBytes > 0 && account.Usage.SizeBytes >= 0 &&
		account.Usage.SizeBytes <= account.Quota.MaxSizeBytes && size <= account.Quota.MaxSizeBytes-account.Usage.SizeBytes &&
		account.Quota.MaxClassAOps > 0 && account.Usage.ClassAOps >= 0 && account.Usage.ClassAOps <= account.Quota.MaxClassAOps &&
		operations <= account.Quota.MaxClassAOps-account.Usage.ClassAOps
}

func CreateUploadSession(session *UploadSession) error {
	if session == nil {
		return ErrInvalidUploadSession
	}
	result := *session
	if result.ID == "" {
		result.ID = uuid.New().String()
	}
	if result.CreatedAt == "" {
		result.CreatedAt = NowString()
	}
	if result.Status == "" {
		result.Status = UploadStatusPending
	}
	if result.ReservedClassAOps == 0 {
		result.ReservedClassAOps = DirectUploadReservedClassAOps
	}
	if result.ReservedClassBOps == 0 {
		result.ReservedClassBOps = DirectUploadReservedClassBOps
	}
	if result.Status != UploadStatusPending || validateStoredUploadSession(result) != nil {
		return ErrInvalidUploadSession
	}
	now := time.Now()
	expires, _ := time.Parse(time.RFC3339, result.ExpiresAt)
	uploadExpires, _ := time.Parse(time.RFC3339, result.UploadExpiresAt)
	if !expires.After(now) || !uploadExpires.After(now) || expires.After(now.Add(24*time.Hour)) {
		return ErrUploadSessionExpired
	}
	dataLock.Lock()
	defer dataLock.Unlock()
	if data == nil {
		return ErrInvalidUploadSession
	}
	account, err := uploadSessionAuthority(result)
	if err != nil {
		return err
	}
	perUser, total := 0, 0
	for _, existing := range data.UploadSessions {
		if existing.ID == result.ID || (result.RequestID != "" && existing.UserID == result.UserID && existing.AuthType == result.AuthType && existing.TokenID == result.TokenID && existing.RequestID == result.RequestID) || (existing.AccountID == result.AccountID && ((result.StagingKey != "" && existing.StagingKey == result.StagingKey) || (existing.FileKey == result.FileKey && uploadSessionUncompleted(existing)))) {
			return ErrUploadSessionConflict
		}
		if uploadSessionUncompleted(existing) {
			total++
			if existing.UserID == result.UserID {
				perUser++
			}
		}
	}
	if perUser >= MaxPendingUploadsPerUser || total >= MaxPendingUploads {
		return ErrUploadSessionLimit
	}
	size, operations, _ := uploadReservationsLocked(result.AccountID, now)
	view := operationUsageView(*account, now)
	if !uploadCapacityAvailable(view, size+result.ExpectedSize, operations+result.ReservedClassAOps) || !operationCapacityAvailable(view, operations+result.ReservedClassAOps, uploadReadReservationsLocked(result.AccountID, now)+result.ReservedClassBOps) {
		return ErrUploadQuotaExceeded
	}
	previous, previousAccount, previousLedger := data.UploadSessions, *account, data.OperationUsage
	result.ConsumedClassAOps = result.ReservedClassAOps
	account.Usage.ClassAOps = view.Usage.ClassAOps + result.ReservedClassAOps
	account.Usage.ClassBOps = view.Usage.ClassBOps
	account.UpdatedAt = NowString()
	recordOperationUsage(*account, now)
	data.UploadSessions = append(append([]UploadSession(nil), previous...), result)
	if err := save(); err != nil {
		data.UploadSessions = previous
		*account = previousAccount
		data.OperationUsage = previousLedger
		return err
	}
	*session = result
	return nil
}

func GetUploadSessionByID(id string) (*UploadSession, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()
	if i := uploadSessionIndex(id); i >= 0 {
		result := data.UploadSessions[i]
		return &result, nil
	}
	return nil, ErrUploadSessionNotFound
}

func ListUploadSessions() []UploadSession {
	dataLock.RLock()
	defer dataLock.RUnlock()
	if data == nil {
		return []UploadSession{}
	}
	return append([]UploadSession{}, data.UploadSessions...)
}

func UpdateUploadSessionState(id, status, etag string) error {
	if status != UploadStatusCompleting || etag == "" || len(etag) > 256 || strings.ContainsAny(etag, "\r\n") {
		return ErrInvalidUploadSession
	}
	dataLock.Lock()
	defer dataLock.Unlock()
	i := uploadSessionIndex(id)
	if i < 0 {
		return ErrUploadSessionNotFound
	}
	previous := data.UploadSessions[i]
	if !uploadSessionPendingAt(previous, time.Now()) {
		return ErrUploadSessionExpired
	}
	account, err := uploadSessionAuthority(previous)
	if err != nil {
		return err
	}
	reservedSize, reservedOperations, _ := uploadReservationsLocked(previous.AccountID, time.Now())
	view := operationUsageView(*account, time.Now())
	if !uploadCapacityAvailable(view, reservedSize, reservedOperations) || !operationCapacityAvailable(view, reservedOperations, uploadReadReservationsLocked(previous.AccountID, time.Now())) {
		return ErrUploadQuotaExceeded
	}
	if previous.Status == UploadStatusCompleting {
		if previous.ETag == etag {
			return nil
		}
		return ErrUploadSessionConflict
	}
	if previous.Status != UploadStatusPending {
		return ErrUploadSessionConflict
	}
	data.UploadSessions[i].Status, data.UploadSessions[i].ETag = status, etag
	if err := save(); err != nil {
		data.UploadSessions[i] = previous
		return err
	}
	return nil
}

// FinalizeUploadSession commits usage and expiration in the same persisted snapshot.
// Repeated completion does not charge usage twice or postpone the file expiration.
func FinalizeUploadSession(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()
	i := uploadSessionIndex(id)
	if i < 0 {
		return ErrUploadSessionNotFound
	}
	previousSession := data.UploadSessions[i]
	if previousSession.Status == UploadStatusCompleted {
		return nil
	}
	if previousSession.Status != UploadStatusCompleting || previousSession.ETag == "" {
		return ErrUploadSessionConflict
	}
	if !uploadSessionPendingAt(previousSession, time.Now()) {
		return ErrUploadSessionExpired
	}
	account, err := uploadSessionAuthority(previousSession)
	if err != nil {
		return err
	}
	reservedSize, reservedOperations, _ := uploadReservationsLocked(previousSession.AccountID, time.Now())
	view := operationUsageView(*account, time.Now())
	if !uploadCapacityAvailable(view, reservedSize, reservedOperations) || !operationCapacityAvailable(view, reservedOperations, uploadReadReservationsLocked(previousSession.AccountID, time.Now())) {
		return ErrUploadQuotaExceeded
	}
	previousAccount, previousExpirations, previousOperationUsage := *account, data.FileExpirations, data.OperationUsage
	now := time.Now().UTC()
	data.UploadSessions[i].Status = UploadStatusCompleted
	data.UploadSessions[i].CompletedAt = now.Format(time.RFC3339)
	account.Usage.SizeBytes += previousSession.ExpectedSize
	account.UsageRevision++
	account.Usage.ClassAOps = view.Usage.ClassAOps + max(previousSession.ReservedClassAOps-previousSession.ConsumedClassAOps, 0)
	account.Usage.ClassBOps = view.Usage.ClassBOps + max(previousSession.ReservedClassBOps-previousSession.ConsumedClassBOps, 0)
	recordOperationUsage(*account, now)
	account.UpdatedAt = now.Format(time.RFC3339)
	if previousSession.ExpirationDays > 0 {
		expiration := FileExpiration{ID: uuid.New().String(), AccountID: previousSession.AccountID, FileKey: previousSession.FileKey,
			ExpiresAt: now.AddDate(0, 0, previousSession.ExpirationDays).Format(time.RFC3339), CreatedAt: now.Format(time.RFC3339)}
		updated := append([]FileExpiration(nil), previousExpirations...)
		replaced := false
		for j := range updated {
			if updated[j].AccountID == expiration.AccountID && updated[j].FileKey == expiration.FileKey {
				updated[j] = expiration
				replaced = true
				break
			}
		}
		if !replaced {
			updated = append(updated, expiration)
		}
		data.FileExpirations = updated
	}
	if err := save(); err != nil {
		data.UploadSessions[i] = previousSession
		*account = previousAccount
		data.FileExpirations = previousExpirations
		data.OperationUsage = previousOperationUsage
		return err
	}
	return nil
}

func DeleteUploadSession(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()
	i := uploadSessionIndex(id)
	if i < 0 {
		return ErrUploadSessionNotFound
	}
	previous := data.UploadSessions
	updated := append([]UploadSession(nil), previous[:i]...)
	data.UploadSessions = append(updated, previous[i+1:]...)
	if err := save(); err != nil {
		data.UploadSessions = previous
		return err
	}
	return nil
}

func GetExpiredUploadSessions(now time.Time) []UploadSession {
	result := make([]UploadSession, 0)
	for _, session := range ListUploadSessions() {
		expires, err := time.Parse(time.RFC3339, session.ExpiresAt)
		if err == nil && !expires.After(now) {
			result = append(result, session)
		}
	}
	return result
}
