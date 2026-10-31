package store

import (
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func useUploadTestData(t *testing.T) *expirationTestBackend {
	b := useExpirationTestData(t, nil)
	data.Users = []User{{ID: "writer", Username: "writer", PasswordHash: "test-hash", Role: RoleMember, IsActive: true, AuthVersion: 1}}
	data.Accounts = []Account{{ID: "storage", Name: "shared", IsActive: true, Permissions: DefaultAccountPermissions(), Quota: Quota{MaxSizeBytes: 1000, MaxClassAOps: 20, MaxClassBOps: 20}}}
	return b
}

func sampleUploadSession() UploadSession {
	id := uuid.New().String()
	now := time.Now().UTC()
	return UploadSession{ID: id, UserID: "writer", AuthType: "jwt", AuthVersion: data.Users[0].AuthVersion, AccountID: "storage", FileKey: "files/" + id + ".txt", Filename: "file.txt", ExpectedSize: 100, ContentType: "text/plain", ExpirationDays: 7, CreatedAt: now.Format(time.RFC3339), UploadExpiresAt: now.Add(15 * time.Minute).Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339), RequestID: id, RequestedPath: "files", RandomFilename: true}
}

func TestUploadContentDigestValidationAndTeamStatePersistence(t *testing.T) {
	useUploadTestData(t)
	for _, digest := range []string{"a", strings.Repeat("g", 64), strings.Repeat("a", 65)} {
		session := sampleUploadSession()
		session.ContentSHA256 = digest
		if err := CreateUploadSession(&session); !errors.Is(err, ErrInvalidUploadSession) {
			t.Fatalf("invalid content digest accepted: %q %v", digest, err)
		}
	}
	session := sampleUploadSession()
	session.ContentSHA256 = strings.Repeat("ab", 32)
	if err := CreateUploadSession(&session); err != nil {
		t.Fatal(err)
	}
	encoded, err := marshalTeamState(data)
	if err != nil {
		t.Fatal(err)
	}
	loaded := Data{Accounts: append([]Account(nil), data.Accounts...)}
	if err := applyTeamState(&loaded, encoded); err != nil || len(loaded.UploadSessions) != 1 || loaded.UploadSessions[0].ContentSHA256 != session.ContentSHA256 {
		t.Fatalf("content digest lost across team-state persistence: %v", err)
	}
}

func snapshotUploadData() Data {
	result := *data
	result.Accounts = append([]Account(nil), data.Accounts...)
	result.UploadSessions = append([]UploadSession(nil), data.UploadSessions...)
	result.FileExpirations = append([]FileExpiration(nil), data.FileExpirations...)
	result.OperationUsage = make(map[string]AccountOperationUsage)
	for id, usage := range data.OperationUsage {
		result.OperationUsage[id] = usage
	}
	if data.OperationUsage == nil {
		result.OperationUsage = nil
	}
	return result
}

func TestUploadFinalizationConsumesReservationsOnce(t *testing.T) {
	b := useUploadTestData(t)
	if err := ReserveAccountOperations("storage", 0, 1); err != nil {
		t.Fatal(err)
	}
	session := sampleUploadSession()
	if err := CreateUploadSession(&session); err != nil {
		t.Fatal(err)
	}
	bytes, operations, count := GetUploadReservations("storage")
	if bytes != 100 || operations != 0 || count != 1 || GetUploadReadReservations("storage") != 1 || data.Accounts[0].Usage.ClassAOps != 1 {
		t.Fatal("authorization did not charge PUT once and retain file/HEAD reservation")
	}
	if err := ConsumeUploadSessionRead(session.ID); err != nil {
		t.Fatal(err)
	}
	if err := UpdateUploadSessionState(session.ID, UploadStatusCompleting, "\"etag\""); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeUploadSession(session.ID); err != nil {
		t.Fatal(err)
	}
	account := data.Accounts[0]
	if account.Usage.SizeBytes != 100 || account.Usage.ClassAOps != 1 || account.Usage.ClassBOps != 2 || account.UsageRevision != 1 || len(data.FileExpirations) != 1 {
		t.Fatalf("completion accounting: %#v", account.Usage)
	}
	before, saves := snapshotUploadData(), b.saveCount
	if err := FinalizeUploadSession(session.ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, *data) || b.saveCount != saves {
		t.Fatal("duplicate completion charged usage or extended expiration")
	}
	if bytes, _, count := GetUploadReservations("storage"); bytes != 0 || count != 0 {
		t.Fatal("completed task retains capacity")
	}
}

func TestUploadExpiredReservationAndAuthorizationAreNotRecycled(t *testing.T) {
	useUploadTestData(t)
	data.Accounts[0].Quota.MaxSizeBytes = 100
	data.Accounts[0].Quota.MaxClassAOps = 1
	session := sampleUploadSession()
	if err := CreateUploadSession(&session); err != nil {
		t.Fatal(err)
	}
	data.UploadSessions[0].ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	if bytes, _, count := GetUploadReservations("storage"); bytes != 100 || count != 1 {
		t.Fatal("unverified expired object reservation was released")
	}
	data.Users[0].IsActive = false
	if err := ConsumeUploadSessionRead(session.ID); !errors.Is(err, ErrUploadSessionExpired) {
		t.Fatalf("expired task allowed user completion: %v", err)
	}
	if err := ConsumeUploadSessionCleanupRead(session.ID); err != nil {
		t.Fatalf("disabled member stranded cleanup: %v", err)
	}
	if err := DeleteUploadSession(session.ID); err != nil {
		t.Fatal(err)
	}
	if data.Accounts[0].Usage.ClassAOps != 1 || data.Accounts[0].Usage.ClassBOps != 1 {
		t.Fatal("cleaning an unconfirmed object refunded authorized operations")
	}
	data.Users[0].IsActive = true
	retry := sampleUploadSession()
	if err := CreateUploadSession(&retry); !errors.Is(err, ErrUploadQuotaExceeded) {
		t.Fatalf("unconfirmed uploads bypass monthly authorization quota: %v", err)
	}
}

func TestUploadQuotaAndCredentialBinding(t *testing.T) {
	useUploadTestData(t)
	data.Accounts[0].Quota.MaxSizeBytes = 150
	first := sampleUploadSession()
	if err := CreateUploadSession(&first); err != nil {
		t.Fatal(err)
	}
	second := sampleUploadSession()
	if err := CreateUploadSession(&second); !errors.Is(err, ErrUploadQuotaExceeded) {
		t.Fatalf("concurrent reservation overfilled quota: %v", err)
	}
	second = sampleUploadSession()
	second.AuthType, second.TokenID = "api_token", "missing"
	if err := CreateUploadSession(&second); !errors.Is(err, ErrUploadSessionForbidden) {
		t.Fatal("revoked API token created an upload")
	}
	data.Users[0].AuthVersion++
	if err := UpdateUploadSessionState(first.ID, UploadStatusCompleting, "etag"); !errors.Is(err, ErrUploadSessionForbidden) {
		t.Fatalf("changed identity used an old task: %v", err)
	}
}

func TestUploadPersistenceFailuresRestoreAllAccounting(t *testing.T) {
	b := useUploadTestData(t)
	session := sampleUploadSession()
	before := snapshotUploadData()
	b.saveError = errors.New("disk failure")
	if err := CreateUploadSession(&session); err == nil || !reflect.DeepEqual(before, *data) {
		t.Fatal("failed creation modified task or quota")
	}
	b.saveError = nil
	if err := CreateUploadSession(&session); err != nil {
		t.Fatal(err)
	}
	before = snapshotUploadData()
	b.saveError = errors.New("disk failure")
	if err := ConsumeUploadSessionRead(session.ID); err == nil || !reflect.DeepEqual(before, *data) {
		t.Fatal("failed read authorization consumed reserved quota")
	}
	if err := DeleteUploadSession(session.ID); err == nil || !reflect.DeepEqual(before, *data) {
		t.Fatal("failed removal lost a live reservation")
	}
	b.saveError = nil
	if err := ConsumeUploadSessionRead(session.ID); err != nil {
		t.Fatal(err)
	}
	if err := UpdateUploadSessionState(session.ID, UploadStatusCompleting, "etag"); err != nil {
		t.Fatal(err)
	}
	before = snapshotUploadData()
	b.saveError = errors.New("disk failure")
	if err := FinalizeUploadSession(session.ID); err == nil || !reflect.DeepEqual(before, *data) {
		t.Fatal("failed completion changed file size, usage, or expiration")
	}
}

func TestOperationQuotaIsAtomicAndSurvivesSync(t *testing.T) {
	useUploadTestData(t)
	data.Accounts[0].Quota.MaxClassAOps, data.Accounts[0].Quota.MaxClassBOps = 10, 10
	var successes atomic.Int64
	var workers sync.WaitGroup
	for i := 0; i < 100; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if ReserveAccountOperations("storage", 1, 1) == nil {
				successes.Add(1)
			}
		}()
	}
	workers.Wait()
	if successes.Load() != 10 || data.Accounts[0].Usage.ClassAOps != 10 || data.Accounts[0].Usage.ClassBOps != 10 {
		t.Fatal("parallel operations exceeded configured quota")
	}
	if err := UpdateAccountUsage("storage", Usage{SizeBytes: 0, ClassAOps: 1, ClassBOps: 1}); err != nil {
		t.Fatal(err)
	}
	if data.Accounts[0].Usage.ClassAOps != 10 || data.Accounts[0].Usage.ClassBOps != 10 {
		t.Fatal("delayed sync erased authorized counters")
	}
	if err := ReserveAccountOperations("storage", 0, math.MaxInt64); !errors.Is(err, ErrUploadQuotaExceeded) {
		t.Fatal("oversized request counter overflowed quota")
	}
}

func TestUploadReadRetriesConsumeAdditionalQuota(t *testing.T) {
	useUploadTestData(t)
	data.Accounts[0].Quota.MaxClassBOps = 2
	session := sampleUploadSession()
	if err := CreateUploadSession(&session); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := ConsumeUploadSessionRead(session.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := ConsumeUploadSessionRead(session.ID); !errors.Is(err, ErrUploadQuotaExceeded) {
		t.Fatal("HEAD retry bypassed the read limit")
	}
}

func TestPendingUploadLimitsIncludeUncleanedExpiredTasks(t *testing.T) {
	useUploadTestData(t)
	data.Accounts[0].Quota.MaxSizeBytes = 100000
	data.Accounts[0].Quota.MaxClassAOps = 10000
	data.Accounts[0].Quota.MaxClassBOps = 10000
	for i := 0; i < MaxPendingUploadsPerUser; i++ {
		session := sampleUploadSession()
		if err := CreateUploadSession(&session); err != nil {
			t.Fatal(err)
		}
		data.UploadSessions[len(data.UploadSessions)-1].ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	}
	session := sampleUploadSession()
	if err := CreateUploadSession(&session); !errors.Is(err, ErrUploadSessionLimit) {
		t.Fatal("expired but uncleaned objects evaded per-user task limit")
	}
}

func TestOperationLedgerResetsInNewUTCMonth(t *testing.T) {
	useUploadTestData(t)
	data.Accounts[0].Usage.ClassAOps = 20
	data.Accounts[0].Usage.ClassBOps = 20
	now := time.Now().UTC()
	previousMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour).Format("2006-01")
	data.OperationUsage = map[string]AccountOperationUsage{"storage": {Month: previousMonth, ClassAOps: 20, ClassBOps: 20}}
	if err := ReserveAccountOperations("storage", 1, 1); err != nil {
		t.Fatal(err)
	}
	if data.Accounts[0].Usage.ClassAOps != 1 || data.Accounts[0].Usage.ClassBOps != 1 {
		t.Fatal("new monthly authorization budget retained previous month's counters")
	}
}

func TestOldOperationLedgerCannotEraseFreshSyncQuota(t *testing.T) {
	for _, class := range []string{"A", "B"} {
		t.Run(class, func(t *testing.T) {
			useUploadTestData(t)
			now := time.Now().UTC()
			previousMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour).Format("2006-01")
			data.OperationUsage = map[string]AccountOperationUsage{"storage": {Month: previousMonth, ClassAOps: 20, ClassBOps: 20}}
			data.Accounts[0].Usage.LastSyncAt = now.Format(time.RFC3339)
			a, b := int64(0), int64(0)
			if class == "A" {
				data.Accounts[0].Usage.ClassAOps, a = 100, 1
			} else {
				data.Accounts[0].Usage.ClassBOps, b = 100, 1
			}
			before := snapshotUploadData()
			if err := ReserveAccountOperations("storage", a, b); !errors.Is(err, ErrUploadQuotaExceeded) {
				t.Fatalf("fresh current-month %s count bypassed quota: %v", class, err)
			}
			if !reflect.DeepEqual(before, *data) {
				t.Fatal("rejected operation changed current-month sync counters")
			}
		})
	}
}

func TestCurrentMonthSyncRefreshesLedgerAndRollsBackOnFailure(t *testing.T) {
	b := useUploadTestData(t)
	now := time.Now().UTC()
	previousMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour).Format("2006-01")
	data.OperationUsage = map[string]AccountOperationUsage{"storage": {Month: previousMonth, ClassAOps: 20, ClassBOps: 20}}
	if err := UpdateAccountUsageSnapshot("storage", Usage{ClassAOps: 6, ClassBOps: 7}, 0); err != nil {
		t.Fatal(err)
	}
	ledger := data.OperationUsage["storage"]
	if ledger.Month != now.Format("2006-01") || ledger.ClassAOps != 6 || ledger.ClassBOps != 7 {
		t.Fatal("new-month sync did not persist its operation floor")
	}
	if err := ReserveAccountOperations("storage", 1, 1); err != nil {
		t.Fatal(err)
	}
	if data.Accounts[0].Usage.ClassAOps != 7 || data.Accounts[0].Usage.ClassBOps != 8 {
		t.Fatal("authorization discarded synchronized counters")
	}
	before := snapshotUploadData()
	b.saveError = errors.New("disk failure")
	if err := UpdateAccountUsageSnapshot("storage", Usage{ClassAOps: 12, ClassBOps: 13}, 0); err == nil {
		t.Fatal("expected sync persistence failure")
	}
	if !reflect.DeepEqual(before, *data) {
		t.Fatal("failed sync changed operation ledger or account usage")
	}
}

func TestAccountGettersProjectNewMonthWithoutWritingData(t *testing.T) {
	b := useUploadTestData(t)
	now := time.Now().UTC()
	previousMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour)
	data.OperationUsage = map[string]AccountOperationUsage{"storage": {Month: previousMonth.Format("2006-01"), ClassAOps: 20, ClassBOps: 20}}
	data.Accounts[0].Usage = Usage{SizeBytes: 1, ClassAOps: 20, ClassBOps: 20, LastSyncAt: previousMonth.Format(time.RFC3339)}
	before := snapshotUploadData()
	for _, accounts := range [][]Account{GetAccounts(), GetAccountsPaged(1, 10).Items, GetActiveAccounts(), GetAvailableAccounts(), GetAvailableAccountsForAutoUpload(), GetAvailableAccountsForClientUpload()} {
		if len(accounts) != 1 || accounts[0].Usage.ClassAOps != 0 || accounts[0].Usage.ClassBOps != 0 || !accounts[0].IsAvailable() {
			t.Fatal("getter returned last month's exhausted operation quota")
		}
	}
	view, err := GetAccountOperationView("storage")
	if err != nil || view.Usage.ClassAOps != 0 || view.Usage.ClassBOps != 0 {
		t.Fatal("single account view returned stale monthly counts")
	}
	stats := GetAccountsStats()
	if stats.AvailableCount != 1 || stats.TotalWriteOps != 0 || stats.TotalReadOps != 0 {
		t.Fatal("account stats used stale monthly counts")
	}
	if b.saveCount != 0 || !reflect.DeepEqual(before, *data) {
		t.Fatal("read-only quota projection modified persisted accounts")
	}
}

func TestUsageRevisionRejectsStaleSizeAndAccountEdits(t *testing.T) {
	useUploadTestData(t)
	snapshot := data.Accounts[0]
	session := sampleUploadSession()
	if err := CreateUploadSession(&session); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeUploadSessionRead(session.ID); err != nil {
		t.Fatal(err)
	}
	if err := UpdateUploadSessionState(session.ID, UploadStatusCompleting, "etag"); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeUploadSession(session.ID); err != nil {
		t.Fatal(err)
	}
	if err := UpdateAccountUsageSnapshot("storage", Usage{SizeBytes: 0}, snapshot.UsageRevision); err != nil {
		t.Fatal(err)
	}
	if data.Accounts[0].Usage.SizeBytes != 100 || data.Accounts[0].UsageRevision != 1 {
		t.Fatal("stale R2 scan erased confirmed capacity")
	}
	snapshot.Name = "edited"
	if err := UpdateAccount(&snapshot); err != nil {
		t.Fatal(err)
	}
	if data.Accounts[0].Usage.SizeBytes != 100 || data.Accounts[0].UsageRevision != 1 {
		t.Fatal("account settings edit erased accounting")
	}
}

func TestSQLiteUploadSessionMetadataAndAtomicFailure(t *testing.T) {
	useUploadTestData(t)
	b, err := NewSQLiteBackend(filepath.Join(t.TempDir(), "upload.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	backend = b
	session := sampleUploadSession()
	if err := CreateUploadSession(&session); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeUploadSessionRead(session.ID); err != nil {
		t.Fatal(err)
	}
	if err := UpdateUploadSessionState(session.ID, UploadStatusCompleting, "etag"); err != nil {
		t.Fatal(err)
	}
	loaded, err := b.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.UploadSessions, data.UploadSessions) || loaded.Accounts[0].Quota.MaxClassBOps != 20 || !reflect.DeepEqual(loaded.OperationUsage, data.OperationUsage) {
		t.Fatal("SQL did not roundtrip task metadata and read quota")
	}
	if _, err := b.db.Exec("CREATE TRIGGER reject_upload_save BEFORE INSERT ON team_state BEGIN SELECT RAISE(ABORT, 'test failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeUploadSession(session.ID); err == nil {
		t.Fatal("expected storage failure")
	}
	reloaded, err := b.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.UploadSessions, loaded.UploadSessions) || reloaded.Accounts[0].Usage != loaded.Accounts[0].Usage || len(reloaded.FileExpirations) != 0 {
		t.Fatal("failed task commit changed persistent quota or TTL")
	}
}
