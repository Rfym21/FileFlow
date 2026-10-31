package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fileflow/server/config"
	"fileflow/server/store"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mcpRPC(t *testing.T, handler http.Handler, token, method string, params any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2025-06-18")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

type mcpBearerTransport struct {
	base  http.RoundTripper
	token string
}

func (transport mcpBearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+transport.token)
	return transport.base.RoundTrip(request)
}

func TestMCPStreamableHTTPClientAndGuide(t *testing.T) {
	router, jwt, _ := setupExpirationAPI(t)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "FileFlow integration test", Version: "1"}, &mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}})
	httpClient := &http.Client{Transport: mcpBearerTransport{base: server.Client().Transport, token: jwt}}
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: httpClient}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) < 35 {
		t.Fatalf("tools discovery: %v, %v", tools, err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "fileflow_check", Arguments: map[string]any{}})
	if err != nil || result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "test-admin") {
		t.Fatalf("check tool: %v, %v", result, err)
	}
	resources, err := session.ListResources(ctx, nil)
	if err != nil || len(resources.Resources) != 1 || resources.Resources[0].URI != "fileflow://guide" {
		t.Fatalf("resource discovery: %v, %v", resources, err)
	}
	guide, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "fileflow://guide"})
	if err != nil || len(guide.Contents) != 1 || !strings.Contains(guide.Contents[0].Text, "5 GiB") {
		t.Fatalf("guide: %v, %v", guide, err)
	}
}

func TestMCPHTTPAuthenticationAndStatelessIsolation(t *testing.T) {
	router, ownerJWT, _ := setupExpirationAPI(t)
	if response := mcpRPC(t, router, "", "tools/list", map[string]any{}); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous discovery: %d", response.Code)
	}
	owner, _ := store.GetUserByUsername("test-admin")
	manager := &store.Token{Name: "full-agent", UserID: owner.ID, Permissions: []string{"read", "write", "delete", "manage"}}
	if err := store.CreateToken(manager); err != nil {
		t.Fatal(err)
	}
	reader, _ := teamUser(t, "mcp-reader-transport", "viewer")
	readToken := &store.Token{Name: "reader-agent", UserID: reader.ID, Permissions: []string{"read"}}
	if err := store.CreateToken(readToken); err != nil {
		t.Fatal(err)
	}
	for _, credential := range []string{ownerJWT, manager.Token, readToken.Token, manager.Token, readToken.Token} {
		response := mcpRPC(t, router, credential, "tools/list", map[string]any{})
		if response.Code != http.StatusOK {
			t.Fatalf("list: %d %s", response.Code, response.Body.String())
		}
		isReader := credential == readToken.Token
		if strings.Contains(response.Body.String(), `"name":"fileflow_create_user"`) == isReader {
			t.Fatalf("cross-caller management discovery: %s", response.Body.String())
		}
		if response.Header().Get("Mcp-Session-Id") != "" || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("unsafe stateless cache headers: %v", response.Header())
		}
	}
	response := mcpRPC(t, router, readToken.Token, "tools/call", map[string]any{"name": "fileflow_create_user", "arguments": map[string]any{"username": "unexpected", "password": "member-password", "role": "member"}})
	if strings.Contains(response.Body.String(), `"status":201`) {
		t.Fatal("reader invoked a hidden management tool")
	}
	if _, err := store.GetUserByUsername("unexpected"); err == nil {
		t.Fatal("forbidden tool mutated members")
	}
	if err := store.DeleteToken(manager.ID); err != nil {
		t.Fatal(err)
	}
	if response := mcpRPC(t, router, manager.Token, "tools/list", map[string]any{}); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token retained stateless permissions: %d", response.Code)
	}
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		request := httptest.NewRequest(method, "/mcp", nil)
		request.Header.Set("Authorization", "Bearer "+ownerJWT)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("stateless %s: %d", method, response.Code)
		}
	}
}

func TestMCPOriginAndPayloadLimits(t *testing.T) {
	router, jwt, _ := setupExpirationAPI(t)
	previous := config.Get().MCPAllowedOrigins
	config.Get().MCPAllowedOrigins = "https://agent.example"
	t.Cleanup(func() { config.Get().MCPAllowedOrigins = previous })
	for _, test := range []struct {
		origin string
		status int
	}{
		{"", http.StatusOK},
		{"http://example.com", http.StatusOK},
		{"https://agent.example", http.StatusOK},
		{"https://evil.example", http.StatusForbidden},
		{"null", http.StatusForbidden},
		{"http://user:pass@example.com", http.StatusForbidden},
		{"http://example.com/path", http.StatusForbidden},
	} {
		request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		request.Header.Set("Origin", test.origin)
		request.Header.Set("Authorization", "Bearer "+jwt)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Errorf("Origin %q: %d %s", test.origin, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(strings.Repeat(" ", (12<<20)+1)))
	request.ContentLength = -1 // Also enforce the limit on chunked/unknown-size requests.
	request.Header.Set("Authorization", "Bearer "+jwt)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized unknown-length payload: %d %s", response.Code, response.Body.String())
	}
}

func TestMCPLoopbackRebindingAndTrustedProxyHost(t *testing.T) {
	router, jwt, _ := setupExpirationAPI(t)
	for _, test := range []struct {
		host, allowedHosts string
		status             int
	}{
		{"127.0.0.1:8188", "", http.StatusOK},
		{"localhost:8188", "", http.StatusOK},
		{"evil.example", "", http.StatusForbidden},
		{"files.example", "files.example", http.StatusOK},
		{"evil.example", "files.example", http.StatusForbidden},
		{"evil.example", "*", http.StatusForbidden},
		{"files.example:8443", "files.example", http.StatusForbidden},
		{"files.example:8443", "files.example:8443", http.StatusOK},
	} {
		config.Get().MCPAllowedHosts = test.allowedHosts
		request := httptest.NewRequest(http.MethodPost, "http://"+test.host+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		request = request.WithContext(context.WithValue(request.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8188}))
		request.Header.Set("Authorization", "Bearer "+jwt)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Errorf("loopback Host %q configured %q: %d %s", test.host, test.allowedHosts, response.Code, response.Body.String())
		}
	}
}
