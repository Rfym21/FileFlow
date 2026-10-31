package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"fileflow/server/middleware"
	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	mcpFileDefinitionsOnce sync.Once
	mcpFileDefinitions     []*mcpToolDefinition
)

func mcpFileOperationCatalog() []*mcpToolDefinition {
	mcpFileDefinitionsOnce.Do(func() {
		file := map[string]any{
			"idGroup":        mcpID("One FileFlow storage account ID; source and destination use the same bucket"),
			"key":            mcpString("Exact source file key; directories are not supported", 1, 1024),
			"destinationKey": mcpString("Exact destination file key; existing objects are never overwritten", 1, 1024),
		}
		mcpFileDefinitions = []*mcpToolDefinition{
			{name: "fileflow_copy_file", description: "Copy one file within a bucket, preserving its exact expiration. Requires read and write. Files are at most 5 GiB; directories require paginated listing and individual file calls. Streams a conditional source GET through bounded temporary disk storage to a conditional destination PUT. Authorizes one Class A PUT and four Class B HEAD/GET requests. Never overwrites the destination.", requiredPermissions: []string{"read", "write"}, method: "POST", route: "/api/mcp/copy-file", handler: MCPCopyFile, inputSchema: mcpObject(file, "idGroup", "key", "destinationKey"), body: true},
			{name: "fileflow_move_file", description: "Move or rename one file within a bucket. Requires read, write and delete. Copies without overwrite, confirms the destination and preserves expiration, then rechecks the source ETag immediately before deletion. R2 lacks conditional deletion: independent storage clients can race the final HEAD/DELETE window. Capacity must temporarily fit a full copy. Uses one Class A PUT and five Class B reads. Partial failure returns result.destination, result.destinationConfirmed and result.sourceDeleted; inspect both keys before recovery. Local storage size stays conservative until usage synchronization.", requiredPermissions: []string{"read", "write", "delete"}, method: "POST", route: "/api/mcp/move-file", handler: MCPMoveFile, inputSchema: mcpObject(file, "idGroup", "key", "destinationKey"), body: true, destructive: true},
			{name: "fileflow_create_directory", description: "Create a zero-byte permanent directory marker. Requires write. key may include or omit the final slash; root and dot path segments are invalid. Existing directory markers are never overwritten. Uses a reserved Class A PUT and Class B HEAD checks.", requiredPermissions: []string{"write"}, method: "POST", route: "/api/mcp/create-directory", handler: MCPCreateDirectory, inputSchema: mcpObject(map[string]any{"idGroup": mcpID("FileFlow storage account ID"), "key": mcpString("Directory key, e.g. documents/projects/", 1, 1024)}, "idGroup", "key"), body: true},
		}
		for _, definition := range mcpFileDefinitions {
			encoded, err := json.Marshal(definition.inputSchema)
			if err != nil {
				panic(err)
			}
			var schema jsonschema.Schema
			if err := json.Unmarshal(encoded, &schema); err != nil {
				panic(err)
			}
			definition.resolved, err = schema.Resolve(nil)
			if err != nil {
				panic(fmt.Errorf("MCP schema %s: %w", definition.name, err))
			}
			definition.router = mcpDispatchRouter(definition)
		}
	})
	return mcpFileDefinitions
}

func registerMCPFileOperationTools(server *mcp.Server, principal *gin.Context) {
	for _, definition := range mcpFileOperationCatalog() {
		if !mcpToolAllowed(principal, definition) {
			continue
		}
		server.AddTool(&mcp.Tool{
			Name: definition.name, Description: definition.description, InputSchema: definition.inputSchema,
			OutputSchema: mcpObject(map[string]any{"status": map[string]any{"type": "integer"}, "data": map[string]any{}}, "status", "data"),
			Annotations:  &mcp.ToolAnnotations{DestructiveHint: &definition.destructive},
		}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return executeMCPTool(ctx, definition, request.Params.Arguments), nil
		})
	}
}

func mcpFileOperationPermissions(c *gin.Context, permissions ...string) bool {
	if middleware.CurrentUser(c) == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证失效"})
		return false
	}
	for _, permission := range permissions {
		if !middleware.HasPermission(c, permission) {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足: 需要 " + permission})
			return false
		}
	}
	return true
}

func mcpFileOperationError(c *gin.Context, result *service.MCPFileOperationResult, err error) {
	status, code := http.StatusInternalServerError, "file_operation_failed"
	switch {
	case errors.Is(err, service.ErrMCPFileNotFound):
		status, code = http.StatusNotFound, "file_not_found"
	case errors.Is(err, service.ErrMCPFileExpired):
		status, code = http.StatusGone, "file_expired"
	case errors.Is(err, store.ErrInvalidUploadSession):
		status, code = http.StatusBadRequest, "invalid_file_operation"
	case errors.Is(err, store.ErrUploadSessionForbidden):
		status, code = http.StatusForbidden, "permission_denied"
	case errors.Is(err, store.ErrUploadSessionConflict):
		status, code = http.StatusConflict, "file_conflict"
	case errors.Is(err, store.ErrUploadQuotaExceeded):
		status, code = http.StatusTooManyRequests, "quota_exceeded"
	case errors.Is(err, store.ErrUploadSessionLimit):
		status, code = http.StatusTooManyRequests, "upload_task_limit"
	}
	response := gin.H{"error": err.Error(), "code": code}
	if result != nil {
		response["result"] = result
		c.Set("audit_target", result.Destination.ID+"/"+result.SourceKey+" → "+result.Destination.Key)
	}
	c.JSON(status, response)
}

func MCPCopyFile(c *gin.Context) { handleMCPCopyFile(c, false) }
func MCPMoveFile(c *gin.Context) { handleMCPCopyFile(c, true) }

func handleMCPCopyFile(c *gin.Context, move bool) {
	c.Header("Cache-Control", "private, no-store")
	permissions := []string{"read", "write"}
	if move {
		permissions = append(permissions, "delete")
	}
	if !mcpFileOperationPermissions(c, permissions...) {
		return
	}
	var request service.MCPFileOperationRequest
	if !decodeUploadRequest(c, &request) {
		return
	}
	c.Set("audit_target", request.IDGroup+"/"+request.Key+" → "+request.DestinationKey)
	result, err := service.CopyMCPFile(c.Request.Context(), uploadActor(c), request, move)
	if err != nil {
		mcpFileOperationError(c, result, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func MCPCreateDirectory(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	if !mcpFileOperationPermissions(c, "write") {
		return
	}
	var request service.MCPFileOperationRequest
	if !decodeUploadRequest(c, &request) {
		return
	}
	c.Set("audit_target", request.IDGroup+"/"+request.Key)
	result, err := service.CreateMCPDirectory(c.Request.Context(), uploadActor(c), request)
	if err != nil {
		mcpFileOperationError(c, result, err)
		return
	}
	c.Set("audit_target", result.Destination.ID+"/"+result.Destination.Key)
	c.JSON(http.StatusOK, result)
}
