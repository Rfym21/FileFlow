package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fileflow/server/middleware"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mcpTestPrincipal(user *store.User, authType string, permissions []string) *gin.Context {
	principal, _ := gin.CreateTestContext(httptest.NewRecorder())
	principal.Set(middleware.ContextKeyProfile, user)
	principal.Set(middleware.ContextKeyUser, user.Username)
	principal.Set(middleware.ContextKeyUserID, user.ID)
	principal.Set(middleware.ContextKeyAuthType, authType)
	principal.Set(middleware.ContextKeyTokenPerm, permissions)
	return principal
}

func mcpTestCall(t *testing.T, principal *gin.Context, name string, arguments any) *mcp.CallToolResult {
	t.Helper()
	encoded, err := json.Marshal(arguments)
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range mcpToolCatalog() {
		if definition.name == "fileflow_"+name {
			ctx := context.WithValue(context.Background(), mcpPrincipalKey{}, principal)
			return executeMCPTool(ctx, definition, encoded)
		}
	}
	t.Fatalf("tool not found: %s", name)
	return nil
}

func mcpResultStatus(t *testing.T, result *mcp.CallToolResult, expected int) any {
	t.Helper()
	output, ok := result.StructuredContent.(map[string]any)
	if !ok || output["status"] != expected || result.IsError != (expected < 200 || expected >= 300) {
		t.Fatalf("unexpected MCP result: %+v", result)
	}
	if len(result.Content) != 1 {
		t.Fatalf("missing text content: %+v", result)
	}
	var textOutput map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &textOutput); err != nil || int(textOutput["status"].(float64)) != expected {
		t.Fatalf("invalid text/structured result agreement: %v", result.Content)
	}
	return output["data"]
}

func TestMCPCatalogScopesAndValidatedSchemas(t *testing.T) {
	_, _, _ = setupExpirationAPI(t)
	owner, _ := store.GetUserByUsername("test-admin")
	viewer, _ := teamUser(t, "mcp-catalog-viewer", "viewer")
	ownerPrincipal := mcpTestPrincipal(owner, middleware.AuthTypeJWT, store.PermissionsForRole(owner.Role))
	viewerPrincipal := mcpTestPrincipal(viewer, middleware.AuthTypeJWT, []string{"read"})
	tokenPrincipal := mcpTestPrincipal(owner, middleware.AuthTypeToken, []string{"read"})
	allScopes := mcpTestPrincipal(owner, middleware.AuthTypeToken, []string{"read", "write", "delete", "manage"})
	seen := map[string]bool{}
	for _, definition := range mcpToolCatalog() {
		if seen[definition.name] || definition.resolved == nil || definition.router == nil {
			t.Fatalf("invalid catalog entry: %s", definition.name)
		}
		seen[definition.name] = true
		if !mcpToolAllowed(ownerPrincipal, definition) {
			t.Fatalf("owner cannot call %s", definition.name)
		}
		if definition.permission == "manage" && (mcpToolAllowed(viewerPrincipal, definition) || mcpToolAllowed(tokenPrincipal, definition)) {
			t.Fatalf("management tool visible to reader: %s", definition.name)
		}
		if mcpToolAllowed(allScopes, definition) == definition.jwtOnly {
			t.Fatalf("full MCP token/JWT-only filtering incorrect: %s", definition.name)
		}
	}
	if len(seen) != 39 {
		t.Fatalf("expected full REST coverage (39 tools), got %d", len(seen))
	}
	for _, input := range []struct {
		tool string
		args any
	}{
		{"list_files", gin.H{"limit": 101}},
		{"list_files", gin.H{"limit": "50"}},
		{"list_files", gin.H{"endpoint": "http://unexpected.invalid"}},
		{"upload_content", gin.H{"filename": "test.txt"}},
		{"upload_content", gin.H{"filename": "test.txt", "contentBase64": "", "requestId": strings.Repeat("a", 129)}},
		{"get_account", gin.H{"id": "../users"}},
		{"upload_content", gin.H{"filename": "test.txt", "contentBase64": "invalid!"}},
		{"upload_content", gin.H{"filename": "test.txt", "contentBase64": "", "contentType": "text/plain\r\nX-Injected: yes"}},
	} {
		mcpResultStatus(t, mcpTestCall(t, ownerPrincipal, input.tool, input.args), http.StatusBadRequest)
	}
	mcpResultStatus(t, mcpTestCall(t, viewerPrincipal, "create_user", gin.H{"username": "new", "password": "password", "role": "member"}), http.StatusForbidden)
	mcpResultStatus(t, mcpTestCall(t, tokenPrincipal, "logout", gin.H{}), http.StatusForbidden)
}

func TestMCPMetadataRedactionAndCredentialCreation(t *testing.T) {
	_, _, _ = setupExpirationAPI(t)
	owner, _ := store.GetUserByUsername("test-admin")
	principal := mcpTestPrincipal(owner, middleware.AuthTypeJWT, store.PermissionsForRole(owner.Role))
	account, _ := relayTestAccount(t)
	for _, input := range []struct {
		tool string
		args any
	}{
		{"list_accounts", gin.H{}},
		{"list_accounts", gin.H{"page": 1, "pageSize": 5}},
		{"get_account", gin.H{"id": account.ID}},
	} {
		data := mcpResultStatus(t, mcpTestCall(t, principal, input.tool, input.args), http.StatusOK)
		encoded, _ := json.Marshal(data)
		for _, secret := range []string{"secretAccessKey", "accessKeyId", "apiToken", "mock-secret", "mock-access"} {
			if strings.Contains(string(encoded), secret) {
				t.Fatalf("%s leaked %s: %s", input.tool, secret, encoded)
			}
		}
	}
	data := mcpResultStatus(t, mcpTestCall(t, principal, "get_account_credentials", gin.H{"id": account.ID}), http.StatusOK)
	if data.(map[string]any)["secretAccessKey"] != "mock-secret" {
		t.Fatal("explicit owner credentials tool omitted requested credentials")
	}
	data = mcpResultStatus(t, mcpTestCall(t, principal, "create_token", gin.H{"name": "mcp-reader", "permissions": []string{"read"}}), http.StatusCreated)
	tokenValue, ok := data.(map[string]any)["token"].(string)
	if !ok || tokenValue == "" {
		t.Fatal("created token secret not returned")
	}
	listed := mcpResultStatus(t, mcpTestCall(t, principal, "list_tokens", gin.H{}), http.StatusOK)
	encoded, _ := json.Marshal(listed)
	if strings.Contains(string(encoded), tokenValue) || strings.Contains(string(encoded), `"token"`) {
		t.Fatalf("token list exposed secret: %s", encoded)
	}
	credential := &store.WebDAVCredential{UserID: owner.ID, AccountID: account.ID, Permissions: []string{"read"}, Password: "secret-webdav-password"}
	if err := store.CreateWebDAVCredential(credential); err != nil {
		t.Fatal(err)
	}
	listed = mcpResultStatus(t, mcpTestCall(t, principal, "list_webdav_credentials", gin.H{}), http.StatusOK)
	encoded, _ = json.Marshal(listed)
	if strings.Contains(string(encoded), credential.Password) || strings.Contains(string(encoded), `"password"`) {
		t.Fatalf("WebDAV list exposed secret: %s", encoded)
	}
}

func TestMCPInlineRelayPreservesIdentityQuotaAndAudit(t *testing.T) {
	_, _, _ = setupExpirationAPI(t)
	owner, _ := store.GetUserByUsername("test-admin")
	token := &store.Token{UserID: owner.ID, Name: "mcp-upload", Permissions: []string{"read", "write", "delete"}}
	if err := store.CreateToken(token); err != nil {
		t.Fatal(err)
	}
	principal := mcpTestPrincipal(owner, middleware.AuthTypeToken, token.Permissions)
	principal.Set(middleware.ContextKeyTokenID, token.ID)
	account, mock := relayTestAccount(t)
	content := "Agent file content\n"
	data := mcpResultStatus(t, mcpTestCall(t, principal, "upload_content", gin.H{
		"filename": "agent.txt", "contentBase64": base64.StdEncoding.EncodeToString([]byte(content)),
		"contentType": "text/plain", "idGroup": account.ID, "path": "mcp", "randomFilename": false, "expirationDays": 0,
	}), http.StatusOK)
	key := data.(map[string]any)["key"].(string)
	if !strings.HasSuffix(key, "mcp/agent.txt") {
		t.Fatalf("original file name/path lost: %s", key)
	}
	mock.Lock()
	object, exists := mock.objects[key]
	puts, heads := mock.puts, mock.heads
	mock.Unlock()
	if !exists || string(object.body) != content || object.contentType != "text/plain" || puts != 1 || heads < 1 {
		t.Fatalf("MCP upload bypassed relay contract: %+v puts=%d heads=%d", object, puts, heads)
	}
	usage, _ := store.GetAccountByID(account.ID)
	if usage.Usage.ClassAOps < 1 || usage.Usage.ClassBOps < 1 || usage.Usage.SizeBytes != int64(len(content)) {
		t.Fatalf("MCP relay did not use shared quota ledger: %+v", usage.Usage)
	}
	audits := store.GetAuditEntries(10)
	if len(audits) == 0 || audits[0].UserID != owner.ID || audits[0].Status != http.StatusOK || !strings.Contains(audits[0].Target, key) {
		t.Fatalf("MCP upload audit lost identity/target: %+v", audits)
	}
	// Same-name upload must keep existing bytes unchanged.
	mcpResultStatus(t, mcpTestCall(t, principal, "upload_content", gin.H{"filename": "agent.txt", "contentBase64": "", "idGroup": account.ID, "path": "mcp", "randomFilename": false}), http.StatusConflict)
	// Public-URL relay must keep the REST handler's SSRF protection.
	mcpResultStatus(t, mcpTestCall(t, principal, "upload_url", gin.H{"url": "http://127.0.0.1/private", "idGroup": account.ID}), http.StatusBadRequest)
}

func TestMCPScheduleUpdateUsesMappedIDAndLocalMetadata(t *testing.T) {
	_, _, _ = setupExpirationAPI(t)
	owner, _ := store.GetUserByUsername("test-admin")
	principal := mcpTestPrincipal(owner, middleware.AuthTypeJWT, store.PermissionsForRole(owner.Role))
	entry := createExpiration(t, "account", "file.txt")
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	mcpResultStatus(t, mcpTestCall(t, principal, "update_expiration", gin.H{"id": entry.ID, "expiresAt": future}), http.StatusOK)
	updated, err := store.GetFileExpirationByID(entry.ID)
	if err != nil || updated.ExpiresAt != future {
		t.Fatalf("MCP path ID/body mapping failed: %+v %v", updated, err)
	}
	mcpResultStatus(t, mcpTestCall(t, principal, "list_expirations", gin.H{}), http.StatusOK)
}

func TestMCPQueryIntegerParametersPreserveDecimalFormat(t *testing.T) {
	definition := &mcpToolDefinition{method: "GET", route: "/api/accounts"}
	request, cleanup, err := mcpToolHTTPRequest(context.Background(), definition, map[string]any{"page": float64(1000000), "pageSize": float64(100)})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if request.URL.Query().Get("page") != "1000000" || request.URL.Query().Get("pageSize") != "100" {
		t.Fatalf("numeric query parameters used exponent notation: %s", request.URL.RawQuery)
	}
}

func TestMCPServerUploadRetryUsesStableRequestID(t *testing.T) {
	_, _, _ = setupExpirationAPI(t)
	owner, _ := store.GetUserByUsername("test-admin")
	principal := mcpTestPrincipal(owner, middleware.AuthTypeJWT, store.PermissionsForRole(owner.Role))
	account, mock := relayTestAccount(t)
	arguments := gin.H{"filename": "retry.txt", "contentBase64": base64.StdEncoding.EncodeToString([]byte("same file")), "idGroup": account.ID, "requestId": "mcp-same-file-retry", "expirationDays": 0}
	first := mcpResultStatus(t, mcpTestCall(t, principal, "upload_content", arguments), http.StatusOK).(map[string]any)
	usageBefore, _ := store.GetAccountByID(account.ID)
	auditsBefore := len(store.GetAuditEntries(100))
	second := mcpResultStatus(t, mcpTestCall(t, principal, "upload_content", arguments), http.StatusOK).(map[string]any)
	usageAfter, _ := store.GetAccountByID(account.ID)
	mock.Lock()
	puts := mock.puts
	objectCount := len(mock.objects)
	mock.Unlock()
	if first["key"] != second["key"] || puts != 1 || objectCount != 1 || usageBefore.Usage != usageAfter.Usage || len(store.GetAuditEntries(100)) != auditsBefore {
		t.Fatalf("idempotent MCP retry duplicated file/quota/audit: first=%v second=%v puts=%d objects=%d usage=%+v/%+v", first, second, puts, objectCount, usageBefore.Usage, usageAfter.Usage)
	}
	arguments["contentBase64"] = base64.StdEncoding.EncodeToString([]byte("different file"))
	mcpResultStatus(t, mcpTestCall(t, principal, "upload_content", arguments), http.StatusConflict)
}
