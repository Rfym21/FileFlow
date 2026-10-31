package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"fileflow/server/middleware"
	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

func TestMCPNativeFileOperationDiscoveryAndPermissionGuards(t *testing.T) {
	_, _, _ = setupExpirationAPI(t)
	owner, _ := store.GetUserByUsername("test-admin")
	catalog := mcpFileOperationCatalog()
	if len(catalog) != 3 {
		t.Fatal("native operation catalog is incomplete")
	}
	for _, scopes := range [][]string{{"read"}, {"write"}, {"read", "write"}, {"read", "write", "delete"}} {
		principal := mcpTestPrincipal(owner, middleware.AuthTypeToken, scopes)
		for _, definition := range catalog {
			want := true
			for _, permission := range definition.requiredPermissions {
				found := false
				for _, grant := range scopes {
					found = found || grant == permission
				}
				want = want && found
			}
			if mcpToolAllowed(principal, definition) != want {
				t.Fatalf("tool discovery does not intersect all scopes for %s: %v", definition.name, scopes)
			}
			if !want {
				ctx := context.WithValue(context.Background(), mcpPrincipalKey{}, principal)
				args, _ := json.Marshal(gin.H{"idGroup": "account", "key": "source.txt", "destinationKey": "copy.txt"})
				mcpResultStatus(t, executeMCPTool(ctx, definition, args), http.StatusForbidden)
			}
		}
	}
	principal := mcpTestPrincipal(owner, middleware.AuthTypeJWT, store.PermissionsForRole(owner.Role))
	for _, definition := range catalog {
		ctx := context.WithValue(context.Background(), mcpPrincipalKey{}, principal)
		args := gin.H{"idGroup": "account", "key": "../invalid"}
		if definition.name != "fileflow_create_directory" {
			args["destinationKey"] = "copy.txt"
		}
		encoded, _ := json.Marshal(args)
		mcpResultStatus(t, executeMCPTool(ctx, definition, encoded), http.StatusBadRequest)
	}
}

func TestMCPNativeFileOperationErrorRetainsPartialRecoveryState(t *testing.T) {
	router := gin.New()
	partial := &service.MCPFileOperationResult{DestinationConfirmed: true, Destination: &service.UploadResult{ID: "account", Key: "copy.txt", Size: 6}, SourceKey: "source.txt"}
	router.GET("/partial", func(c *gin.Context) { mcpFileOperationError(c, partial, store.ErrUploadSessionConflict) })
	response := expirationRequest(t, router, "GET", "/partial", nil, "")
	var output struct {
		Result service.MCPFileOperationResult `json:"result"`
		Code   string                         `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusConflict || output.Result.Destination.Key != "copy.txt" || !output.Result.DestinationConfirmed || output.Result.SourceDeleted || output.Code != "file_conflict" {
		t.Fatalf("partial recovery information lost: %d %s", response.Code, response.Body.String())
	}
}
