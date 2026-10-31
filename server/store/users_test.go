package store

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func useTeamTestData(t *testing.T) *expirationTestBackend {
	t.Helper()
	b := useExpirationTestData(t, nil)
	data.Accounts = []Account{{ID: "storage", Name: "Existing storage"}}
	return b
}

func bootstrapTestOwner(t *testing.T) User {
	t.Helper()
	if err := BootstrapOwner("Admin", "old-password"); err != nil {
		t.Fatal(err)
	}
	owner, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	return *owner
}

func TestBootstrapOwnerPreservesExistingResourcesAndRunsOnce(t *testing.T) {
	b := useTeamTestData(t)
	data.Tokens = []Token{{ID: "legacy-token", Token: "old-value", Permissions: []string{"read"}}}
	data.WebDAVCredentials = []WebDAVCredential{{ID: "legacy-webdav", Password: "old-secret", AccountID: "storage"}}
	data.FileExpirations = []FileExpiration{{ID: "legacy-expiration", AccountID: "storage", FileKey: "old/file.txt", ExpiresAt: "2030-01-01T00:00:00Z"}}
	initialAccounts := append([]Account(nil), data.Accounts...)
	initialExpirations := append([]FileExpiration(nil), data.FileExpirations...)
	// Legacy bootstrap passwords shorter than the new-member minimum are retained.
	if err := BootstrapOwner("OriginalAdmin", "old"); err != nil {
		t.Fatal(err)
	}
	owner, err := AuthenticateUser("originaladmin", "old")
	if err != nil {
		t.Fatal(err)
	}
	if owner.Role != RoleOwner || !owner.IsActive || owner.AuthVersion != 1 || owner.PasswordHash == "old" {
		t.Fatalf("invalid owner: %#v", owner)
	}
	if data.Tokens[0].UserID != owner.ID || data.WebDAVCredentials[0].UserID != owner.ID {
		t.Fatal("legacy credentials were not bound to the owner")
	}
	if data.Tokens[0].Token != "old-value" || data.WebDAVCredentials[0].Password != "old-secret" || !reflect.DeepEqual(data.Accounts, initialAccounts) || !reflect.DeepEqual(data.FileExpirations, initialExpirations) {
		t.Fatal("migration changed existing resources")
	}
	if err := BootstrapOwner("DifferentAdmin", "different-password"); err != nil {
		t.Fatal(err)
	}
	if len(GetUsers()) != 1 || b.saveCount != 1 {
		t.Fatal("bootstrap changed an already initialized team")
	}
	if _, err := AuthenticateUser("OriginalAdmin", "old"); err != nil {
		t.Fatal("bootstrap rewrote the original password")
	}
}

func TestBootstrapOwnerRollsBackOnSaveFailure(t *testing.T) {
	b := useTeamTestData(t)
	data.Tokens = []Token{{ID: "token"}}
	data.WebDAVCredentials = []WebDAVCredential{{ID: "credential"}}
	b.saveError = errors.New("disk full")
	if err := BootstrapOwner("admin", "password"); err == nil {
		t.Fatal("expected save failure")
	}
	if len(data.Users) != 0 || data.Tokens[0].UserID != "" || data.WebDAVCredentials[0].UserID != "" {
		t.Fatal("failed migration remained active in memory")
	}
}

func TestMemberIdentityValidationAndCredentials(t *testing.T) {
	useTeamTestData(t)
	bootstrapTestOwner(t)
	for _, invalid := range []User{{Username: "ok-user", Role: RoleOwner}, {Username: "a", Role: RoleMember}, {Username: "user name", Role: RoleMember}, {Username: "ok-user", Role: "unknown"}} {
		if err := CreateUser(&invalid, "password-123"); !errors.Is(err, ErrInvalidUser) {
			t.Fatalf("invalid member accepted: %v", err)
		}
	}
	user := User{Username: "writer", DisplayName: "Team Writer", Role: RoleMember, IsActive: true}
	if err := CreateUser(&user, "password-123"); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticateUser("WRITER", "password-123"); err != nil {
		t.Fatal(err)
	}
	duplicate := User{Username: "Writer", Role: RoleViewer, IsActive: true}
	if err := CreateUser(&duplicate, "password-123"); !errors.Is(err, ErrUsernameExists) {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := AuthenticateUser("writer", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("wrong password authenticated")
	}
	for _, password := range []string{"short", strings.Repeat("a", 73)} {
		member := User{Username: "another", Role: RoleMember}
		if err := CreateUser(&member, password); !errors.Is(err, ErrInvalidUser) {
			t.Fatalf("invalid password accepted: %v", err)
		}
	}
	returned, _ := GetUserByID(user.ID)
	returned.Role = RoleOwner
	if actual, _ := GetUserByID(user.ID); actual.Role != RoleMember {
		t.Fatal("caller mutated stored user")
	}
	all := GetUsers()
	all[0].Username = "mutated"
	if _, err := GetUserByUsername("Admin"); err != nil {
		t.Fatal("caller mutated stored user list")
	}
}

func TestUserChangesRevokeSessionsAndCannotDisableOwner(t *testing.T) {
	b := useTeamTestData(t)
	owner := bootstrapTestOwner(t)
	if err := UpdateUser(owner.ID, UserUpdate{DisplayName: "Owner", Role: RoleOwner, IsActive: false}); !errors.Is(err, ErrOwnerProtected) {
		t.Fatal("owner could be disabled")
	}
	if err := UpdateUser(owner.ID, UserUpdate{DisplayName: "Owner", Role: RoleAdmin, IsActive: true}); !errors.Is(err, ErrOwnerProtected) {
		t.Fatal("owner could be demoted")
	}
	user := User{Username: "writer", Role: RoleMember, IsActive: true}
	if err := CreateUser(&user, "password-123"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateUser(user.ID, UserUpdate{Role: RoleOwner, IsActive: true}); !errors.Is(err, ErrOwnerProtected) {
		t.Fatal("member could become owner")
	}
	if err := UpdateUser(user.ID, UserUpdate{DisplayName: "New name", Role: RoleMember, IsActive: true}); err != nil {
		t.Fatal(err)
	}
	current, _ := GetUserByID(user.ID)
	if current.AuthVersion != user.AuthVersion {
		t.Fatal("display name change revoked sessions")
	}
	if err := UpdateUser(user.ID, UserUpdate{DisplayName: "New name", Role: RoleViewer, IsActive: true}); err != nil {
		t.Fatal(err)
	}
	current, _ = GetUserByID(user.ID)
	if current.AuthVersion != user.AuthVersion+1 || current.HasPermission("write") {
		t.Fatal("role change did not revoke write access")
	}
	previous := *current
	b.saveError = errors.New("disk full")
	if err := UpdateUser(user.ID, UserUpdate{Role: RoleMember, IsActive: false, Password: "new-password"}); err == nil {
		t.Fatal("expected save failure")
	}
	current, _ = GetUserByID(user.ID)
	if *current != previous {
		t.Fatal("failed update changed stored identity")
	}
	b.saveError = nil
	if err := UpdateUser(user.ID, UserUpdate{Role: RoleViewer, IsActive: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticateUser("writer", "password-123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("disabled member authenticated")
	}
}

func TestPasswordChangeRequiresCurrentPasswordAndRollsBack(t *testing.T) {
	b := useTeamTestData(t)
	owner := bootstrapTestOwner(t)
	if err := ChangeUserPassword(owner.ID, "wrong-password", "new-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("wrong current password allowed a change")
	}
	b.saveError = errors.New("disk full")
	if err := ChangeUserPassword(owner.ID, "old-password", "new-password"); err == nil {
		t.Fatal("expected save failure")
	}
	if current, _ := GetUserByID(owner.ID); *current != owner {
		t.Fatal("failed password update changed identity")
	}
	b.saveError = nil
	if err := ChangeUserPassword(owner.ID, "old-password", "new-password"); err != nil {
		t.Fatal(err)
	}
	if current, _ := GetUserByID(owner.ID); current.AuthVersion != owner.AuthVersion+1 {
		t.Fatal("password change did not revoke sessions")
	}
	if _, err := AuthenticateUser("Admin", "old-password"); err == nil {
		t.Fatal("old password remained valid")
	}
	if _, err := AuthenticateUser("Admin", "new-password"); err != nil {
		t.Fatal(err)
	}
	if err := RevokeUserSessions(owner.ID); err != nil {
		t.Fatal(err)
	}
	if current, _ := GetUserByID(owner.ID); current.AuthVersion != owner.AuthVersion+2 {
		t.Fatal("session revocation did not advance version")
	}
}

func TestTeamCredentialsRespectCurrentMemberPermissions(t *testing.T) {
	b := useTeamTestData(t)
	owner := bootstrapTestOwner(t)
	viewer := User{Username: "viewer", Role: RoleViewer, IsActive: true}
	if err := CreateUser(&viewer, "password-123"); err != nil {
		t.Fatal(err)
	}
	if err := CreateToken(&Token{UserID: viewer.ID, Permissions: []string{"write"}}); !errors.Is(err, ErrInvalidUser) {
		t.Fatal("viewer received write token")
	}
	token := Token{UserID: viewer.ID, Permissions: []string{"read"}}
	if err := CreateToken(&token); err != nil {
		t.Fatal(err)
	}
	defaultToken := Token{Permissions: []string{"read"}}
	if err := CreateToken(&defaultToken); err != nil || defaultToken.UserID != owner.ID {
		t.Fatalf("default token owner = %q, %v", defaultToken.UserID, err)
	}
	if err := CreateWebDAVCredential(&WebDAVCredential{AccountID: "storage", UserID: viewer.ID, Permissions: []string{"delete"}}); !errors.Is(err, ErrInvalidUser) {
		t.Fatal("viewer received deleting WebDAV credential")
	}
	credential := WebDAVCredential{AccountID: "storage", UserID: viewer.ID, Permissions: []string{"read"}}
	if err := CreateWebDAVCredential(&credential); err != nil {
		t.Fatal(err)
	}
	previousTokens := append([]Token(nil), data.Tokens...)
	previousCredentials := append([]WebDAVCredential(nil), data.WebDAVCredentials...)
	b.saveError = errors.New("disk full")
	if err := CreateToken(&Token{Permissions: []string{"read"}}); err == nil {
		t.Fatal("expected save failure")
	}
	if err := CreateWebDAVCredential(&WebDAVCredential{AccountID: "storage", Permissions: []string{"read"}}); err == nil {
		t.Fatal("expected save failure")
	}
	if !reflect.DeepEqual(data.Tokens, previousTokens) || !reflect.DeepEqual(data.WebDAVCredentials, previousCredentials) {
		t.Fatal("failed credential creation remained active")
	}
	b.saveError = nil
	if err := UpdateUser(viewer.ID, UserUpdate{Role: RoleViewer, IsActive: false}); err != nil {
		t.Fatal(err)
	}
	if err := CreateToken(&Token{UserID: viewer.ID, Permissions: []string{"read"}}); !errors.Is(err, ErrInvalidUser) {
		t.Fatal("disabled member received token")
	}
	if err := UpdateWebDAVCredential(credential.ID, &WebDAVCredential{IsActive: false}); err != nil {
		t.Fatal("could not disable a disabled member's credential")
	}
}

func TestTeamStateRoundTripAndRejectsCorruption(t *testing.T) {
	useTeamTestData(t)
	owner := bootstrapTestOwner(t)
	data.Tokens = []Token{{ID: "token", UserID: owner.ID}}
	data.WebDAVCredentials = []WebDAVCredential{{ID: "credential", UserID: owner.ID}}
	data.AuditEntries = []AuditEntry{{ID: "event", Action: "upload", Target: "file.txt", Status: 200}}
	encoded, err := marshalTeamState(data)
	if err != nil {
		t.Fatal(err)
	}
	loaded := Data{Tokens: []Token{{ID: "token"}}, WebDAVCredentials: []WebDAVCredential{{ID: "credential"}}}
	if err := applyTeamState(&loaded, encoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Users, data.Users) || !reflect.DeepEqual(loaded.AuditEntries, data.AuditEntries) || loaded.Tokens[0].UserID != owner.ID || loaded.WebDAVCredentials[0].UserID != owner.ID {
		t.Fatal("team metadata did not round trip")
	}
	if bcrypt.CompareHashAndPassword([]byte(loaded.Users[0].PasswordHash), []byte("old-password")) != nil {
		t.Fatal("password hash did not round trip")
	}
	for _, invalid := range []string{"{", `{"version":99}`, `{"version":1,"users":[{"id":"x","username":"admin","role":"unknown","passwordHash":"x"}]}`} {
		if err := applyTeamState(&Data{}, invalid); err == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
}

func TestAuditRetentionAndSaveFailure(t *testing.T) {
	b := useTeamTestData(t)
	for i := 0; i < maxAuditEntries; i++ {
		data.AuditEntries = append(data.AuditEntries, AuditEntry{ID: string(rune(i + 1))})
	}
	previous := append([]AuditEntry(nil), data.AuditEntries...)
	b.saveError = errors.New("disk full")
	entry := AuditEntry{Action: "delete", Target: "file.txt", Status: 200}
	if err := AppendAuditEntry(&entry); err == nil || !reflect.DeepEqual(data.AuditEntries, previous) {
		t.Fatal("failed audit save changed records")
	}
	b.saveError = nil
	if err := AppendAuditEntry(&entry); err != nil {
		t.Fatal(err)
	}
	if len(data.AuditEntries) != maxAuditEntries || data.AuditEntries[0].ID != previous[1].ID || entry.ID == "" || entry.CreatedAt == "" {
		t.Fatal("audit retention incorrect")
	}
	logs := GetAuditEntries(1)
	if len(logs) != 1 || logs[0] != entry {
		t.Fatal("audit entries are not newest first")
	}
}
