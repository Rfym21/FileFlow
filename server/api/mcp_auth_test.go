package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"fileflow/server/middleware"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

func addMCPAuthTestRoutes(router *gin.Engine) {
	group := router.Group("/mcp-auth-test", middleware.MCPAuthMiddleware())
	group.GET("/check", Check)
	group.POST("/tokens", middleware.RequirePermission("manage"), CreateToken)
	group.GET("/tokens", middleware.RequirePermission("manage"), GetTokens)
	// This route intentionally invokes the shared handler without the tool's
	// management guard, verifying its independent delegation ceiling.
	group.POST("/delegate-test", CreateToken)
}

func TestMCPManagementTokenScopeIsOptInAndTransportSpecific(t *testing.T) {
	router, ownerJWT, _ := setupExpirationAPI(t)
	addMCPAuthTestRoutes(router)
	owner, err := store.GetUserByUsername("test-admin")
	if err != nil {
		t.Fatal(err)
	}
	member, _ := teamUser(t, "mcp-member", "member")
	for _, userID := range []string{owner.ID, member.ID} {
		response := expirationRequest(t, router, "POST", "/api/tokens", gin.H{"name": "agent", "userId": userID, "permissions": []string{"read", "write", "delete", "manage"}}, ownerJWT)
		if userID == member.ID {
			if response.Code != http.StatusBadRequest {
				t.Fatalf("member granted manage: %d %s", response.Code, response.Body.String())
			}
			continue
		}
		if response.Code != http.StatusCreated {
			t.Fatalf("create manager token: %d %s", response.Code, response.Body.String())
		}
		var token TokenResponse
		if err := json.Unmarshal(response.Body.Bytes(), &token); err != nil {
			t.Fatal(err)
		}
		for _, input := range []struct {
			path       string
			wantManage bool
			wantStatus int
		}{
			{"/mcp-auth-test/check", true, http.StatusOK},
			{"/api/check", false, http.StatusOK},
			{"/api/users", false, http.StatusUnauthorized},
			{"/api/accounts", false, http.StatusUnauthorized},
		} {
			response := expirationRequest(t, router, "GET", input.path, nil, token.Token)
			if response.Code != input.wantStatus || strings.Contains(response.Body.String(), `"manage"`) != input.wantManage {
				t.Fatalf("%s token access: %d %s", input.path, response.Code, response.Body.String())
			}
		}
		// The permissions array persists in the existing database schema.
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		if err := store.Init(); err != nil {
			t.Fatal(err)
		}
		if response := expirationRequest(t, router, "GET", "/mcp-auth-test/check", nil, token.Token); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"manage"`) {
			t.Fatalf("manage scope lost after reload: %d %s", response.Code, response.Body.String())
		}
	}
	account := &store.Account{Name: "webdav", IsActive: true, Permissions: store.DefaultAccountPermissions()}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	response := expirationRequest(t, router, "POST", "/api/webdav-credentials", gin.H{"accountId": account.ID, "permissions": []string{"manage"}}, ownerJWT)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("WebDAV accepted manage: %d %s", response.Code, response.Body.String())
	}
	response = expirationRequest(t, router, "POST", "/api/tokens", gin.H{"name": "partial-management", "permissions": []string{"manage"}}, ownerJWT)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "read、write、delete") {
		t.Fatalf("partial manage scope accepted: %d %s", response.Code, response.Body.String())
	}
}

func TestMCPTokenDelegationAndLiveRoleChanges(t *testing.T) {
	router, _, _ := setupExpirationAPI(t)
	addMCPAuthTestRoutes(router)
	admin, _ := teamUser(t, "mcp-manager", "admin")
	token := &store.Token{Name: "read-only", UserID: admin.ID, Permissions: []string{"read"}}
	if err := store.CreateToken(token); err != nil {
		t.Fatal(err)
	}
	stronger := &store.Token{Name: "full-agent", UserID: admin.ID, Permissions: []string{"read", "write", "delete", "manage"}}
	if err := store.CreateToken(stronger); err != nil {
		t.Fatal(err)
	}
	response := expirationRequest(t, router, "POST", "/mcp-auth-test/delegate-test", gin.H{"name": "broader", "permissions": []string{"read", "write"}}, token.Token)
	if response.Code != http.StatusForbidden {
		t.Fatalf("scoped token delegated broader read: %d %s", response.Code, response.Body.String())
	}
	response = expirationRequest(t, router, "POST", "/mcp-auth-test/tokens", gin.H{"name": "same-scope", "permissions": []string{"read", "write", "delete", "manage"}}, stronger.Token)
	if response.Code != http.StatusCreated {
		t.Fatalf("same scope delegation failed: %d %s", response.Code, response.Body.String())
	}
	response = expirationRequest(t, router, "GET", "/mcp-auth-test/tokens", nil, stronger.Token)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), stronger.Token) || strings.Contains(response.Body.String(), token.Token) {
		t.Fatalf("credential list leaked bearer values: %d %s", response.Code, response.Body.String())
	}
	if err := store.UpdateUser(admin.ID, store.UserUpdate{Role: "member", IsActive: true}); err != nil {
		t.Fatal(err)
	}
	response = expirationRequest(t, router, "GET", "/mcp-auth-test/check", nil, stronger.Token)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"manage"`) || !strings.Contains(response.Body.String(), `"read"`) {
		t.Fatalf("role downgrade did not intersect scopes: %d %s", response.Code, response.Body.String())
	}
	if err := store.UpdateUser(admin.ID, store.UserUpdate{Role: "member", IsActive: false}); err != nil {
		t.Fatal(err)
	}
	if response = expirationRequest(t, router, "GET", "/mcp-auth-test/check", nil, stronger.Token); response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled member token still authenticates: %d", response.Code)
	}
}
