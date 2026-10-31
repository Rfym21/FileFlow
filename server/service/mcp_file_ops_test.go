package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fileflow/server/store"
)

type mcpOperationObject struct {
	body, contentType, marker, etag string
}

type mcpOperationR2 struct {
	sync.Mutex
	objects                         map[string]mcpOperationObject
	heads, gets, copies, puts, dels int
	copyHook                        func(*mcpOperationR2, string)
	putHook                         func(*mcpOperationR2)
	deleteFailure                   bool
	t                               *testing.T
}

func (mock *mcpOperationR2) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	mock.Lock()
	defer mock.Unlock()
	key := strings.TrimPrefix(request.URL.Path, "/bucket/")
	object, exists := mock.objects[key]
	switch request.Method {
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
	case "GET":
		mock.gets++
		if mock.copyHook != nil {
			mock.copyHook(mock, key)
		}
		object, exists = mock.objects[key]
		if !exists || request.Header.Get("If-Match") != object.etag {
			w.WriteHeader(412)
			_, _ = w.Write([]byte(`<Error><Code>PreconditionFailed</Code></Error>`))
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(object.body)))
		w.Header().Set("Content-Type", object.contentType)
		w.Header().Set("ETag", object.etag)
		_, _ = w.Write([]byte(object.body))
	case "PUT":
		if request.Header.Get("If-None-Match") != "*" || request.Header.Get("X-Amz-Meta-Fileflow-Upload-Id") == "" || request.Header.Get("X-Amz-Copy-Source") != "" {
			mock.t.Error("write must use conditional PUT with upload metadata")
		}
		directory := request.Header.Get("Content-Type") == "application/x-directory"
		if directory {
			mock.puts++
		} else {
			mock.copies++
		}
		if exists {
			w.WriteHeader(412)
			_, _ = w.Write([]byte(`<Error><Code>PreconditionFailed</Code></Error>`))
			return
		}
		body, _ := io.ReadAll(request.Body)
		if directory && (len(body) != 0 || request.Header.Get("Content-MD5") != "1B2M2Y8AsgTpgAmY7PhCfg==") {
			mock.t.Error("directory marker must be fixed empty content")
		}
		mock.objects[key] = mcpOperationObject{string(body), request.Header.Get("Content-Type"), request.Header.Get("X-Amz-Meta-Fileflow-Upload-Id"), `"uploaded"`}
		if !directory && mock.putHook != nil {
			mock.putHook(mock)
		}
		w.Header().Set("ETag", `"uploaded"`)
	case "DELETE":
		mock.dels++
		if mock.deleteFailure || !exists {
			w.WriteHeader(412)
			_, _ = w.Write([]byte(`<Error><Code>PreconditionFailed</Code></Error>`))
			return
		}
		delete(mock.objects, key)
		w.WriteHeader(204)
	default:
		mock.t.Errorf("unexpected R2 method %s", request.Method)
		w.WriteHeader(400)
	}
}

func setupMCPFileOperation(t *testing.T) (*store.Account, UploadActor, *mcpOperationR2) {
	t.Helper()
	mock := &mcpOperationR2{t: t, objects: map[string]mcpOperationObject{"source/中文 + #%?.txt": {"source", "text/plain", "", `"source"`}}}
	server := httptest.NewServer(mock)
	t.Cleanup(server.Close)
	account := setupPreviewAccount(t, server.URL)
	account.Permissions = store.DefaultAccountPermissions()
	account.Quota = store.Quota{MaxSizeBytes: 1000, MaxClassAOps: 100, MaxClassBOps: 100}
	if err := store.UpdateAccount(account); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateAccountUsage(account.ID, store.Usage{SizeBytes: 6}); err != nil {
		t.Fatal(err)
	}
	var owner store.User
	for _, user := range store.GetUsers() {
		if user.Role == store.RoleOwner {
			owner = user
		}
	}
	return account, UploadActor{UserID: owner.ID, AuthVersion: owner.AuthVersion, AuthType: "jwt"}, mock
}

func TestMCPCopyAndMoveReserveMeterAndPreserveExpiration(t *testing.T) {
	for _, move := range []bool{false, true} {
		t.Run(fmt.Sprint("move=", move), func(t *testing.T) {
			account, actor, mock := setupMCPFileOperation(t)
			deadline := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
			sourceKey := "source/中文 + #%?.txt"
			if err := store.CreateFileExpiration(&store.FileExpiration{AccountID: account.ID, FileKey: sourceKey, ExpiresAt: deadline}); err != nil {
				t.Fatal(err)
			}
			result, err := CopyMCPFile(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: sourceKey, DestinationKey: "dest/副本.txt"}, move)
			if err != nil || !result.DestinationConfirmed || result.Destination.Key != "dest/副本.txt" || result.SourceDeleted != move || result.ExpiresAt != deadline {
				t.Fatalf("operation: %+v %v", result, err)
			}
			if _, exists := mock.objects[sourceKey]; exists == move {
				t.Fatal("source deletion state is wrong")
			}
			if mock.objects[result.Destination.Key].body != "source" || mock.copies != 1 || mock.gets != 1 || mock.heads != map[bool]int{false: 3, true: 4}[move] || mock.dels != map[bool]int{true: 1, false: 0}[move] {
				t.Fatalf("remote operations are wrong: %+v", mock)
			}
			usage, _ := store.GetAccountByID(account.ID)
			if usage.Usage.ClassAOps != 1 || usage.Usage.ClassBOps != map[bool]int64{false: 4, true: 5}[move] || usage.Usage.SizeBytes != 12 {
				t.Fatalf("usage should charge one PUT and bounded HEAD/GET reads once: %+v", usage.Usage)
			}
			if reserved, _, count := store.GetUploadReservations(account.ID); reserved != 0 || count != 0 {
				t.Fatal("completed copy retains reservation")
			}
			found := false
			for _, expiration := range store.GetFileExpirations() {
				if expiration.FileKey == result.Destination.Key {
					found = expiration.ExpiresAt == deadline
				}
				if move && expiration.FileKey == sourceKey {
					t.Fatal("move retained source schedule")
				}
			}
			if !found {
				t.Fatal("copy did not preserve exact expiry")
			}
		})
	}
}

func TestMCPFileWritesRejectQuotaBeforeTransferAndCollisions(t *testing.T) {
	for _, mode := range []string{"size", "write", "read", "existing", "destination-race", "source-race"} {
		t.Run(mode, func(t *testing.T) {
			account, actor, mock := setupMCPFileOperation(t)
			switch mode {
			case "size":
				account.Quota.MaxSizeBytes = 6
			case "write":
				account.Quota.MaxClassAOps = 0
			case "read":
				account.Quota.MaxClassBOps = 2
			case "existing":
				mock.objects["destination.txt"] = mcpOperationObject{body: "original", contentType: "text/plain", etag: `"old"`}
			case "destination-race":
				mock.copyHook = func(mock *mcpOperationR2, _ string) {
					mock.objects["destination.txt"] = mcpOperationObject{body: "original", contentType: "text/plain", etag: `"old"`}
				}
			case "source-race":
				mock.copyHook = func(mock *mcpOperationR2, _ string) {
					mock.objects["source/中文 + #%?.txt"] = mcpOperationObject{body: "new source", contentType: "text/plain", etag: `"new"`}
				}
			}
			if err := store.UpdateAccount(account); err != nil {
				t.Fatal(err)
			}
			_, err := CopyMCPFile(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: "source/中文 + #%?.txt", DestinationKey: "destination.txt"}, true)
			if err == nil || mock.dels != 0 {
				t.Fatalf("unsafe operation accepted: %v", err)
			}
			if strings.HasSuffix(mode, "race") {
				if mock.gets != 1 || !errors.Is(err, store.ErrUploadSessionConflict) {
					t.Fatalf("conditional failure not reported: %v", err)
				}
			} else if mock.copies != 0 || mock.puts != 0 {
				t.Fatal("quota/collision failure transmitted a write")
			}
			if object, ok := mock.objects["destination.txt"]; ok && object.body != "original" {
				t.Fatal("original destination was overwritten")
			}
		})
	}
}

func TestMCPDirectoryWritesUseEmptyConditionalContract(t *testing.T) {
	account, actor, mock := setupMCPFileOperation(t)
	result, err := CreateMCPDirectory(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: "new/subdir"})
	if err != nil || !result.Directory || result.Destination.Key != "new/subdir/" || mock.puts != 1 || mock.heads != 2 {
		t.Fatalf("mkdir: %+v %v", result, err)
	}
	usage, _ := store.GetAccountByID(account.ID)
	if usage.Usage.ClassAOps != 1 || usage.Usage.ClassBOps != 2 || usage.Usage.SizeBytes != 6 || len(store.GetFileExpirations()) != 0 {
		t.Fatalf("directory metering/expiry: %+v", usage.Usage)
	}
	if _, err := CreateMCPDirectory(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: "new/subdir/"}); !errors.Is(err, store.ErrUploadSessionConflict) || mock.puts != 1 {
		t.Fatalf("existing marker overwritten: %v", err)
	}
}

func TestMCPMoveReturnsConfirmedDestinationOnSourceDeleteFailure(t *testing.T) {
	account, actor, mock := setupMCPFileOperation(t)
	mock.deleteFailure = true
	result, err := CopyMCPFile(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: "source/中文 + #%?.txt", DestinationKey: "copy.txt"}, true)
	if err == nil || result == nil || !result.DestinationConfirmed || result.SourceDeleted || result.Destination.Key != "copy.txt" || mock.objects["copy.txt"].body != "source" || mock.objects[result.SourceKey].body != "source" {
		t.Fatalf("partial move lost recovery/source: %+v %v", result, err)
	}
	if len(store.ListUploadSessions()) != 1 || store.ListUploadSessions()[0].Status != store.UploadStatusCompleted {
		t.Fatal("source deletion happened before destination confirmation")
	}
}

func TestMCPFileOperationsValidatePathsAndCurrentTokenRights(t *testing.T) {
	account, actor, mock := setupMCPFileOperation(t)
	for _, key := range []string{"", "/", "/root.txt", "../secret.txt", "x/./y", "x//y", "x\\y", "directory/", "x\n.txt"} {
		if _, err := CopyMCPFile(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: key, DestinationKey: "copy.txt"}, false); !errors.Is(err, store.ErrInvalidUploadSession) {
			t.Fatalf("invalid copy key %q accepted: %v", key, err)
		}
	}
	token := &store.Token{Name: "write only", UserID: actor.UserID, Permissions: []string{"write"}}
	if err := store.CreateToken(token); err != nil {
		t.Fatal(err)
	}
	actor.AuthType, actor.TokenID = "api_token", token.ID
	if _, err := CopyMCPFile(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: "source/中文 + #%?.txt", DestinationKey: "copy.txt"}, false); !errors.Is(err, store.ErrUploadSessionForbidden) || mock.heads != 0 {
		t.Fatalf("write-only token copied source: %v", err)
	}
	if err := store.DeleteToken(token.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateMCPDirectory(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: "dir"}); !errors.Is(err, store.ErrUploadSessionForbidden) || mock.puts != 0 {
		t.Fatalf("revoked token wrote directory: %v", err)
	}
}

func TestMCPCopyPreservesDeadlineThatElapsedDuringTransfer(t *testing.T) {
	account, actor, mock := setupMCPFileOperation(t)
	deadline := time.Now().Add(150 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	if err := store.CreateFileExpiration(&store.FileExpiration{AccountID: account.ID, FileKey: "source/中文 + #%?.txt", ExpiresAt: deadline}); err != nil {
		t.Fatal(err)
	}
	mock.copyHook = func(_ *mcpOperationR2, _ string) { time.Sleep(200 * time.Millisecond) }
	result, err := CopyMCPFile(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: "source/中文 + #%?.txt", DestinationKey: "copy.txt"}, false)
	if err != nil || result.ExpiresAt != deadline {
		t.Fatalf("elapsed deadline copy: %+v %v", result, err)
	}
	for _, expiration := range store.GetFileExpirations() {
		if expiration.FileKey == "copy.txt" && expiration.ExpiresAt == deadline {
			return
		}
	}
	t.Fatal("elapsed deadline was changed or dropped")
}

func TestMCPFileOperationsRejectExpiredSourceAndReplacedMoveSource(t *testing.T) {
	account, actor, mock := setupMCPFileOperation(t)
	sourceKey := "source/中文 + #%?.txt"
	if err := store.CreateFileExpiration(&store.FileExpiration{AccountID: account.ID, FileKey: sourceKey, ExpiresAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if _, err := CopyMCPFile(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: sourceKey, DestinationKey: "copy.txt"}, false); !errors.Is(err, ErrMCPFileExpired) || mock.heads != 0 || mock.gets != 0 {
		t.Fatalf("expired file copy performed remote work: %v", err)
	}
	if err := store.DeleteFileExpiration(account.ID, sourceKey); err != nil {
		t.Fatal(err)
	}
	mock.putHook = func(mock *mcpOperationR2) {
		mock.objects[sourceKey] = mcpOperationObject{body: "replacement", contentType: "text/plain", etag: `"replacement"`}
	}
	result, err := CopyMCPFile(context.Background(), actor, MCPFileOperationRequest{IDGroup: account.ID, Key: sourceKey, DestinationKey: "copy.txt"}, true)
	if !errors.Is(err, store.ErrUploadSessionConflict) || result == nil || !result.DestinationConfirmed || result.SourceDeleted || mock.dels != 0 || mock.objects[sourceKey].body != "replacement" {
		t.Fatalf("move deleted replaced source: %+v %v", result, err)
	}
}
