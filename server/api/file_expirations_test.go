package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fileflow/server/config"
	"fileflow/server/middleware"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

func setupExpirationAPI(t *testing.T) (*gin.Engine, string, string) {
	t.Helper()
	dir := t.TempDir()
	databaseURL := "sqlite:" + filepath.Join(dir, "test.db")
	t.Setenv("FILEFLOW_ADMIN_PASSWORD", "test-password")
	t.Setenv("FILEFLOW_JWT_SECRET", "test-secret")
	t.Setenv("FILEFLOW_DATA_DIR", dir)
	t.Setenv("FILEFLOW_DATABASE_URL", databaseURL)
	cfg := config.Get()
	previous := *cfg
	cfg.DataDir, cfg.DatabaseURL = dir, databaseURL
	t.Cleanup(func() { store.Close(); *cfg = previous })
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	token, err := middleware.GenerateJWT("test-admin")
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRouter(router)
	return router, token, filepath.Join(dir, "test.db")
}

func expirationRequest(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func createExpiration(t *testing.T, accountID, fileKey string) *store.FileExpiration {
	t.Helper()
	record := &store.FileExpiration{AccountID: accountID, FileKey: fileKey, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
	if err := store.CreateFileExpiration(record); err != nil {
		t.Fatal(err)
	}
	return record
}

func TestFileExpirationUpdateRoutesAndPersistence(t *testing.T) {
	router, token, dbPath := setupExpirationAPI(t)
	first := createExpiration(t, "account", "first.jpg")
	second := createExpiration(t, "account", "second.jpg")
	future := time.Now().Add(48 * time.Hour).In(time.FixedZone("east", 8*60*60)).Format(time.RFC3339)

	for _, input := range []struct {
		method, path string
		body         any
	}{{"PATCH", "/api/file-expirations", gin.H{"ids": []string{first.ID}, "expiresAt": future}}, {"PATCH", "/api/file-expirations/" + first.ID, gin.H{"expiresAt": future}}, {"POST", "/api/file-expirations/batch-delete", gin.H{"ids": []string{first.ID}}}} {
		response := expirationRequest(t, router, input.method, input.path, input.body, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without JWT returned %d", input.method, input.path, response.Code)
		}
	}
	apiToken := &store.Token{Name: "not-an-admin", Permissions: []string{"write", "delete"}}
	if err := store.CreateToken(apiToken); err != nil {
		t.Fatal(err)
	}
	response := expirationRequest(t, router, "PATCH", "/api/file-expirations/"+first.ID, gin.H{"expiresAt": future}, apiToken.Token)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("API token was allowed to administer expiration: %d", response.Code)
	}

	for _, body := range []any{
		gin.H{"ids": []string{}, "expiresAt": future},
		gin.H{"ids": []string{" "}, "expiresAt": future},
		gin.H{"ids": []string{first.ID}, "expiresAt": "invalid"},
		gin.H{"ids": []string{first.ID}, "expiresAt": time.Now().Add(-time.Hour).Format(time.RFC3339)},
	} {
		response = expirationRequest(t, router, "PATCH", "/api/file-expirations", body, token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid batch returned %d: %s", response.Code, response.Body.String())
		}
	}
	response = expirationRequest(t, router, "PATCH", "/api/file-expirations", gin.H{"ids": []string{first.ID, "missing"}, "expiresAt": future}, token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing batch record returned %d", response.Code)
	}
	current, _ := store.GetFileExpirationByID(first.ID)
	if current.ExpiresAt != first.ExpiresAt {
		t.Fatal("invalid batch partially changed a valid record")
	}
	response = expirationRequest(t, router, "PATCH", "/api/file-expirations/"+first.ID, gin.H{"expiresAt": future}, token)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"updatedCount":1`) {
		t.Fatalf("single update returned %d: %s", response.Code, response.Body.String())
	}
	response = expirationRequest(t, router, "PATCH", "/api/file-expirations", gin.H{"ids": []string{first.ID, second.ID, first.ID}, "expiresAt": future}, token)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"updatedCount":2`) {
		t.Fatalf("batch update returned %d: %s", response.Code, response.Body.String())
	}
	persisted, err := store.NewSQLiteBackend(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := persisted.Init(); err != nil {
		t.Fatal(err)
	}
	defer persisted.Close()
	loaded, err := persisted.Load()
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := time.Parse(time.RFC3339, future)
	if len(loaded.FileExpirations) != 2 {
		t.Fatalf("saved count = %d", len(loaded.FileExpirations))
	}
	for _, record := range loaded.FileExpirations {
		if record.ExpiresAt != parsed.UTC().Format(time.RFC3339) || record.AccountID != "account" || record.CreatedAt == "" {
			t.Fatalf("saved record changed unexpectedly: %#v", record)
		}
	}
}

func TestBatchDeleteFileExpirationsKeepsFailuresAndDeduplicates(t *testing.T) {
	router, token, _ := setupExpirationAPI(t)
	var lock sync.Mutex
	requests := make(map[string]int)
	mockS3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected cloud method %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		lock.Lock()
		requests[r.URL.Path]++
		lock.Unlock()
		if strings.HasSuffix(r.URL.Path, "/fail.jpg") {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`<Error><Code>AccessDenied</Code><Message>simulated failure</Message></Error>`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mockS3.Close()
	account := &store.Account{Name: "mock", IsActive: true, Endpoint: mockS3.URL, BucketName: "bucket", AccessKeyId: "test-key", SecretAccessKey: "test-secret"}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	ok := createExpiration(t, account.ID, "ok.jpg")
	fail := createExpiration(t, account.ID, "fail.jpg")
	single := createExpiration(t, account.ID, "single.jpg")
	for _, ids := range [][]string{nil, {}, {" "}} {
		response := expirationRequest(t, router, "POST", "/api/file-expirations/batch-delete", gin.H{"ids": ids}, token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid delete returned %d", response.Code)
		}
	}
	response := expirationRequest(t, router, "POST", "/api/file-expirations/batch-delete", gin.H{"ids": []string{ok.ID, fail.ID, "missing", ok.ID}}, token)
	if response.Code != http.StatusOK {
		t.Fatalf("batch returned %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Results      []batchDeleteFileExpirationResult `json:"results"`
		SuccessCount int                               `json:"successCount"`
		FailCount    int                               `json:"failCount"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SuccessCount != 1 || result.FailCount != 2 || len(result.Results) != 3 || !result.Results[0].Success || result.Results[1].Error == "" || result.Results[2].Error == "" {
		t.Fatalf("incorrect partial results: %#v", result)
	}
	if _, err := store.GetFileExpirationByID(ok.ID); err == nil {
		t.Fatal("successfully deleted file still scheduled")
	}
	if _, err := store.GetFileExpirationByID(fail.ID); err != nil {
		t.Fatal("failed cloud deletion lost its schedule")
	}
	lock.Lock()
	if requests["/bucket/ok.jpg"] != 1 || requests["/bucket/fail.jpg"] != 1 {
		t.Errorf("unexpected cloud requests: %#v", requests)
	}
	lock.Unlock()
	response = expirationRequest(t, router, "DELETE", "/api/file-expirations/"+single.ID, nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("single deletion returned %d: %s", response.Code, response.Body.String())
	}
	response = expirationRequest(t, router, "DELETE", "/api/file-expirations/"+single.ID, nil, token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing single deletion returned %d", response.Code)
	}
}
