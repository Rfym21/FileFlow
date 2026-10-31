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
	"net/url"
	"strings"
	"sync"
	"testing"

	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

type directMockObject struct {
	body                      []byte
	contentType, marker, etag string
}
type directMockR2 struct {
	sync.Mutex
	objects              map[string]directMockObject
	heads, puts, deletes int
}

func (mock *directMockR2) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Length", fmt.Sprint(len(object.body)))
		w.Header().Set("Content-Type", object.contentType)
		w.Header().Set("ETag", object.etag)
		w.Header().Set("X-Amz-Meta-Fileflow-Upload-Id", object.marker)
	case "PUT":
		mock.puts++
		if exists && r.Header.Get("If-None-Match") == "*" {
			w.WriteHeader(412)
			w.Write([]byte(`<Error><Code>PreconditionFailed</Code></Error>`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		mock.objects[key] = directMockObject{body, r.Header.Get("Content-Type"), r.Header.Get("X-Amz-Meta-Fileflow-Upload-Id"), `"mock-etag"`}
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

func directTestAccount(t *testing.T) (*store.Account, *directMockR2) {
	t.Helper()
	mock := &directMockR2{objects: map[string]directMockObject{}}
	server := httptest.NewServer(mock)
	t.Cleanup(server.Close)
	account := &store.Account{Name: "direct mock", IsActive: true, Endpoint: server.URL, BucketName: "bucket", PublicDomain: "example.invalid", AccessKeyId: "mock-access", SecretAccessKey: "mock-secret",
		Permissions: store.DefaultAccountPermissions(), Quota: store.Quota{MaxSizeBytes: 1024 * 1024, MaxClassAOps: 1000, MaxClassBOps: 1000}}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	return account, mock
}

func directInitRequest(t *testing.T, router http.Handler, token string, body any) service.DirectUploadInit {
	t.Helper()
	response := expirationRequest(t, router, "POST", "/api/upload/init", body, token)
	if response.Code != 200 {
		t.Fatalf("init: %d %s", response.Code, response.Body.String())
	}
	var result service.DirectUploadInit
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func directPutRequest(t *testing.T, init service.DirectUploadInit, body []byte) int {
	t.Helper()
	request, err := http.NewRequest(init.Method, init.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range init.Headers {
		request.Header.Set(name, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	return response.StatusCode
}

func TestDirectUploadContractOwnershipPendingAndIdempotentCompletion(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, mock := directTestAccount(t)
	_, otherToken := teamUser(t, "other-writer", "member")
	_, viewerToken := teamUser(t, "direct-reader", "viewer")
	body := []byte("团队直传测试")
	input := gin.H{"filename": "原文件名.txt", "path": "docs", "size": len(body), "contentType": "text/plain", "idGroup": account.ID, "expirationDays": 7, "randomFilename": false, "requestId": "same-request"}
	for _, token := range []string{"", viewerToken} {
		response := expirationRequest(t, router, "POST", "/api/upload/init", input, token)
		if response.Code != 401 && response.Code != 403 {
			t.Fatalf("unauthorized init: %d", response.Code)
		}
	}
	init := directInitRequest(t, router, ownerToken, input)
	if init.Key != "docs/原文件名.txt" || init.UploadID == "" || init.Method != "PUT" || init.Size != int64(len(body)) {
		t.Fatalf("bad init contract: %#v", init)
	}
	address, _ := url.Parse(init.URL)
	signedHeaders := address.Query().Get("X-Amz-SignedHeaders")
	for _, required := range []string{"content-length", "content-type", "if-none-match", "x-amz-meta-fileflow-upload-id"} {
		if !strings.Contains(signedHeaders, required) {
			t.Fatalf("signature does not bind %s: %s", required, signedHeaders)
		}
	}
	if init.Headers["Content-Length"] != "" || init.Headers["Host"] != "" || init.Headers["If-None-Match"] != "*" {
		t.Fatalf("invalid browser headers: %#v", init.Headers)
	}
	if duplicate := directInitRequest(t, router, ownerToken, input); duplicate.UploadID != init.UploadID {
		t.Fatal("init retry created a second reservation")
	}
	response := expirationRequest(t, router, "POST", "/api/upload/complete", gin.H{"uploadId": init.UploadID}, otherToken)
	if response.Code != 403 {
		t.Fatalf("another member completed upload: %d", response.Code)
	}
	response = expirationRequest(t, router, "POST", "/api/upload/complete", gin.H{"uploadId": init.UploadID}, ownerToken)
	if response.Code != 404 || !strings.Contains(response.Body.String(), "upload_not_uploaded") {
		t.Fatalf("unuploaded completion: %d %s", response.Code, response.Body.String())
	}
	if status := directPutRequest(t, init, body); status != 200 {
		t.Fatalf("PUT: %d", status)
	}
	if !service.IsPendingUploadKey(account.ID, init.Key) {
		t.Fatal("unconfirmed object was exposed")
	}
	if _, err := service.GetFilePreview(httptest.NewRequest("GET", "/", nil).Context(), account.ID, init.Key); err == nil {
		t.Fatal("pending object was previewable")
	}
	response = expirationRequest(t, router, "POST", "/api/upload/complete", gin.H{"uploadId": init.UploadID}, ownerToken)
	if response.Code != 200 {
		t.Fatalf("completion: %d %s", response.Code, response.Body.String())
	}
	if service.IsPendingUploadKey(account.ID, init.Key) || len(store.GetFileExpirations()) != 1 {
		t.Fatal("completion did not finalize file metadata")
	}
	after, _ := store.GetAccountByID(account.ID)
	if after.Usage.SizeBytes != int64(len(body)) || after.Usage.ClassAOps != 1 {
		t.Fatalf("usage charged incorrectly: %#v", after.Usage)
	}
	headCount := mock.heads
	response = expirationRequest(t, router, "POST", "/api/upload/complete", gin.H{"uploadId": init.UploadID}, ownerToken)
	if response.Code != 200 || mock.heads != headCount || len(store.GetFileExpirations()) != 1 {
		t.Fatal("repeated completion was not idempotent")
	}
	if status := directPutRequest(t, init, []byte("changed")); status != 412 {
		t.Fatalf("replay overwrote existing object: %d", status)
	}
	input["requestId"] = "collision-request"
	response = expirationRequest(t, router, "POST", "/api/upload/init", input, ownerToken)
	if response.Code != 409 {
		t.Fatalf("filename collision: %d %s", response.Code, response.Body.String())
	}
}

func TestDirectUploadZeroSizeAndTamperedObjectRejected(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, mock := directTestAccount(t)
	zero := directInitRequest(t, router, ownerToken, gin.H{"filename": "empty.txt", "size": 0, "idGroup": account.ID})
	parsed, _ := url.Parse(zero.URL)
	if !strings.Contains(parsed.Query().Get("X-Amz-SignedHeaders"), "content-md5") || zero.Headers["Content-Md5"] != "1B2M2Y8AsgTpgAmY7PhCfg==" {
		t.Fatal("zero-size signature must bind the empty-body Content-MD5 checksum")
	}
	if status := directPutRequest(t, zero, nil); status != 200 {
		t.Fatalf("zero PUT: %d", status)
	}
	response := expirationRequest(t, router, "POST", "/api/upload/complete", gin.H{"uploadId": zero.UploadID}, ownerToken)
	if response.Code != 200 {
		t.Fatalf("zero completion: %d %s", response.Code, response.Body.String())
	}
	init := directInitRequest(t, router, ownerToken, gin.H{"filename": "bad.txt", "size": 2, "contentType": "text/plain", "idGroup": account.ID})
	mock.objects[init.Key] = directMockObject{[]byte("wrong-size"), "text/plain", init.UploadID, `"tag"`}
	response = expirationRequest(t, router, "POST", "/api/upload/complete", gin.H{"uploadId": init.UploadID}, ownerToken)
	if response.Code != 409 {
		t.Fatalf("tampered file accepted: %d %s", response.Code, response.Body.String())
	}
	after, _ := store.GetAccountByID(account.ID)
	if after.Usage.SizeBytes != 0 {
		t.Fatal("tampered object charged completed storage")
	}
	for _, body := range []any{gin.H{"filename": "x.txt", "size": -1}, gin.H{"filename": "x.txt", "size": 6 * 1024 * 1024 * 1024}, gin.H{"filename": "x.txt", "size": 0, "path": "../escape"}, gin.H{"filename": "x.txt", "size": 0, "unexpected": "secret"}} {
		response = expirationRequest(t, router, "POST", "/api/upload/init", body, ownerToken)
		if response.Code != 400 {
			t.Fatalf("invalid init accepted: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestRelayUploadRetainsMultipartAndOriginalFilenameWithoutOverwrite(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, mock := directTestAccount(t)
	call := func() *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "原名.txt")
		_, _ = part.Write([]byte("relay payload"))
		for name, value := range map[string]string{"path": "relay", "idGroup": account.ID, "randomFilename": "false", "expirationDays": "0"} {
			_ = writer.WriteField(name, value)
		}
		_ = writer.Close()
		request := httptest.NewRequest("POST", "/api/upload", &body)
		request.Header.Set("Authorization", "Bearer "+ownerToken)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	response := call()
	if response.Code != 200 || !strings.Contains(response.Body.String(), "relay/原名.txt") {
		t.Fatalf("relay upload: %d %s", response.Code, response.Body.String())
	}
	if mock.puts != 1 || len(store.GetFileExpirations()) != 0 || service.IsPendingUploadKey(account.ID, "relay/原名.txt") {
		t.Fatal("relay completion did not preserve contract")
	}
	response = call()
	if response.Code != 409 || mock.puts != 1 {
		t.Fatalf("relay overwrote original: %d %s", response.Code, response.Body.String())
	}
}

func TestDirectUploadTokenUsesAPIChannelAndRequiresSameCredential(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	account, _ := directTestAccount(t)
	account.Permissions.ClientUpload = false
	if err := store.UpdateAccount(account); err != nil {
		t.Fatal(err)
	}
	token := &store.Token{Name: "API writer", Permissions: []string{"write"}}
	if err := store.CreateToken(token); err != nil {
		t.Fatal(err)
	}
	init := directInitRequest(t, router, token.Token, gin.H{"filename": "api.txt", "size": 3, "idGroup": account.ID})
	if status := directPutRequest(t, init, []byte("api")); status != 200 {
		t.Fatalf("api PUT: %d", status)
	}
	response := expirationRequest(t, router, "POST", "/api/upload/complete", gin.H{"uploadId": init.UploadID}, ownerToken)
	if response.Code != 403 {
		t.Fatalf("JWT completed API token's session: %d", response.Code)
	}
	response = expirationRequest(t, router, "POST", "/api/upload/complete", gin.H{"uploadId": init.UploadID}, token.Token)
	if response.Code != 200 {
		t.Fatalf("token complete: %d %s", response.Code, response.Body.String())
	}
}

func TestDirectUploadAutomaticSelectionSkipsInsufficientReadQuota(t *testing.T) {
	router, ownerToken, _ := setupExpirationAPI(t)
	first, _ := directTestAccount(t)
	first.Quota.MaxClassBOps = 1
	if err := store.UpdateAccount(first); err != nil {
		t.Fatal(err)
	}
	second := *first
	second.ID = ""
	second.Name = "read-quota available"
	second.Quota.MaxClassBOps = 100
	second.Usage.SizeBytes = 1
	if err := store.CreateAccount(&second); err != nil {
		t.Fatal(err)
	}
	init := directInitRequest(t, router, ownerToken, gin.H{"filename": "automatic.txt", "size": 3})
	if init.AccountID != second.ID {
		t.Fatal("automatic upload selected an account without enough read quota")
	}
	uncharged, _ := store.GetAccountByID(first.ID)
	if uncharged.Usage.ClassBOps != 0 {
		t.Fatal("skipped account consumed read quota")
	}
}
