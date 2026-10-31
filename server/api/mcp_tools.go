package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"fileflow/server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpMaxContentBytes = 8 * 1024 * 1024

// Tools dispatch to the same handlers as the web application. Their isolated
// routers carry the current caller's identity, so upload ownership, quotas and
// mutation auditing apply equally to REST and MCP without self HTTP requests.
type mcpToolDefinition struct {
	name                string
	description         string
	permission          string
	requiredPermissions []string
	method              string
	route               string
	handler             gin.HandlerFunc
	inputSchema         map[string]any
	body                bool
	uploadMode          string
	jwtOnly             bool
	includeSecrets      bool
	destructive         bool
	idempotent          bool
	resolved            *jsonschema.Resolved
	router              *gin.Engine
}

var (
	mcpDefinitionsOnce sync.Once
	mcpDefinitions     []*mcpToolDefinition
)

func mcpString(description string, min, max int) map[string]any {
	return map[string]any{"type": "string", "description": description, "minLength": min, "maxLength": max}
}

func mcpID(description string) map[string]any {
	s := mcpString(description, 1, 128)
	s["pattern"] = `^[A-Za-z0-9._-]+$`
	return s
}

func mcpInteger(description string, min, max int64) map[string]any {
	return map[string]any{"type": "integer", "description": description, "minimum": min, "maximum": max}
}

func mcpBool(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func mcpEnum(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

func mcpArray(description string, items any, min, max int) map[string]any {
	return map[string]any{"type": "array", "description": description, "items": items, "minItems": min, "maxItems": max, "uniqueItems": true}
}

func mcpObject(properties map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func mcpProperties(groups ...map[string]any) map[string]any {
	p := map[string]any{}
	for _, group := range groups {
		for key, value := range group {
			p[key] = value
		}
	}
	return p
}

func mcpToolCatalog() []*mcpToolDefinition {
	mcpDefinitionsOnce.Do(func() {
		id := map[string]any{"id": mcpID("FileFlow record ID, not a Cloudflare account ID")}
		file := map[string]any{
			"idGroup": mcpID("One FileFlow storage account ID"),
			"key":     mcpString("Full object key including its directory path", 1, 1024),
		}
		keys := map[string]any{
			"accountId": mcpID("FileFlow storage account ID"),
			"keys":      mcpArray("Exact object keys; at most 100 per operation", mcpString("Object key", 1, 1024), 1, 100),
		}
		ids := map[string]any{"ids": mcpArray("Expiration record IDs; at most 100 per operation", mcpID("Expiration record ID"), 1, 100)}
		expires := map[string]any{"expiresAt": mcpString("Future RFC3339 timestamp with time zone", 1, 64)}
		upload := map[string]any{
			"path":           mcpString("Optional destination directory prefix, at most 800 UTF-8 bytes", 0, 800),
			"idGroup":        mcpID("Optional FileFlow storage account ID; omit for automatic selection"),
			"expirationDays": mcpInteger("-1 or omitted uses defaults; 0 is permanent; positive values are days", -1, 3650),
			"randomFilename": mcpBool("Defaults to true; false preserves the original name and refuses overwrite"),
			"requestId":      mcpString("Optional stable client request ID; reusing it with the same file safely retries upload", 1, 128),
		}
		content := mcpProperties(upload, map[string]any{
			"filename":      mcpString("Original file name, at most 255 UTF-8 bytes", 1, 255),
			"contentBase64": mcpString("Standard padded base64 of file bytes, maximum 8 MiB decoded", 0, ((mcpMaxContentBytes+2)/3)*4),
			"contentType":   mcpString("MIME type, defaults to application/octet-stream", 0, 255),
		})
		account := map[string]any{
			"name":            mcpString("Storage account display name", 1, 160),
			"isActive":        mcpBool("Whether uploads and normal access are enabled"),
			"description":     mcpString("Storage account description", 0, 2000),
			"accountId":       mcpString("Cloudflare account ID", 1, 128),
			"accessKeyId":     mcpString("R2 access key; required to create, omitted or empty preserves on update", 0, 512),
			"secretAccessKey": mcpString("R2 secret key; required to create, omitted or empty preserves on update", 0, 512),
			"bucketName":      mcpString("R2 bucket name", 1, 255),
			"endpoint":        mcpString("R2 S3 endpoint URL", 1, 2048),
			"publicDomain":    mcpString("Public object domain", 1, 2048),
			"apiToken":        mcpString("Optional Cloudflare usage API token", 0, 1024),
			"quota": mcpObject(map[string]any{
				"maxSizeBytes": mcpInteger("Storage capacity limit in bytes", 0, 1<<53-1),
				"maxClassAOps": mcpInteger("UTC monthly Class A request limit; listing/search also count", 0, 1<<53-1),
				"maxClassBOps": mcpInteger("UTC monthly Class B request limit; 0 keeps unlimited reads", 0, 1<<53-1),
			}, "maxSizeBytes", "maxClassAOps", "maxClassBOps"),
			"permissions": mcpObject(map[string]any{
				"webdav":       mcpBool("Enable WebDAV"),
				"autoUpload":   mcpBool("Permit automatic upload account selection"),
				"apiUpload":    mcpBool("Permit API Token uploads including MCP"),
				"clientUpload": mcpBool("Permit member session uploads"),
			}),
		}
		accountRequired := []string{"name", "isActive", "accountId", "bucketName", "endpoint", "publicDomain", "quota", "permissions"}
		settings := map[string]any{
			"syncInterval":           mcpInteger("Usage synchronization interval in minutes", 1, 1440),
			"endpointProxy":          mcpBool("Enable public-link endpoint proxy"),
			"endpointProxyUrl":       mcpString("Proxy URL prefix", 0, 2048),
			"defaultExpirationDays":  mcpInteger("Default file lifetime in days; 0 is permanent", 0, 3650),
			"expirationCheckMinutes": mcpInteger("Expired file cleanup interval in minutes", 60, 1440),
		}
		webdavPerms := mcpArray("Permissions must not exceed the credential's member or the calling token", mcpEnum("Permission", "read", "write", "delete"), 0, 3)
		tokenPerms := mcpArray("Permissions must not exceed the associated member or caller; manage only applies to MCP", mcpEnum("Permission", "read", "write", "delete", "manage"), 1, 4)
		add := func(name, description, permission, method, route string, handler gin.HandlerFunc, schema map[string]any, body, destructive bool) *mcpToolDefinition {
			d := &mcpToolDefinition{name: "fileflow_" + name, description: description, permission: permission, method: method, route: route, handler: handler, inputSchema: schema, body: body, destructive: destructive}
			mcpDefinitions = append(mcpDefinitions, d)
			return d
		}
		add("check", "Get the authenticated member profile, auth type and current effective permissions. Does not call R2.", "", "GET", "/api/check", Check, mcpObject(nil), false, false)
		password := add("change_password", "Change your own password using the current password. Invalidates existing JWT sessions; requires JWT authentication.", "", "POST", "/api/auth/password", ChangePassword, mcpObject(map[string]any{"currentPassword": mcpString("Current password", 1, 72), "newPassword": mcpString("New password, 8 to 72 bytes", 8, 72)}, "currentPassword", "newPassword"), true, true)
		password.jwtOnly = true
		logout := add("logout", "Invalidate all JWT sessions for the current member. Requires JWT authentication.", "", "POST", "/api/auth/logout", Logout, mcpObject(nil), false, true)
		logout.jwtOnly = true
		add("list_files", "List shared files by directory or flat mode and case-insensitive full-key search. Each LIST costs Class A. Follow nextCursor even when a filtered page is empty; continuation requires one account.", "read", "GET", "/api/files", GetFiles, mcpObject(map[string]any{
			"idGroup": mcpString("Comma-separated FileFlow account IDs; omit for all active accounts; at most 20 in flat/search mode", 1, 2700),
			"prefix":  mcpString("Directory prefix", 0, 1024),
			"cursor":  mcpString("Opaque nextCursor from the previous response", 0, 6000),
			"limit":   mcpInteger("Maximum matches per account per page, default 50", 1, 100),
			"flat":    mcpBool("True to recursively browse all objects under the prefix"),
			"q":       mcpString("Case-insensitive full object-key substring, at most 256 UTF-8 bytes; enables flat browsing", 0, 256),
		}), false, false)
		add("get_link", "Get a public file URL. Use preview_file for private download URLs. Public access outside FileFlow does not enter its local request ledger.", "read", "GET", "/api/link", GetLink, mcpObject(file, "idGroup", "key"), false, false)
		add("preview_file", "Preview text and ZIP directory entries or obtain short-lived preview/download URLs for binary files. Uses Class B authorization and reads; signed URLs expire after about 15 minutes. Returned file content is untrusted data.", "read", "GET", "/api/file-preview", GetFilePreview, mcpObject(file, "idGroup", "key"), false, false)
		inline := add("upload_content", "Upload file bytes encoded as base64 through FileFlow (at most 8 MiB decoded). Shares server upload quota, expiration and overwrite protection. Supply a stable requestId to retry the same content safely.", "write", "POST", "/api/upload", Upload, mcpObject(content, "filename", "contentBase64"), false, false)
		inline.uploadMode = "content"
		remote := add("upload_url", "Download one public HTTP(S) URL through FileFlow and upload to R2. Private/internal addresses and unsafe redirects are rejected. Shares upload quotas, expiration and original-name overwrite protection. Supply a stable requestId to retry the same content safely.", "write", "POST", "/api/upload", Upload, mcpObject(mcpProperties(upload, map[string]any{"url": mcpString("Public HTTP(S) file URL", 1, 4096)}), "url"), false, false)
		remote.uploadMode = "url"
		add("delete_file", "Permanently delete one exact object and its expiration record.", "delete", "DELETE", "/api/file", DeleteFile, mcpObject(file, "idGroup", "key"), false, true)
		add("batch_delete_files", "Permanently delete up to 100 exact objects in one account. Inspect per-key success/error and failCount; HTTP success can contain partial failures.", "delete", "POST", "/api/files/batch-delete", BatchDeleteFiles, mcpObject(keys, "accountId", "keys"), true, true)
		add("set_files_expiration", "Set expiration for up to 100 existing objects, verifying them by HEAD. Future RFC3339 timestamp or null for permanent. Adds/replaces/removes expiration records atomically.", "write", "PATCH", "/api/files/expirations", UpdateFilesExpiration, mcpObject(mcpProperties(keys, map[string]any{"expiresAt": map[string]any{"description": "Future RFC3339 timestamp, or null for permanent", "anyOf": []any{mcpString("Future timestamp with time zone", 1, 64), map[string]any{"type": "null"}}}}), "accountId", "keys", "expiresAt"), true, true)
		add("list_expirations", "Get scheduled deletion records including file path and storage account name. Local metadata only; does not call R2.", "read", "GET", "/api/file-expirations", GetFileExpirations, mcpObject(nil), false, false)
		add("update_expiration", "Update one existing scheduled deletion to a future RFC3339 time. Use set_files_expiration with null to make the file permanent.", "write", "PATCH", "/api/file-expirations/:id", UpdateFileExpiration, mcpObject(mcpProperties(id, expires), "id", "expiresAt"), true, true)
		add("batch_update_expirations", "Atomically update up to 100 existing scheduled deletions to a future RFC3339 time; missing records fail the whole batch.", "write", "PATCH", "/api/file-expirations", UpdateFileExpirations, mcpObject(mcpProperties(ids, expires), "ids", "expiresAt"), true, true)
		add("delete_expiration", "Permanently delete the FILE associated with a scheduled deletion and its record. This does not merely cancel the schedule.", "delete", "DELETE", "/api/file-expirations/:id", DeleteFileExpirationByID, mcpObject(id, "id"), false, true)
		add("batch_delete_expirations", "Permanently delete up to 100 files referenced by scheduled deletion records. Inspect per-record failures; unsuccessful records remain for retry.", "delete", "POST", "/api/file-expirations/batch-delete", BatchDeleteFileExpirations, mcpObject(ids, "ids"), true, true)
		add("list_accounts", "List shared storage account metadata and quotas, optionally paginated. R2 access credentials are redacted, including for managers; no remote request.", "read", "GET", "/api/accounts", GetAccounts, mcpObject(map[string]any{"page": mcpInteger("One-based page number", 1, 1000000), "pageSize": mcpInteger("Page size", 1, 100)}), false, false)
		add("get_account", "Get one storage account's metadata, quotas and current usage without R2 credentials.", "read", "GET", "/api/accounts/:id", GetAccount, mcpObject(id, "id"), false, false)
		secrets := add("get_account_credentials", "Get one account including its sensitive R2 keys and Cloudflare token. Requires manage plus read/write/delete; avoid unless credentials are specifically needed.", "manage", "GET", "/api/accounts/:id", GetAccount, mcpObject(id, "id"), false, false)
		secrets.includeSecrets = true
		add("account_stats", "Get aggregate account capacity and operation usage stored by FileFlow. Does not refresh R2.", "read", "GET", "/api/accounts/stats", GetAccountsStats, mcpObject(nil), false, false)
		add("create_account", "Create and configure an R2 storage account. Provide quotas and access permissions explicitly. Response redacts keys.", "manage", "POST", "/api/accounts", CreateAccount, mcpObject(account, append(append([]string{}, accountRequired...), "accessKeyId", "secretAccessKey")...), true, false)
		add("update_account", "Replace an account's editable configuration; first get_account to preserve current values. Omitted/empty sensitive fields preserve stored keys. Response redacts keys.", "manage", "PUT", "/api/accounts/:id", UpdateAccount, mcpObject(mcpProperties(id, account), append(append([]string{}, accountRequired...), "id")...), true, true)
		add("delete_account", "Remove an R2 account configuration from FileFlow. Does not delete the actual bucket or its objects.", "manage", "DELETE", "/api/accounts/:id", DeleteAccount, mcpObject(id, "id"), false, true)
		add("sync_accounts", "Synchronize one or all accounts with remote R2 usage. May make LIST/usage requests and update counters conservatively.", "manage", "POST", "/api/accounts/sync", SyncAccounts, mcpObject(map[string]any{"accountId": mcpID("Optional FileFlow account ID; omitted synchronizes all")}), false, false)
		add("clear_bucket", "Permanently delete every completed object in one bucket. Pending authorized uploads remain protected. Listing consumes Class A operations.", "manage", "POST", "/api/accounts/:id/clear", ClearBucket, mcpObject(id, "id"), false, true)
		add("delete_old_files", "Permanently delete objects from selected accounts with last-modified dates up to the end of beforeDate (UTC YYYY-MM-DD). Can return partial failures. Listing consumes Class A operations.", "manage", "POST", "/api/accounts/delete-old-files", DeleteOldFiles, mcpObject(map[string]any{"accountIds": mcpArray("FileFlow storage account IDs", mcpID("Account ID"), 1, 20), "beforeDate": mcpString("UTC date YYYY-MM-DD; includes the entire date", 10, 10)}, "accountIds", "beforeDate"), true, true)
		add("get_settings", "Get persisted system settings. Does not call R2.", "read", "GET", "/api/settings", GetSettings, mcpObject(nil), false, false)
		add("update_settings", "Replace all system settings and reload background schedulers; first get_settings to preserve unchanged values.", "manage", "PUT", "/api/settings", UpdateSettings, mcpObject(settings, "syncInterval", "endpointProxy", "endpointProxyUrl", "defaultExpirationDays", "expirationCheckMinutes"), true, true)
		add("list_users", "List team member profiles without password hashes; owner/admin management rules apply.", "manage", "GET", "/api/users", GetUsers, mcpObject(nil), false, false)
		add("create_user", "Create a team member. Only the owner may create admins; nobody may create another owner. Password must be 8 to 72 bytes.", "manage", "POST", "/api/users", CreateUser, mcpObject(map[string]any{"username": mcpString("3 to 64 ASCII characters: letters, digits, dot, underscore, hyphen", 3, 64), "displayName": mcpString("Member display name", 0, 160), "password": mcpString("Initial password", 8, 72), "role": mcpEnum("Member role", "admin", "member", "viewer")}, "username", "password", "role"), true, false)
		add("update_user", "Update a member display name, role, enabled status or optionally reset their password. Role/password/disable changes revoke JWT sessions. Owner and peer-admin protections apply.", "manage", "PUT", "/api/users/:id", UpdateUser, mcpObject(mcpProperties(id, map[string]any{"displayName": mcpString("Member display name", 0, 160), "role": mcpEnum("Role; owner is only accepted for owner's own display-name edit", "owner", "admin", "member", "viewer"), "isActive": mcpBool("Enable or disable member"), "password": mcpString("Optional replacement password; empty preserves", 0, 72)}), "id", "displayName", "role", "isActive"), true, true)
		add("list_tokens", "List API Token IDs, member ownership and permissions. Existing token secret values are redacted.", "manage", "GET", "/api/tokens", GetTokens, mcpObject(nil), false, false)
		add("create_token", "Create a member-bound API Token and return its secret. Granted scopes cannot exceed the member or calling token; manage is MCP-only and requires an owner/admin member.", "manage", "POST", "/api/tokens", CreateToken, mcpObject(map[string]any{"userId": mcpID("Optional member ID; defaults to yourself"), "name": mcpString("Token name", 1, 160), "permissions": tokenPerms}, "name", "permissions"), true, false)
		add("delete_token", "Revoke one API Token immediately. Owner/peer-admin credential protections apply.", "manage", "DELETE", "/api/tokens/:id", DeleteToken, mcpObject(id, "id"), false, true)
		add("list_webdav_credentials", "List manageable member-bound WebDAV credentials without their passwords.", "manage", "GET", "/api/webdav-credentials", GetWebDAVCredentials, mcpObject(nil), false, false)
		add("create_webdav_credential", "Create credentials for one WebDAV-enabled account and member. Returns the created username/password; permission and member protections apply.", "manage", "POST", "/api/webdav-credentials", CreateWebDAVCredential, mcpObject(map[string]any{"userId": mcpID("Optional member ID; defaults to yourself"), "accountId": mcpID("FileFlow storage account ID"), "description": mcpString("Credential description", 0, 2000), "permissions": webdavPerms, "username": mcpString("Optional username; omit to generate", 1, 160), "password": mcpString("Optional password; omit to generate", 8, 72)}, "accountId", "permissions"), true, false)
		add("update_webdav_credential", "Replace description, permission scopes and enabled status on one WebDAV credential. Permission and member protections apply.", "manage", "PUT", "/api/webdav-credentials/:id", UpdateWebDAVCredential, mcpObject(mcpProperties(id, map[string]any{"description": mcpString("Credential description", 0, 2000), "permissions": webdavPerms, "isActive": mcpBool("Enable or disable credential")}), "id", "description", "permissions", "isActive"), true, true)
		add("delete_webdav_credential", "Revoke one WebDAV credential. Owner/peer-admin credential protections apply.", "manage", "DELETE", "/api/webdav-credentials/:id", DeleteWebDAVCredential, mcpObject(id, "id"), false, true)
		add("audit_logs", "Read recent team mutation audit records. Responses do not include credential bodies or file contents.", "manage", "GET", "/api/audit-logs", GetAuditLogs, mcpObject(map[string]any{"limit": mcpInteger("Maximum recent entries, default 100", 1, 2000)}), false, false)
		for _, definition := range mcpDefinitions {
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
	return mcpDefinitions
}

func mcpToolAllowed(principal *gin.Context, definition *mcpToolDefinition) bool {
	if principal == nil || middleware.CurrentUser(principal) == nil {
		return false
	}
	if definition.jwtOnly && principal.GetString(middleware.ContextKeyAuthType) != middleware.AuthTypeJWT {
		return false
	}
	if definition.permission != "" && !middleware.HasPermission(principal, definition.permission) {
		return false
	}
	for _, permission := range definition.requiredPermissions {
		if !middleware.HasPermission(principal, permission) {
			return false
		}
	}
	if definition.includeSecrets {
		for _, permission := range []string{"read", "write", "delete", "manage"} {
			if !middleware.HasPermission(principal, permission) {
				return false
			}
		}
	}
	return true
}

func registerMCPTools(server *mcp.Server, principal *gin.Context) {
	for _, definition := range mcpToolCatalog() {
		if !mcpToolAllowed(principal, definition) {
			continue
		}
		readOnly := definition.method == http.MethodGet
		server.AddTool(&mcp.Tool{
			Name: definition.name, Description: definition.description,
			InputSchema:  definition.inputSchema,
			OutputSchema: mcpObject(map[string]any{"status": map[string]any{"type": "integer"}, "data": map[string]any{}}, "status", "data"),
			Annotations:  &mcp.ToolAnnotations{ReadOnlyHint: readOnly, DestructiveHint: &definition.destructive, IdempotentHint: definition.idempotent || readOnly},
		}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return executeMCPTool(ctx, definition, request.Params.Arguments), nil
		})
	}
}

func mcpToolResult(status int, data any) *mcp.CallToolResult {
	output := map[string]any{"status": status, "data": data}
	encoded, err := json.Marshal(output)
	if err != nil {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Unable to serialize the tool result"}}}
	}
	return &mcp.CallToolResult{StructuredContent: output, Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}, IsError: status < 200 || status >= 300}
}

func executeMCPTool(ctx context.Context, definition *mcpToolDefinition, arguments json.RawMessage) *mcp.CallToolResult {
	principal, ok := mcpContext(ctx)
	if !ok || !mcpToolAllowed(principal, definition) {
		return mcpToolResult(http.StatusForbidden, map[string]any{"error": "当前成员或令牌无权调用此工具"})
	}
	if err := ctx.Err(); err != nil {
		return mcpToolResult(http.StatusRequestTimeout, map[string]any{"error": "请求已取消"})
	}
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	if len(arguments) > 12*1024*1024 {
		return mcpToolResult(http.StatusRequestEntityTooLarge, map[string]any{"error": "MCP 参数超出大小限制"})
	}
	var values map[string]any
	if err := json.Unmarshal(arguments, &values); err != nil || values == nil {
		return mcpToolResult(http.StatusBadRequest, map[string]any{"error": "工具参数必须为 JSON 对象"})
	}
	if err := definition.resolved.Validate(values); err != nil {
		// Do not echo schema errors: they may contain a password or file content.
		return mcpToolResult(http.StatusBadRequest, map[string]any{"error": "工具参数不符合 inputSchema，请检查必填字段、类型、范围和未知字段"})
	}
	if principal.GetString(middleware.ContextKeyAuthType) == middleware.AuthTypeToken && strings.Contains(definition.route, "credentials") {
		if permissions, ok := values["permissions"].([]any); ok {
			for _, permission := range permissions {
				if !middleware.HasPermission(principal, permission.(string)) {
					return mcpToolResult(http.StatusForbidden, map[string]any{"error": "凭证权限不能超过调用令牌权限"})
				}
			}
		}
	}
	request, cleanup, err := mcpToolHTTPRequest(ctx, definition, values)
	if err != nil {
		return mcpToolResult(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	defer cleanup()
	recorder := httptest.NewRecorder()
	definition.router.ServeHTTP(recorder, request)
	var data any
	if err := json.Unmarshal(recorder.Body.Bytes(), &data); err != nil {
		return mcpToolResult(http.StatusInternalServerError, map[string]any{"error": "工具处理器返回了无效响应"})
	}
	if !definition.includeSecrets {
		mcpRedactResponse(definition, data)
	}
	return mcpToolResult(recorder.Code, data)
}

func mcpDispatchRouter(definition *mcpToolDefinition) *gin.Engine {
	router := gin.New()
	router.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "工具处理失败"})
	}))
	router.Use(func(c *gin.Context) {
		principal, ok := mcpContext(c.Request.Context())
		if !ok || !mcpToolAllowed(principal, definition) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			return
		}
		for _, key := range []string{middleware.ContextKeyUser, middleware.ContextKeyUserID, middleware.ContextKeyProfile, middleware.ContextKeyTokenID, middleware.ContextKeyAuthType, middleware.ContextKeyTokenPerm} {
			if value, exists := principal.Get(key); exists {
				c.Set(key, value)
			}
		}
		c.Set("audit_mcp_tool", definition.name)
		c.Next()
	})
	router.Use(middleware.AuditMiddleware())
	router.Handle(definition.method, definition.route, definition.handler)
	return router
}

func mcpToolHTTPRequest(ctx context.Context, definition *mcpToolDefinition, values map[string]any) (*http.Request, func(), error) {
	route := definition.route
	if strings.Contains(route, ":id") {
		route = strings.ReplaceAll(route, ":id", url.PathEscape(values["id"].(string)))
		delete(values, "id")
	}
	cleanup := func() {}
	var body io.Reader
	contentType := ""
	if definition.uploadMode == "url" {
		form := url.Values{}
		for key, value := range values {
			form.Set(key, mcpScalarParameter(value))
		}
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	} else if definition.uploadMode == "content" {
		encoded := values["contentBase64"].(string)
		if base64.StdEncoding.DecodedLen(len(encoded)) > mcpMaxContentBytes+2 {
			return nil, cleanup, fmt.Errorf("contentBase64 文件超过 8 MiB；大文件请使用服务器 multipart 上传")
		}
		content, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(content) > mcpMaxContentBytes {
			return nil, cleanup, fmt.Errorf("contentBase64 无效或文件超过 8 MiB")
		}
		buffer := &bytes.Buffer{}
		writer := multipart.NewWriter(buffer)
		for key, value := range values {
			if key == "contentBase64" || key == "contentType" || key == "filename" {
				continue
			}
			if err := writer.WriteField(key, mcpScalarParameter(value)); err != nil {
				return nil, cleanup, err
			}
		}
		fileType, _ := values["contentType"].(string)
		if fileType == "" {
			fileType = "application/octet-stream"
		}
		if _, _, err := mime.ParseMediaType(fileType); err != nil || strings.ContainsAny(fileType, "\r\n") {
			return nil, cleanup, fmt.Errorf("contentType 必须为有效的 MIME 类型")
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": values["filename"].(string)}))
		header.Set("Content-Type", fileType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, cleanup, err
		}
		if _, err := part.Write(content); err != nil {
			return nil, cleanup, err
		}
		if err := writer.Close(); err != nil {
			return nil, cleanup, err
		}
		body = buffer
		contentType = writer.FormDataContentType()
	} else if definition.body {
		encoded, err := json.Marshal(values)
		if err != nil {
			return nil, cleanup, err
		}
		body = bytes.NewReader(encoded)
		contentType = "application/json"
	} else {
		query := url.Values{}
		for key, value := range values {
			query.Set(key, mcpScalarParameter(value))
		}
		if len(query) > 0 {
			route += "?" + query.Encode()
		}
	}
	request, err := http.NewRequestWithContext(ctx, definition.method, route, body)
	if err != nil {
		return nil, cleanup, err
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request, func() {
		if request.MultipartForm != nil {
			_ = request.MultipartForm.RemoveAll()
		}
	}, nil
}

func mcpScalarParameter(value any) string {
	if number, ok := value.(float64); ok {
		return strconv.FormatFloat(number, 'f', -1, 64)
	}
	return fmt.Sprint(value)
}

func mcpRedactResponse(definition *mcpToolDefinition, data any) {
	var forbidden map[string]bool
	switch {
	case strings.HasPrefix(definition.route, "/api/accounts"):
		forbidden = map[string]bool{"accessKeyId": true, "secretAccessKey": true, "apiToken": true}
	case definition.name == "fileflow_list_tokens":
		forbidden = map[string]bool{"token": true}
	case definition.name == "fileflow_list_webdav_credentials":
		forbidden = map[string]bool{"password": true}
	default:
		return
	}
	var redact func(any)
	redact = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if forbidden[key] {
					delete(value, key)
				} else {
					redact(child)
				}
			}
		case []any:
			for _, child := range value {
				redact(child)
			}
		}
	}
	redact(data)
}
