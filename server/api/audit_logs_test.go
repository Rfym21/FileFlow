package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

func TestTeamAuditRecordsActorAndDenialWithoutCredentials(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	before := len(store.GetAuditEntries(2000))
	anonymous := expirationRequest(t, router, http.MethodPost, "/api/upload", nil, "")
	if anonymous.Code != http.StatusUnauthorized || len(store.GetAuditEntries(2000)) != before {
		t.Fatal("anonymous requests must not write team audit snapshots")
	}
	password := "viewer-test-password"
	created := expirationRequest(t, router, http.MethodPost, "/api/users", gin.H{
		"username": "audit-viewer", "displayName": "只读成员", "password": password, "role": "viewer",
	}, ownerToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("create viewer: %d %s", created.Code, created.Body.String())
	}
	var viewer UserProfile
	if err := json.Unmarshal(created.Body.Bytes(), &viewer); err != nil {
		t.Fatal(err)
	}
	loggedIn := expirationRequest(t, router, http.MethodPost, "/api/auth/login", gin.H{
		"username": "audit-viewer", "password": password,
	}, "")
	if loggedIn.Code != http.StatusOK {
		t.Fatalf("login viewer: %d %s", loggedIn.Code, loggedIn.Body.String())
	}
	var login LoginResponse
	if err := json.Unmarshal(loggedIn.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	secret := "must-not-appear-in-audit"
	denied := expirationRequest(t, router, http.MethodPut, "/api/settings", gin.H{"secret": secret}, login.Token)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("viewer setting mutation: %d", denied.Code)
	}
	for _, bearer := range []string{"", login.Token} {
		result := expirationRequest(t, router, http.MethodGet, "/api/audit-logs", nil, bearer)
		if result.Code != http.StatusUnauthorized && result.Code != http.StatusForbidden {
			t.Fatalf("non-admin read audit: %d", result.Code)
		}
	}
	result := expirationRequest(t, router, http.MethodGet, "/api/audit-logs?limit=100", nil, ownerToken)
	if result.Code != http.StatusOK {
		t.Fatalf("owner read audit: %d %s", result.Code, result.Body.String())
	}
	var response struct {
		Entries []store.AuditEntry `json:"entries"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Entries) < 3 {
		t.Fatalf("missing audit events: %#v", response.Entries)
	}
	entry := response.Entries[0]
	if entry.UserID != viewer.ID || entry.Username != "audit-viewer" || entry.Status != http.StatusForbidden {
		t.Fatalf("audit failed to attribute denial: %#v", entry)
	}
	for _, sensitive := range []string{password, secret, login.Token, "passwordHash"} {
		if strings.Contains(result.Body.String(), sensitive) {
			t.Fatalf("audit contains confidential value %q", sensitive)
		}
	}
	for _, limit := range []string{"0", "2001", "invalid"} {
		result = expirationRequest(t, router, http.MethodGet, "/api/audit-logs?limit="+limit, nil, ownerToken)
		if result.Code != http.StatusBadRequest {
			t.Fatalf("invalid limit %q returned %d", limit, result.Code)
		}
	}
}
