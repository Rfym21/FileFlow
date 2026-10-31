package store

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSQLiteLegacyMigrationAndAtomicTeamPersistence(t *testing.T) {
	previousData, previousBackend := data, backend
	t.Cleanup(func() { data, backend = previousData, previousBackend })
	path := filepath.Join(t.TempDir(), "legacy.db")
	b, err := NewSQLiteBackend(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	legacy := &Data{
		Accounts:          []Account{{ID: "storage", Name: "Storage", Permissions: DefaultAccountPermissions()}},
		Tokens:            []Token{{ID: "token", Name: "Legacy Token", Token: "sk-old", Permissions: []string{"read"}}},
		WebDAVCredentials: []WebDAVCredential{{ID: "credential", Username: "legacy-webdav", Password: "secret", AccountID: "storage", IsActive: true}},
		FileExpirations:   []FileExpiration{{ID: "expiration", AccountID: "storage", FileKey: "old/file.txt", ExpiresAt: "2030-01-01T00:00:00Z"}},
		Settings:          Settings{SyncInterval: 5, DefaultExpirationDays: 30, ExpirationCheckMinutes: 720},
	}
	if err := b.Save(legacy); err != nil {
		t.Fatal(err)
	}
	// Simulate an old database containing only the original tables.
	if _, err := b.db.Exec("DROP TABLE team_state"); err != nil {
		t.Fatal(err)
	}
	if err := b.createTables(); err != nil {
		t.Fatal(err)
	}
	loaded, err := b.Load()
	if err != nil {
		t.Fatal(err)
	}
	data, backend = loaded, b
	if err := BootstrapOwner("admin", "old-password"); err != nil {
		t.Fatal(err)
	}
	member := User{Username: "reader", Role: RoleViewer, IsActive: true}
	if err := CreateUser(&member, "member-password"); err != nil {
		t.Fatal(err)
	}
	entry := AuditEntry{UserID: member.ID, Action: "read", Target: "old/file.txt", Status: 200}
	if err := AppendAuditEntry(&entry); err != nil {
		t.Fatal(err)
	}
	roundtrip, err := b.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(roundtrip.Users) != 2 || roundtrip.Users[1] != member || !reflect.DeepEqual(roundtrip.AuditEntries, data.AuditEntries) {
		t.Fatal("SQLite did not persist team identities and audit")
	}
	if !reflect.DeepEqual(roundtrip.Accounts, loaded.Accounts) || !reflect.DeepEqual(roundtrip.FileExpirations, legacy.FileExpirations) || roundtrip.Tokens[0].Token != "sk-old" || roundtrip.Tokens[0].UserID != data.Users[0].ID || roundtrip.WebDAVCredentials[0].UserID != data.Users[0].ID || roundtrip.WebDAVCredentials[0].Password != "secret" {
		t.Fatal("migration did not preserve legacy resources")
	}
	before := roundtrip.Users[1]
	if _, err := b.db.Exec("CREATE TRIGGER reject_team_save BEFORE INSERT ON team_state BEGIN SELECT RAISE(ABORT, 'test persistence failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateUser(member.ID, UserUpdate{DisplayName: "Changed", Role: RoleMember, IsActive: true}); err == nil {
		t.Fatal("expected team save failure")
	}
	actual, _ := GetUserByID(member.ID)
	if *actual != before {
		t.Fatal("failed transaction changed in-memory user")
	}
	reloaded, err := b.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Users[1] != before || !reflect.DeepEqual(reloaded.Tokens, roundtrip.Tokens) || !reflect.DeepEqual(reloaded.Accounts, roundtrip.Accounts) {
		t.Fatal("failed team save changed persistent resources")
	}
}
