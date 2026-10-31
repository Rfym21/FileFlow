package store

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

type expirationTestBackend struct {
	saveCount int
	saveError error
}

func (b *expirationTestBackend) Init() error          { return nil }
func (b *expirationTestBackend) Close() error         { return nil }
func (b *expirationTestBackend) Load() (*Data, error) { return nil, nil }
func (b *expirationTestBackend) Save(*Data) error {
	b.saveCount++
	return b.saveError
}

func useExpirationTestData(t *testing.T, records []FileExpiration) *expirationTestBackend {
	t.Helper()
	previousData, previousBackend := data, backend
	b := &expirationTestBackend{}
	data = &Data{FileExpirations: append([]FileExpiration(nil), records...)}
	backend = b
	t.Cleanup(func() { data, backend = previousData, previousBackend })
	return b
}

func TestUpdateFileExpirationsAtomicAndPersistent(t *testing.T) {
	initial := []FileExpiration{
		{ID: "first", AccountID: "account", FileKey: "a.jpg", CreatedAt: "created", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)},
		{ID: "second", AccountID: "account", FileKey: "b.jpg", CreatedAt: "created", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)},
	}
	b := useExpirationTestData(t, initial)
	future := time.Now().Add(48 * time.Hour).In(time.FixedZone("offset", 8*60*60)).Format(time.RFC3339)

	if _, err := UpdateFileExpirations([]string{"first", "missing"}, future); !errors.Is(err, ErrFileExpirationNotFound) {
		t.Fatalf("missing record error = %v", err)
	}
	if !reflect.DeepEqual(data.FileExpirations, initial) || b.saveCount != 0 {
		t.Fatal("missing record partially changed or saved the batch")
	}
	for _, input := range []struct {
		ids []string
		at  string
	}{{nil, future}, {[]string{" "}, future}, {[]string{"first"}, "invalid"}, {[]string{"first"}, time.Now().Add(-time.Hour).Format(time.RFC3339)}} {
		if _, err := UpdateFileExpirations(input.ids, input.at); !errors.Is(err, ErrInvalidFileExpiration) {
			t.Fatalf("invalid input error = %v", err)
		}
	}
	b.saveError = errors.New("disk failure")
	if _, err := UpdateFileExpirations([]string{"first", "second"}, future); err == nil {
		t.Fatal("expected persistence failure")
	}
	if !reflect.DeepEqual(data.FileExpirations, initial) {
		t.Fatal("persistence failure changed in-memory deadlines")
	}
	b.saveError = nil
	count, err := UpdateFileExpirations([]string{"first", "first", " second "}, future)
	if err != nil || count != 2 || b.saveCount != 2 {
		t.Fatalf("batch result = (%d, %v), saves = %d", count, err, b.saveCount)
	}
	parsed, _ := time.Parse(time.RFC3339, future)
	for i, record := range data.FileExpirations {
		expected := initial[i]
		expected.ExpiresAt = parsed.UTC().Format(time.RFC3339)
		if record != expected {
			t.Fatalf("record changed beyond deadline: %#v", record)
		}
	}
}

func TestDeleteFileExpirationRestoresRecordOnSaveFailure(t *testing.T) {
	initial := []FileExpiration{{ID: "one"}, {ID: "two"}, {ID: "three"}}
	b := useExpirationTestData(t, initial)
	b.saveError = errors.New("disk failure")
	if err := DeleteFileExpirationByID("two"); err == nil {
		t.Fatal("expected persistence failure")
	}
	if !reflect.DeepEqual(data.FileExpirations, initial) {
		t.Fatal("failed deletion lost an in-memory record")
	}
}

func TestGetExpiredFilesComparesActualTimesAcrossOffsets(t *testing.T) {
	now := time.Now()
	useExpirationTestData(t, []FileExpiration{
		{ID: "expired-positive", ExpiresAt: now.Add(-time.Hour).In(time.FixedZone("east", 14*60*60)).Format(time.RFC3339)},
		{ID: "future-negative", ExpiresAt: now.Add(time.Hour).In(time.FixedZone("west", -12*60*60)).Format(time.RFC3339)},
		{ID: "expired-utc", ExpiresAt: now.Add(-time.Hour).UTC().Format(time.RFC3339)},
		{ID: "invalid", ExpiresAt: "invalid"},
	})
	expired := GetExpiredFiles()
	if len(expired) != 2 || expired[0].ID != "expired-positive" || expired[1].ID != "expired-utc" {
		t.Fatalf("wrong expired files: %#v", expired)
	}
}
