package webdav

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"fileflow/server/config"
	"fileflow/server/store"
)

func TestWebDAVHidesAndProtectsPendingDirectUploads(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FILEFLOW_ADMIN_USER", "owner")
	t.Setenv("FILEFLOW_ADMIN_PASSWORD", "test-password")
	t.Setenv("FILEFLOW_JWT_SECRET", "test-only-secret")
	cfg := config.Get()
	previous := *cfg
	cfg.AdminUser, cfg.AdminPassword, cfg.DataDir, cfg.DatabaseURL = "owner", "test-password", dir, "sqlite:"+filepath.Join(dir, "test.db")
	t.Cleanup(func() { store.Close(); *cfg = previous })
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	owner, err := store.GetUserByUsername("owner")
	if err != nil {
		t.Fatal(err)
	}
	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Query().Has("list-type") {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>pending.txt</Key><Size>3</Size></Contents><Contents><Key>visible.txt</Key><Size>3</Size></Contents></ListBucketResult>`)
			return
		}
		mutations++
		w.WriteHeader(500)
	}))
	defer server.Close()
	account := &store.Account{Name: "pending", IsActive: true, Endpoint: server.URL, BucketName: "bucket", AccessKeyId: "fake", SecretAccessKey: "fake", Permissions: store.DefaultAccountPermissions(), Quota: store.Quota{MaxSizeBytes: 1000, MaxClassAOps: 100, MaxClassBOps: 100}}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	task := &store.UploadSession{UserID: owner.ID, AuthType: "jwt", AuthVersion: owner.AuthVersion, AccountID: account.ID, FileKey: "pending.txt", Filename: "pending.txt", ExpectedSize: 3, ContentType: "text/plain", CreatedAt: now.Format(time.RFC3339), UploadExpiresAt: now.Add(15 * time.Minute).Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339), Status: store.UploadStatusPending}
	if err := store.CreateUploadSession(task); err != nil {
		t.Fatal(err)
	}
	storage, err := NewS3Storage(account)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := storage.Get(ctx, "/pending.txt"); err == nil {
		t.Fatal("pending upload metadata was visible")
	}
	if body, _, err := storage.Open(ctx, "/pending.txt"); err == nil {
		body.Close()
		t.Fatal("pending upload was readable")
	}
	if err := storage.Put(ctx, "/pending.txt", bytes.NewReader([]byte("bad")), 3, "text/plain"); err == nil {
		t.Fatal("pending upload was overwriteable")
	}
	if err := storage.Remove(ctx, "/pending.txt"); err == nil {
		t.Fatal("pending upload was deletable")
	}
	if err := storage.Copy(ctx, "/visible.txt", "/pending.txt"); err == nil {
		t.Fatal("copy could overwrite pending upload")
	}
	files, err := storage.List(ctx, "/")
	if err != nil || len(files) != 1 || files[0].GetName() != "visible.txt" {
		t.Fatalf("pending upload appeared in directory: %v %#v", err, files)
	}
	if mutations != 0 {
		t.Fatal("protected pending paths reached the R2 transport")
	}
}
