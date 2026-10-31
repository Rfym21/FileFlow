package api

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

type relayMockObject struct {
	body                              []byte
	contentType, marker, etag, digest string
}
type relayMockR2 struct {
	sync.Mutex
	objects              map[string]relayMockObject
	heads, puts, deletes int
	rejectNextPut        bool
	failPutAfterSave     bool
	failConfirmation     bool
	putHook              func(*relayMockR2, string)
}

func (mock *relayMockR2) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mock.Lock()
	defer mock.Unlock()
	key := strings.TrimPrefix(r.URL.Path, "/bucket/")
	object, exists := mock.objects[key]
	switch r.Method {
	case "HEAD":
		mock.heads++
		if !exists {
			w.WriteHeader(404)
			return
		}
		if mock.failConfirmation {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`<Error><Code>ServiceUnavailable</Code></Error>`))
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(object.body)))
		w.Header().Set("Content-Type", object.contentType)
		w.Header().Set("ETag", object.etag)
		w.Header().Set("X-Amz-Meta-Fileflow-Upload-Id", object.marker)
		w.Header().Set("X-Amz-Meta-Fileflow-Content-Sha256", object.digest)
	case "PUT":
		mock.puts++
		if mock.rejectNextPut {
			mock.rejectNextPut = false
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`<Error><Code>ServiceUnavailable</Code></Error>`))
			return
		}
		if mock.putHook != nil {
			mock.putHook(mock, key)
			object, exists = mock.objects[key]
		}
		if exists && r.Header.Get("If-None-Match") == "*" {
			w.WriteHeader(412)
			w.Write([]byte(`<Error><Code>PreconditionFailed</Code></Error>`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		mock.objects[key] = relayMockObject{body: body, contentType: r.Header.Get("Content-Type"), marker: r.Header.Get("X-Amz-Meta-Fileflow-Upload-Id"), etag: `"mock-etag"`, digest: r.Header.Get("X-Amz-Meta-Fileflow-Content-Sha256")}
		if mock.failPutAfterSave {
			mock.failPutAfterSave = false
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`<Error><Code>ServiceUnavailable</Code></Error>`))
			return
		}
		w.Header().Set("ETag", `"mock-etag"`)
	case "DELETE":
		mock.deletes++
		delete(mock.objects, key)
		w.WriteHeader(204)
	case "GET":
		if r.URL.Query().Has("list-type") {
			w.Header().Set("Content-Type", "application/xml")
			var listing strings.Builder
			listing.WriteString(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsTruncated>false</IsTruncated>`)
			for key, object := range mock.objects {
				var escaped bytes.Buffer
				_ = xml.EscapeText(&escaped, []byte(key))
				fmt.Fprintf(&listing, "<Contents><Key>%s</Key><Size>%d</Size></Contents>", escaped.String(), len(object.body))
			}
			listing.WriteString("</ListBucketResult>")
			w.Write([]byte(listing.String()))
			return
		}
		if !exists {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", object.contentType)
		w.Write(object.body)
	default:
		w.WriteHeader(400)
	}
}

func relayTestAccount(t *testing.T) (*store.Account, *relayMockR2) {
	t.Helper()
	mock := &relayMockR2{objects: map[string]relayMockObject{}}
	server := httptest.NewServer(mock)
	t.Cleanup(server.Close)
	account := &store.Account{Name: "relay mock", IsActive: true, Endpoint: server.URL, BucketName: "bucket", PublicDomain: "example.invalid", AccessKeyId: "mock-access", SecretAccessKey: "mock-secret",
		Permissions: store.DefaultAccountPermissions(), Quota: store.Quota{MaxSizeBytes: 1024 * 1024, MaxClassAOps: 1000, MaxClassBOps: 1000}}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	return account, mock
}

func relayUploadRequest(t *testing.T, router http.Handler, token, filename, content string, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(content))
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	_ = writer.Close()
	request := httptest.NewRequest("POST", "/api/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestRelayUploadPreservesOriginalFilenameAndRefusesOverwrite(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, mock := relayTestAccount(t)
	fields := map[string]string{"idGroup": account.ID, "path": "relay", "randomFilename": "false", "expirationDays": "0"}
	response := relayUploadRequest(t, router, ownerToken, "原名.txt", "relay payload", fields)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "relay/原名.txt") {
		t.Fatalf("relay upload: %d %s", response.Code, response.Body.String())
	}
	if mock.puts != 1 || mock.heads != 2 || len(store.GetFileExpirations()) != 0 || service.IsPendingUploadKey(account.ID, "relay/原名.txt") {
		t.Fatal("upload did not preserve completion, naming or expiration")
	}
	usage, _ := store.GetAccountByID(account.ID)
	if usage.Usage.ClassAOps != 1 || usage.Usage.ClassBOps != 2 || usage.Usage.SizeBytes != int64(len("relay payload")) {
		t.Fatalf("relay quota accounting: %+v", usage.Usage)
	}
	response = relayUploadRequest(t, router, ownerToken, "原名.txt", "replacement", fields)
	if response.Code != http.StatusConflict || mock.puts != 1 || string(mock.objects["relay/原名.txt"].body) != "relay payload" {
		t.Fatalf("existing file overwritten: %d %s", response.Code, response.Body.String())
	}
}

func TestRelayRequestIDReturnsCachedResultWithoutQuotaOrAuditCharges(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, mock := relayTestAccount(t)
	fields := map[string]string{"idGroup": account.ID, "path": "retry", "requestId": "stable-request", "expirationDays": "1"}
	first := relayUploadRequest(t, router, ownerToken, "original.txt", "file bytes", fields)
	if first.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", first.Code, first.Body.String())
	}
	var result service.UploadResult
	if err := json.Unmarshal(first.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	usage, _ := store.GetAccountByID(account.ID)
	before, auditCount := usage.Usage, len(store.GetAuditEntries(2000))
	expiry := store.GetFileExpirations()[0].ExpiresAt
	account.Quota = store.Quota{MaxSizeBytes: int64(len("file bytes")), MaxClassAOps: 1, MaxClassBOps: 2}
	if err := store.UpdateAccount(account); err != nil {
		t.Fatal(err)
	}
	second := relayUploadRequest(t, router, ownerToken, "original.txt", "file bytes", fields)
	if second.Code != http.StatusOK || first.Body.String() != second.Body.String() || len(mock.objects) != 1 || mock.puts != 1 || mock.heads != 2 {
		t.Fatalf("retry duplicated upload: %d %s", second.Code, second.Body.String())
	}
	after, _ := store.GetAccountByID(account.ID)
	if after.Usage != before || len(store.GetAuditEntries(2000)) != auditCount || store.GetFileExpirations()[0].ExpiresAt != expiry {
		t.Fatal("cached retry changed quota, audit or expiration")
	}
	if sessions := store.ListUploadSessions(); len(sessions) != 1 || len(sessions[0].ContentSHA256) != 64 || sessions[0].Status != store.UploadStatusCompleted {
		t.Fatal("server upload did not store its content digest")
	}
	changed := relayUploadRequest(t, router, ownerToken, "original.txt", "otherbytes", fields)
	if changed.Code != http.StatusConflict || mock.puts != 1 || len(mock.objects) != 1 {
		t.Fatalf("request ID reused with changed content: %d %s", changed.Code, changed.Body.String())
	}
}

func TestConcurrentRelayRetriesCreateOneObjectAndAudit(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, mock := relayTestAccount(t)
	fields := map[string]string{"idGroup": account.ID, "path": "concurrent", "requestId": "same-task"}
	var group sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 4)
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			results <- relayUploadRequest(t, router, ownerToken, "same.txt", "same bytes", fields)
		}()
	}
	group.Wait()
	close(results)
	var first string
	for response := range results {
		if response.Code != http.StatusOK {
			t.Fatalf("concurrent retry: %d %s", response.Code, response.Body.String())
		}
		if first == "" {
			first = response.Body.String()
		} else if first != response.Body.String() {
			t.Fatal("concurrent retry returned a different key")
		}
	}
	if mock.puts != 1 || mock.heads != 2 || len(mock.objects) != 1 || len(store.GetAuditEntries(2000)) != 1 {
		t.Fatal("concurrent retry duplicated requests, objects or audit")
	}
}

func TestRelayRetryProbesUncertainUploadAndChargesEveryRepeatedPUT(t *testing.T) {
	for _, mode := range []string{"saved-but-error", "put-rejected", "confirmation-failed"} {
		t.Run(mode, func(t *testing.T) {
			router, ownerToken, _ := setupExpirationAPI(t)
			account, mock := relayTestAccount(t)
			switch mode {
			case "saved-but-error":
				mock.failPutAfterSave = true
			case "put-rejected":
				mock.rejectNextPut = true
			case "confirmation-failed":
				mock.failConfirmation = true
			}
			fields := map[string]string{"idGroup": account.ID, "path": "retry", "requestId": mode}
			first := relayUploadRequest(t, router, ownerToken, "same.txt", "payload", fields)
			if first.Code < 400 {
				t.Fatal("test failed to simulate an uncertain upload")
			}
			mock.failConfirmation = false
			second := relayUploadRequest(t, router, ownerToken, "same.txt", "payload", fields)
			if second.Code != http.StatusOK || len(mock.objects) != 1 {
				t.Fatalf("recover upload: %d %s", second.Code, second.Body.String())
			}
			wantPuts := 1
			if mode == "put-rejected" {
				wantPuts = 2
			}
			usage, _ := store.GetAccountByID(account.ID)
			if mock.puts != wantPuts || usage.Usage.ClassAOps != int64(wantPuts) || usage.Usage.SizeBytes != int64(len("payload")) || usage.Usage.ClassBOps != int64(mock.heads) {
				t.Fatalf("retry bypassed request counting: %+v puts=%d heads=%d", usage.Usage, mock.puts, mock.heads)
			}
		})
	}
}

func TestRelayUploadRoleTokenChannelQuotaAndDeprecatedEndpoints(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, mock := relayTestAccount(t)
	_, viewerToken := teamUser(t, "relay-viewer", "viewer")
	fields := map[string]string{"idGroup": account.ID, "path": "api"}
	for _, token := range []string{"", viewerToken} {
		response := relayUploadRequest(t, router, token, "no.txt", "blocked", fields)
		if response.Code != http.StatusUnauthorized && response.Code != http.StatusForbidden {
			t.Fatalf("unauthorized upload: %d", response.Code)
		}
	}
	account.Permissions.ClientUpload = false
	if err := store.UpdateAccount(account); err != nil {
		t.Fatal(err)
	}
	apiToken := &store.Token{Name: "API writer", Permissions: []string{"write"}}
	if err := store.CreateToken(apiToken); err != nil {
		t.Fatal(err)
	}
	response := relayUploadRequest(t, router, apiToken.Token, "api.txt", "API", fields)
	if response.Code != http.StatusOK {
		t.Fatalf("API channel failed: %d %s", response.Code, response.Body.String())
	}
	response = relayUploadRequest(t, router, ownerToken, "client.txt", "client", fields)
	if response.Code < 400 || mock.puts != 1 {
		t.Fatal("disabled client channel uploaded file")
	}
	for _, endpoint := range []string{"/api/upload/init", "/api/upload/complete"} {
		response := expirationRequest(t, router, "POST", endpoint, gin.H{}, ownerToken)
		if response.Code != http.StatusNotFound {
			t.Fatalf("removed direct endpoint still callable: %s %d", endpoint, response.Code)
		}
	}
	account.Permissions.ClientUpload = true
	account.Quota.MaxSizeBytes = 3
	if err := store.UpdateAccount(account); err != nil {
		t.Fatal(err)
	}
	response = relayUploadRequest(t, router, ownerToken, "oversize.txt", "large", fields)
	if response.Code != http.StatusTooManyRequests || mock.puts != 1 {
		t.Fatalf("quota exceeded before relay upload: %d %s", response.Code, response.Body.String())
	}
}

func TestRelayUploadOriginalNameRaceCannotOverwrite(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, mock := relayTestAccount(t)
	mock.putHook = func(mock *relayMockR2, key string) {
		mock.objects[key] = relayMockObject{body: []byte("original"), contentType: "text/plain", etag: `"original"`}
	}
	response := relayUploadRequest(t, router, ownerToken, "same.txt", "replacement", map[string]string{"idGroup": account.ID, "path": "race", "randomFilename": "false"})
	if response.Code != http.StatusConflict || string(mock.objects["race/same.txt"].body) != "original" {
		t.Fatalf("destination race overwritten: %d %s", response.Code, response.Body.String())
	}
}

func TestRelayUploadInvalidExpirationAndEmptyFile(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, mock := relayTestAccount(t)
	fields := map[string]string{"idGroup": account.ID, "path": "empty", "randomFilename": "false", "expirationDays": "bad"}
	response := relayUploadRequest(t, router, ownerToken, "empty.txt", "", fields)
	if response.Code != http.StatusBadRequest || mock.puts != 0 {
		t.Fatal("invalid expiry silently became permanent")
	}
	fields["expirationDays"] = "2"
	response = relayUploadRequest(t, router, ownerToken, "empty.txt", "", fields)
	if response.Code != http.StatusOK || len(mock.objects["empty/empty.txt"].body) != 0 || len(store.GetFileExpirations()) != 1 {
		t.Fatalf("empty file upload: %d %s", response.Code, response.Body.String())
	}
	deadline, _ := time.Parse(time.RFC3339, store.GetFileExpirations()[0].ExpiresAt)
	if time.Until(deadline) < 47*time.Hour || time.Until(deadline) > 49*time.Hour {
		t.Fatal("explicit expiry was not preserved")
	}
}
