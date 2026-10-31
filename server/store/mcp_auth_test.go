package store

import (
	"errors"
	"testing"
)

func TestManagementScopeOnlyAcceptedForAdministratorAPITokens(t *testing.T) {
	useTeamTestData(t)
	owner := bootstrapTestOwner(t)
	member := &User{Username: "mcp-member", Role: RoleMember, IsActive: true}
	if err := CreateUser(member, "member-password"); err != nil {
		t.Fatal(err)
	}
	if err := CreateToken(&Token{Name: "manager", UserID: owner.ID, Permissions: []string{"read", "write", "delete", "manage"}}); err != nil {
		t.Fatal(err)
	}
	if err := CreateToken(&Token{Name: "partial-manager", UserID: owner.ID, Permissions: []string{"read", "manage"}}); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("partial management accepted: %v", err)
	}
	if err := CreateToken(&Token{Name: "member-manager", UserID: member.ID, Permissions: []string{"manage"}}); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("member granted management scope: %v", err)
	}
	if err := CreateToken(&Token{Name: "unknown", UserID: owner.ID, Permissions: []string{"unknown"}}); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("invalid scope accepted: %v", err)
	}
	if err := CreateWebDAVCredential(&WebDAVCredential{AccountID: "storage", UserID: owner.ID, Username: "mcp-webdav", Password: "password", Permissions: []string{"manage"}}); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("WebDAV granted management scope: %v", err)
	}
}
