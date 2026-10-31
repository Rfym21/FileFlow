package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"fileflow/server/service"
	"fileflow/server/store"
)

func TestFilePreviewRequiresReadPermissionAndValidFile(t *testing.T) {
	router, jwtToken, _ := setupExpirationAPI(t)
	mockS3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>missing mock file</Message></Error>`))
	}))
	defer mockS3.Close()
	account := &store.Account{Name: "private mock", Endpoint: mockS3.URL, BucketName: "bucket", AccessKeyId: "mock-access", SecretAccessKey: "mock-secret"}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	readToken := &store.Token{Name: "reader", Permissions: []string{"read"}}
	writeToken := &store.Token{Name: "writer", Permissions: []string{"write"}}
	for _, token := range []*store.Token{readToken, writeToken} {
		if err := store.CreateToken(token); err != nil {
			t.Fatal(err)
		}
	}
	fileKey := "folder/中文 + #?.png"
	endpoint := "/api/file-preview?idGroup=" + url.QueryEscape(account.ID) + "&key=" + url.QueryEscape(fileKey)
	response := expirationRequest(t, router, "GET", endpoint, nil, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated preview returned %d", response.Code)
	}
	response = expirationRequest(t, router, "GET", endpoint, nil, writeToken.Token)
	if response.Code != http.StatusForbidden {
		t.Fatalf("write-only token preview returned %d", response.Code)
	}
	for _, token := range []string{jwtToken, readToken.Token} {
		response = expirationRequest(t, router, "GET", endpoint, nil, token)
		if response.Code != http.StatusOK {
			t.Fatalf("reader preview returned %d: %s", response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("preview contents and temporary links must not be cached")
		}
		var preview service.FilePreview
		if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		address, _ := url.Parse(preview.URL)
		if preview.Kind != "image" || address.Path != "/bucket/"+fileKey || preview.DownloadURL == "" {
			t.Fatalf("invalid API preview: %#v", preview)
		}
	}
	for _, endpoint := range []string{
		"/api/file-preview",
		"/api/file-preview?key=file.png",
		"/api/file-preview?idGroup=" + account.ID,
		"/api/file-preview?idGroup=" + account.ID + "&key=folder%2F",
	} {
		response = expirationRequest(t, router, "GET", endpoint, nil, readToken.Token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid preview %s returned %d", endpoint, response.Code)
		}
	}
	response = expirationRequest(t, router, "GET", "/api/file-preview?idGroup=missing&key=image.png", nil, readToken.Token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing account returned %d", response.Code)
	}
	response = expirationRequest(t, router, "GET", "/api/file-preview?idGroup="+account.ID+"&key=missing.txt", nil, readToken.Token)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "读取文件预览失败") {
		t.Fatalf("missing text returned %d: %s", response.Code, response.Body.String())
	}
	response = expirationRequest(t, router, "GET", "/api/file-preview?idGroup="+account.ID+"&key=missing-without-extension", nil, readToken.Token)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "读取文件类型失败") {
		t.Fatalf("missing metadata returned %d: %s", response.Code, response.Body.String())
	}
}
