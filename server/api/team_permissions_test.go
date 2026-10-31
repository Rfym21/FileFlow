package api

import (
	"encoding/json"
	"fileflow/server/config"
	"fileflow/server/middleware"
	"fileflow/server/store"
	"fileflow/server/webdav"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func teamUser(t *testing.T, name, role string) (*store.User, string) {
	t.Helper()
	user := &store.User{Username: name, DisplayName: name, Role: role, IsActive: true}
	if err := store.CreateUser(user, "member-password"); err != nil {
		t.Fatal(err)
	}
	token, err := middleware.GenerateJWT(name)
	if err != nil {
		t.Fatal(err)
	}
	return user, token
}
func TestTeamRolesProtectManagementAndR2Credentials(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account := &store.Account{Name: "shared", IsActive: true, AccountID: "cloudflare", Endpoint: "https://example.invalid", BucketName: "bucket", AccessKeyId: "sensitive-access", SecretAccessKey: "sensitive-secret", APIToken: "sensitive-cloudflare-token"}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	_, memberToken := teamUser(t, "writer", "member")
	_, viewerToken := teamUser(t, "reader", "viewer")
	_, adminToken := teamUser(t, "manager", "admin")
	for _, token := range []string{memberToken, viewerToken} {
		for _, endpoint := range []string{"/api/accounts", "/api/accounts?page=1&pageSize=5", "/api/accounts/" + account.ID} {
			response := expirationRequest(t, router, "GET", endpoint, nil, token)
			if response.Code != http.StatusOK {
				t.Fatalf("member account metadata returned %d: %s", response.Code, response.Body.String())
			}
			for _, secret := range []string{"secretAccessKey", "accessKeyId", "apiToken", "sensitive-secret", "sensitive-access"} {
				if strings.Contains(response.Body.String(), "\""+secret+"\"") {
					t.Fatalf("account metadata exposed %s: %s", secret, response.Body.String())
				}
			}
		}
		for _, endpoint := range []string{"/api/users", "/api/tokens", "/api/webdav-credentials", "/api/audit-logs"} {
			response := expirationRequest(t, router, "GET", endpoint, nil, token)
			if response.Code != http.StatusForbidden {
				t.Fatalf("%s allowed ordinary member: %d", endpoint, response.Code)
			}
		}
		for _, endpoint := range []string{"/api/accounts", "/api/accounts/sync", "/api/accounts/" + account.ID + "/clear", "/api/accounts/delete-old-files"} {
			if response := expirationRequest(t, router, "POST", endpoint, gin.H{}, token); response.Code != http.StatusForbidden {
				t.Fatalf("%s allowed ordinary member: %d", endpoint, response.Code)
			}
		}
		if response := expirationRequest(t, router, "PUT", "/api/settings", gin.H{}, token); response.Code != http.StatusForbidden {
			t.Fatalf("ordinary member changed settings: %d", response.Code)
		}
	}
	for _, token := range []string{ownerToken, adminToken} {
		response := expirationRequest(t, router, "GET", "/api/accounts/"+account.ID, nil, token)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "sensitive-secret") {
			t.Fatalf("manager cannot edit credentials: %d: %s", response.Code, response.Body.String())
		}
	}
	record := createExpiration(t, account.ID, "shared.txt")
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	for _, input := range []struct {
		method, endpoint string
		body             any
	}{
		{"POST", "/api/upload", gin.H{}},
		{"DELETE", "/api/file?idGroup=" + account.ID + "&key=shared.txt", nil},
		{"PATCH", "/api/file-expirations/" + record.ID, gin.H{"expiresAt": future}},
		{"POST", "/api/file-expirations/batch-delete", gin.H{"ids": []string{record.ID}}},
	} {
		response := expirationRequest(t, router, input.method, input.endpoint, input.body, viewerToken)
		if response.Code != http.StatusForbidden {
			t.Fatalf("viewer mutation %s returned %d", input.endpoint, response.Code)
		}
	}
	if response := expirationRequest(t, router, "GET", "/api/file-expirations", nil, viewerToken); response.Code != http.StatusOK {
		t.Fatalf("viewer cannot see schedule: %d", response.Code)
	}
	if response := expirationRequest(t, router, "PATCH", "/api/file-expirations/"+record.ID, gin.H{"expiresAt": future}, memberToken); response.Code != http.StatusOK {
		t.Fatalf("writer cannot modify schedule: %d: %s", response.Code, response.Body.String())
	}
}

func TestMemberAdministrationProtectsOwnerAndAdmins(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	admin, adminToken := teamUser(t, "manager", "admin")
	reader, _ := teamUser(t, "reader", "viewer")
	owner, err := store.GetUserByUsername("test-admin")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		method, path string
		body         any
	}{
		{"POST", "/api/users", gin.H{"username": "new-admin", "password": "member-password", "role": "admin"}},
		{"POST", "/api/users", gin.H{"username": "new-owner", "password": "member-password", "role": "owner"}},
		{"PUT", "/api/users/" + owner.ID, gin.H{"displayName": "Other", "role": "owner", "isActive": true}},
		{"PUT", "/api/users/" + admin.ID, gin.H{"role": "member", "isActive": true}},
		{"PUT", "/api/users/" + reader.ID, gin.H{"role": "admin", "isActive": true}},
	} {
		response := expirationRequest(t, router, input.method, input.path, input.body, adminToken)
		if response.Code != http.StatusForbidden {
			t.Fatalf("admin privilege escalation %s: %d %s", input.path, response.Code, response.Body.String())
		}
	}
	response := expirationRequest(t, router, "PUT", "/api/users/"+owner.ID, gin.H{"displayName": "Team Owner", "role": "owner", "isActive": true}, ownerToken)
	if response.Code != http.StatusOK {
		t.Fatalf("owner profile edit failed: %d %s", response.Code, response.Body.String())
	}
	response = expirationRequest(t, router, "POST", "/api/users", gin.H{"username": "added-reader", "displayName": "New Reader", "password": "member-password", "role": "viewer"}, adminToken)
	if response.Code != http.StatusCreated || strings.Contains(response.Body.String(), "password") || strings.Contains(response.Body.String(), "authVersion") {
		t.Fatalf("unsafe user creation response: %d %s", response.Code, response.Body.String())
	}
	response = expirationRequest(t, router, "GET", "/api/users", nil, ownerToken)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "passwordHash") || strings.Contains(response.Body.String(), "authVersion") {
		t.Fatalf("unsafe user list: %s", response.Body.String())
	}
	response = expirationRequest(t, router, "PUT", "/api/users/"+owner.ID, gin.H{"role": "viewer", "isActive": false}, ownerToken)
	if response.Code != http.StatusForbidden {
		t.Fatalf("owner was disabled: %d", response.Code)
	}
}

func TestRoleChangesAndDisabledMembersRevokeSessionsAndLimitCredentials(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	user, session := teamUser(t, "writer", "member")
	token := &store.Token{Name: "integration", UserID: user.ID, Permissions: []string{"read", "write", "delete"}}
	if err := store.CreateToken(token); err != nil {
		t.Fatal(err)
	}
	account := &store.Account{Name: "shared", IsActive: true, Permissions: store.DefaultAccountPermissions()}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	credential := &store.WebDAVCredential{AccountID: account.ID, UserID: user.ID, Username: "webdav-writer", Password: "webdav-password", Permissions: []string{"read", "write", "delete"}}
	if err := store.CreateWebDAVCredential(credential); err != nil {
		t.Fatal(err)
	}
	response := expirationRequest(t, router, "PUT", "/api/users/"+user.ID, gin.H{"role": "viewer", "isActive": true}, ownerToken)
	if response.Code != http.StatusOK {
		t.Fatalf("role change failed: %d %s", response.Code, response.Body.String())
	}
	if response = expirationRequest(t, router, "GET", "/api/check", nil, session); response.Code != http.StatusUnauthorized {
		t.Fatalf("old role session remained valid: %d", response.Code)
	}
	response = expirationRequest(t, router, "GET", "/api/check", nil, token.Token)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "\"permissions\":[\"read\"]") {
		t.Fatalf("role downgrade did not limit API token: %d %s", response.Code, response.Body.String())
	}
	if response = expirationRequest(t, router, "POST", "/api/upload", gin.H{}, token.Token); response.Code != http.StatusForbidden {
		t.Fatalf("downgraded API token can write: %d", response.Code)
	}
	handler := webdav.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current, _ := webdav.GetCredentialFromContext(r.Context())
		json.NewEncoder(w).Encode(current.Permissions)
	}))
	req := httptest.NewRequest("GET", "/webdav/", nil)
	req.SetBasicAuth(credential.Username, credential.Password)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "[\"read\"]" {
		t.Fatalf("WebDAV kept old permissions: %d %s", recorder.Code, recorder.Body.String())
	}
	response = expirationRequest(t, router, "POST", "/api/tokens", gin.H{"name": "invalid", "userId": user.ID, "permissions": []string{"write"}}, ownerToken)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("created credential above member role: %d", response.Code)
	}
	response = expirationRequest(t, router, "PUT", "/api/users/"+user.ID, gin.H{"role": "viewer", "isActive": false}, ownerToken)
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	if response = expirationRequest(t, router, "GET", "/api/check", nil, token.Token); response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled member API token remained valid: %d", response.Code)
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("disabled member WebDAV remained valid: %d", recorder.Code)
	}
	response = expirationRequest(t, router, "PUT", "/api/webdav-credentials/"+credential.ID, gin.H{"permissions": []string{"read"}, "isActive": false}, ownerToken)
	if response.Code != http.StatusOK {
		t.Fatalf("could not disable disabled member's WebDAV: %d %s", response.Code, response.Body.String())
	}
}

func TestMemberPasswordsLogoutAndJWTValidation(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	user, first := teamUser(t, "writer", "member")
	second, err := middleware.GenerateJWT(user.Username)
	if err != nil {
		t.Fatal(err)
	}
	response := expirationRequest(t, router, "POST", "/api/auth/password", gin.H{"currentPassword": "wrong-password", "newPassword": "changed-password"}, first)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("wrong current password accepted: %d", response.Code)
	}
	response = expirationRequest(t, router, "POST", "/api/auth/password", gin.H{"currentPassword": "member-password", "newPassword": "changed-password"}, first)
	if response.Code != http.StatusOK {
		t.Fatalf("password change failed: %d %s", response.Code, response.Body.String())
	}
	for _, token := range []string{first, second} {
		if response = expirationRequest(t, router, "GET", "/api/check", nil, token); response.Code != http.StatusUnauthorized {
			t.Fatalf("old password session remained valid: %d", response.Code)
		}
	}
	response = expirationRequest(t, router, "POST", "/api/auth/login", gin.H{"username": "WRITER", "password": "changed-password"}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("updated password login failed: %d %s", response.Code, response.Body.String())
	}
	var login LoginResponse
	if err := json.Unmarshal(response.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	if login.Profile.ID != user.ID || len(login.Permissions) != 3 || strings.Contains(response.Body.String(), "passwordHash") {
		t.Fatal("login did not return safe member profile")
	}
	second, err = middleware.GenerateJWT(user.Username)
	if err != nil {
		t.Fatal(err)
	}
	response = expirationRequest(t, router, "POST", "/api/auth/logout", nil, login.Token)
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	if response = expirationRequest(t, router, "GET", "/api/check", nil, second); response.Code != http.StatusUnauthorized {
		t.Fatal("logout did not revoke another session")
	}
	owner, err := store.GetUserByUsername("test-admin")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	malformed := []struct {
		claims middleware.Claims
		method jwt.SigningMethod
	}{
		{middleware.Claims{Username: owner.Username, AuthVersion: owner.AuthVersion, RegisteredClaims: jwt.RegisteredClaims{Issuer: "fileflow", ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)), IssuedAt: jwt.NewNumericDate(now)}}, jwt.SigningMethodHS256},
		{middleware.Claims{Username: owner.Username, AuthVersion: owner.AuthVersion, RegisteredClaims: jwt.RegisteredClaims{Issuer: "fileflow", Subject: owner.ID, IssuedAt: jwt.NewNumericDate(now)}}, jwt.SigningMethodHS256},
		{middleware.Claims{Username: owner.Username, AuthVersion: owner.AuthVersion, RegisteredClaims: jwt.RegisteredClaims{Issuer: "fileflow", Subject: owner.ID, ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)), IssuedAt: jwt.NewNumericDate(now.Add(-2 * time.Hour))}}, jwt.SigningMethodHS256},
		{middleware.Claims{Username: owner.Username, AuthVersion: owner.AuthVersion, RegisteredClaims: jwt.RegisteredClaims{Issuer: "fileflow", Subject: owner.ID, ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)), IssuedAt: jwt.NewNumericDate(now)}}, jwt.SigningMethodHS384},
	}
	for _, test := range malformed {
		value, err := jwt.NewWithClaims(test.method, test.claims).SignedString([]byte(config.Get().JWTSecret))
		if err != nil {
			t.Fatal(err)
		}
		if response = expirationRequest(t, router, "GET", "/api/check", nil, value); response.Code != http.StatusUnauthorized {
			t.Fatalf("invalid JWT accepted: %d", response.Code)
		}
	}
	if response = expirationRequest(t, router, "GET", "/api/check", nil, ownerToken); response.Code != http.StatusOK {
		t.Fatal("one member's logout invalidated owner")
	}
}

func TestAdminsCannotImpersonateOwnersThroughCredentials(t *testing.T) {
	router, ownerSession, _ := setupExpirationAPI(t)
	owner, err := store.GetUserByUsername("test-admin")
	if err != nil {
		t.Fatal(err)
	}
	_, adminSession := teamUser(t, "manager", "admin")
	ownerToken := &store.Token{Name: "owner integration", UserID: owner.ID, Permissions: []string{"read"}}
	if err := store.CreateToken(ownerToken); err != nil {
		t.Fatal(err)
	}
	account := &store.Account{Name: "shared", IsActive: true, Permissions: store.DefaultAccountPermissions()}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	ownerCredential := &store.WebDAVCredential{AccountID: account.ID, UserID: owner.ID, Username: "owner-webdav", Password: "owner-webdav-password", Permissions: []string{"read"}}
	if err := store.CreateWebDAVCredential(ownerCredential); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"/api/tokens", "/api/webdav-credentials"} {
		response := expirationRequest(t, router, "GET", endpoint, nil, adminSession)
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), ownerToken.Token) || strings.Contains(response.Body.String(), ownerCredential.Password) {
			t.Fatalf("admin exposed owner credential: %d %s", response.Code, response.Body.String())
		}
	}
	for _, input := range []struct {
		method, endpoint string
		body             any
	}{
		{"POST", "/api/tokens", gin.H{"name": "impersonation", "userId": owner.ID, "permissions": []string{"read"}}},
		{"POST", "/api/webdav-credentials", gin.H{"accountId": account.ID, "userId": owner.ID, "permissions": []string{"read"}}},
		{"DELETE", "/api/tokens/" + ownerToken.ID, nil},
		{"DELETE", "/api/webdav-credentials/" + ownerCredential.ID, nil},
		{"PUT", "/api/webdav-credentials/" + ownerCredential.ID, gin.H{"permissions": []string{"read"}, "isActive": false}},
	} {
		response := expirationRequest(t, router, input.method, input.endpoint, input.body, adminSession)
		if response.Code != http.StatusForbidden {
			t.Fatalf("admin impersonation %s: %d %s", input.endpoint, response.Code, response.Body.String())
		}
	}
	response := expirationRequest(t, router, "GET", "/api/tokens", nil, ownerSession)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), ownerToken.Token) {
		t.Fatal("owner cannot manage own credential")
	}
}
