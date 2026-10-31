package api

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"fileflow/server/service"
	"fileflow/server/store"
	"github.com/gin-gonic/gin"
)

func browserTestAccount(t *testing.T, keys []string) *store.Account {
	t.Helper()
	sort.Strings(keys)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		if r.Method == "HEAD" {
			if key == "missing.txt" {
				w.WriteHeader(404)
			} else {
				w.Header().Set("Content-Length", "3")
			}
			return
		}
		if r.Method == "DELETE" {
			if key == "bad.txt" {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(403)
				fmt.Fprint(w, `<Error><Code>AccessDenied</Code></Error>`)
			} else {
				w.WriteHeader(204)
			}
			return
		}
		if !r.URL.Query().Has("list-type") {
			w.WriteHeader(400)
			return
		}
		matched := []string{}
		for _, value := range keys {
			if strings.HasPrefix(value, r.URL.Query().Get("prefix")) && value > r.URL.Query().Get("start-after") {
				matched = append(matched, value)
			}
		}
		more := len(matched) > 1000
		if more {
			matched = matched[:1000]
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsTruncated>%t</IsTruncated>`, more)
		for _, value := range matched {
			var escaped strings.Builder
			xml.EscapeText(&escaped, []byte(value))
			fmt.Fprintf(w, "<Contents><Key>%s</Key><Size>3</Size><LastModified>2026-10-31T00:00:00Z</LastModified></Contents>", escaped.String())
		}
		fmt.Fprint(w, "</ListBucketResult>")
	}))
	t.Cleanup(server.Close)
	account := &store.Account{Name: "browser", IsActive: true, Endpoint: server.URL, BucketName: "bucket", AccessKeyId: "fake", SecretAccessKey: "fake", Permissions: store.DefaultAccountPermissions(), Quota: store.Quota{MaxSizeBytes: 1 << 20, MaxClassAOps: 100, MaxClassBOps: 100}}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	return account
}

func TestFlatBrowserPaginationAndWholePathSearch(t *testing.T) {
	router, owner, _ := setupExpirationAPI(t)
	account := browserTestAccount(t, []string{"a/readme.txt", "a/图.txt", "b/readme.txt", "folder/", "z.txt"})
	readPage := func(query, cursor string) service.AccountFiles {
		params := url.Values{"flat": {"true"}, "limit": {"1"}, "idGroup": {account.ID}, "q": {query}, "cursor": {cursor}}
		result := expirationRequest(t, router, "GET", "/api/files?"+params.Encode(), nil, owner)
		if result.Code != http.StatusOK {
			t.Fatalf("browse %d: %s", result.Code, result.Body.String())
		}
		var accounts []service.AccountFiles
		if err := json.Unmarshal(result.Body.Bytes(), &accounts); err != nil || len(accounts) != 1 {
			t.Fatalf("invalid browser response: %v", err)
		}
		return accounts[0]
	}
	seen := map[string]bool{}
	cursor := ""
	for pages := 0; pages < 10; pages++ {
		page := readPage("", cursor)
		for _, file := range page.Files {
			if seen[file.Key] || file.IsDir {
				t.Fatal("duplicate or folder in flat view")
			}
			seen[file.Key] = true
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 4 || !seen["a/图.txt"] {
		t.Fatalf("flat pagination lost files: %#v", seen)
	}
	first := readPage("READme", "")
	second := readPage("READme", first.NextCursor)
	if len(first.Files) != 1 || len(second.Files) != 1 || first.Files[0].Key == second.Files[0].Key {
		t.Fatal("search dropped equal basenames in different folders")
	}
	if second.NextCursor != "" {
		last := readPage("READme", second.NextCursor)
		if len(last.Files) != 0 || last.NextCursor != "" {
			t.Fatal("search did not terminate after its final scan")
		}
	}
	invalid := expirationRequest(t, router, "GET", "/api/files?flat=true&idGroup="+account.ID+"&q=other&cursor="+url.QueryEscape(first.NextCursor), nil, owner)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("cursor scope change accepted: %d", invalid.Code)
	}
}

func TestFlatSearchEmptyPageRetainsContinuation(t *testing.T) {
	router, owner, _ := setupExpirationAPI(t)
	keys := []string{"zz-needle.txt"}
	for i := 0; i < 1000; i++ {
		keys = append(keys, fmt.Sprintf("noise-%04d.txt", i))
	}
	account := browserTestAccount(t, keys)
	endpoint := "/api/files?flat=true&q=needle&limit=10&idGroup=" + account.ID
	response := expirationRequest(t, router, "GET", endpoint, nil, owner)
	var first []service.AccountFiles
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &first) != nil || len(first) != 1 || len(first[0].Files) != 0 || first[0].NextCursor == "" {
		t.Fatal("search falsely completed after a page without matches")
	}
	response = expirationRequest(t, router, "GET", endpoint+"&cursor="+url.QueryEscape(first[0].NextCursor), nil, owner)
	var second []service.AccountFiles
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &second) != nil || len(second[0].Files) != 1 || second[0].Files[0].Key != "zz-needle.txt" {
		t.Fatal("search continuation lost later match")
	}
}

func TestFileBatchPermissionsPartialDeleteAndAtomicExpiration(t *testing.T) {
	router, owner, _ := setupExpirationAPI(t)
	account := browserTestAccount(t, []string{"good.txt", "bad.txt"})
	_, reader := teamUser(t, "flat-reader", "viewer")
	_, writer := teamUser(t, "flat-writer", "member")
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	for _, test := range []struct {
		method, path string
		body         any
	}{
		{"POST", "/api/files/batch-delete", gin.H{"accountId": account.ID, "keys": []string{"good.txt"}}},
		{"PATCH", "/api/files/expirations", gin.H{"accountId": account.ID, "keys": []string{"good.txt"}, "expiresAt": future}},
	} {
		result := expirationRequest(t, router, test.method, test.path, test.body, reader)
		if result.Code != 403 {
			t.Fatalf("reader mutation %d", result.Code)
		}
	}
	request := gin.H{"accountId": account.ID, "keys": []string{"good.txt", "missing.txt"}, "expiresAt": future}
	result := expirationRequest(t, router, "PATCH", "/api/files/expirations", request, writer)
	if result.Code == 200 || len(store.GetFileExpirations()) != 0 {
		t.Fatal("missing file partially updated expiration")
	}
	request["keys"] = []string{"good.txt", "bad.txt", "good.txt"}
	result = expirationRequest(t, router, "PATCH", "/api/files/expirations", request, writer)
	if result.Code != 200 || !strings.Contains(result.Body.String(), `"updatedCount":2`) {
		t.Fatalf("batch expiration failed: %s", result.Body.String())
	}
	result = expirationRequest(t, router, "POST", "/api/files/batch-delete", gin.H{"accountId": account.ID, "keys": []string{"good.txt", "bad.txt"}}, writer)
	if result.Code != 200 || !strings.Contains(result.Body.String(), `"successCount":1`) || !strings.Contains(result.Body.String(), `"failCount":1`) {
		t.Fatalf("partial delete not reported: %s", result.Body.String())
	}
	if records := store.GetFileExpirations(); len(records) != 1 || records[0].FileKey != "bad.txt" {
		t.Fatal("failed delete removed its expiration record")
	}
	result = expirationRequest(t, router, "PATCH", "/api/files/expirations", gin.H{"accountId": account.ID, "keys": []string{"bad.txt"}, "expiresAt": nil}, owner)
	if result.Code != 200 || len(store.GetFileExpirations()) != 0 {
		t.Fatal("permanent expiration did not clear plan")
	}
}
