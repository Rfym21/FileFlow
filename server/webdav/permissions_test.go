package webdav

import (
	"context"
	"errors"
	"fileflow/server/store"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type permissionStorage struct {
	Storage
	exists  bool
	removed bool
	copied  bool
	moved   bool
}

func (s *permissionStorage) Get(context.Context, string) (FileInfo, error) {
	if !s.exists {
		return nil, errors.New("not found")
	}
	return &S3FileInfo{name: "destination"}, nil
}
func (s *permissionStorage) Remove(context.Context, string) error       { s.removed = true; return nil }
func (s *permissionStorage) Copy(context.Context, string, string) error { s.copied = true; return nil }
func (s *permissionStorage) Move(context.Context, string, string) error { s.moved = true; return nil }

func TestWebDAVCopyMoveRequiresSourceAndOverwritePermissions(t *testing.T) {
	for _, test := range []struct {
		name, method string
		permissions  []string
		exists       bool
		status       int
	}{
		{"copy requires read", "COPY", []string{"write", "delete"}, false, http.StatusForbidden},
		{"move requires delete", "MOVE", []string{"read", "write"}, false, http.StatusForbidden},
		{"copy overwrite requires delete", "COPY", []string{"read", "write"}, true, http.StatusForbidden},
		{"copy new destination", "COPY", []string{"read", "write"}, false, http.StatusCreated},
		{"copy overwrite", "COPY", []string{"read", "write", "delete"}, true, http.StatusNoContent},
		{"move with permissions", "MOVE", []string{"read", "write", "delete"}, false, http.StatusCreated},
	} {
		t.Run(test.name, func(t *testing.T) {
			storage := &permissionStorage{exists: test.exists}
			handler := &Handler{LockSystem: NewMemLS()}
			request := httptest.NewRequest(test.method, "http://example.com/source", nil)
			request.Header.Set("Destination", "http://example.com/destination")
			user := NewWebDAVUser(&store.WebDAVCredential{Permissions: test.permissions}, &store.Account{})
			status, err := handler.handleCopyMove(httptest.NewRecorder(), request, storage, user)
			if status != test.status {
				t.Fatalf("status=%d want=%d err=%v", status, test.status, err)
			}
			if test.status == http.StatusForbidden && (storage.removed || storage.copied || storage.moved) {
				t.Fatal("forbidden operation mutated storage")
			}
		})
	}
}

func TestWebDAVLockedCopyDoesNotDeleteDestination(t *testing.T) {
	storage := &permissionStorage{exists: true}
	locks := NewMemLS()
	if _, err := locks.Create(time.Now(), LockDetails{Root: "/source", Duration: time.Hour, ZeroDepth: true}); err != nil {
		t.Fatal(err)
	}
	handler := &Handler{LockSystem: locks}
	request := httptest.NewRequest("COPY", "http://example.com/source", nil)
	request.Header.Set("Destination", "http://example.com/destination")
	user := NewWebDAVUser(&store.WebDAVCredential{Permissions: []string{"read", "write", "delete"}}, &store.Account{})
	status, _ := handler.handleCopyMove(httptest.NewRecorder(), request, storage, user)
	if status != StatusLocked || storage.removed || storage.copied {
		t.Fatalf("locked source mutated destination: status=%d removed=%v copied=%v", status, storage.removed, storage.copied)
	}
}

func TestWebDAVAccountLocksIsolateIdenticalPaths(t *testing.T) {
	accountA := "test:" + t.Name() + ":a"
	accountB := "test:" + t.Name() + ":b"
	locksA := getAccountLockSystem(accountA)
	locksB := getAccountLockSystem(accountB)
	tokenA, err := locksA.Create(time.Now(), LockDetails{Root: "/source", Duration: time.Hour, ZeroDepth: true})
	if err != nil {
		t.Fatal(err)
	}
	defer locksA.Unlock(time.Now(), tokenA)
	// Another credential for the same account must see its existing lock.
	if _, err := getAccountLockSystem(accountA).Create(time.Now(), LockDetails{Root: "/source", Duration: time.Hour, ZeroDepth: true}); err != ErrLocked {
		t.Fatalf("same account did not share the lock: %v", err)
	}
	user := NewWebDAVUser(&store.WebDAVCredential{Permissions: []string{"read", "write", "delete"}}, &store.Account{})
	for _, test := range []struct {
		name  string
		locks LockSystem
		want  int
	}{
		{"same account", locksA, StatusLocked},
		{"other account", locksB, http.StatusCreated},
	} {
		storage := &permissionStorage{}
		request := httptest.NewRequest("COPY", "http://example.com/source", nil)
		request.Header.Set("Destination", "http://example.com/destination")
		handler := &Handler{LockSystem: test.locks}
		status, err := handler.handleCopyMove(httptest.NewRecorder(), request, storage, user)
		if status != test.want {
			t.Fatalf("%s: status=%d want=%d err=%v", test.name, status, test.want, err)
		}
		if test.want == StatusLocked && storage.copied {
			t.Fatal("a second member bypassed the account lock")
		}
		if test.want == http.StatusCreated && !storage.copied {
			t.Fatal("unrelated bucket was blocked by the account lock")
		}
	}
	// The same path can be locked in the other bucket independently.
	tokenB, err := locksB.Create(time.Now(), LockDetails{Root: "/source", Duration: time.Hour, ZeroDepth: true})
	if err != nil {
		t.Fatal(err)
	}
	defer locksB.Unlock(time.Now(), tokenB)
}
