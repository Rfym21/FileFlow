package service

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"fileflow/server/config"
	"fileflow/server/store"
)

func setupPreviewAccount(t *testing.T, endpoint string) *store.Account {
	t.Helper()
	dir := t.TempDir()
	databaseURL := "sqlite:" + filepath.Join(dir, "preview.db")
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
	account := &store.Account{Name: "private mock bucket", IsActive: true, Endpoint: endpoint, BucketName: "bucket", AccessKeyId: "mock-access", SecretAccessKey: "mock-secret"}
	if err := store.CreateAccount(account); err != nil {
		t.Fatal(err)
	}
	return account
}

func TestClassifyPreviewUsesAllowlist(t *testing.T) {
	for _, input := range []struct {
		key, kind, contentType string
	}{
		{"nested/Picture.PNG", "image", "image/png"},
		{"video.mp4", "video", "video/mp4"},
		{"sound.mp3", "audio", "audio/mpeg"},
		{"document.pdf", "pdf", "application/pdf"},
		{"code.tsx", "text", "text/plain; charset=utf-8"},
		{"data.json", "text", "application/json"},
		{"data.csv", "text", "text/csv; charset=utf-8"},
		{"page.html", "text", "text/plain; charset=utf-8"},
		{"graphic.svg", "text", "text/plain; charset=utf-8"},
		{"Dockerfile", "text", "text/plain; charset=utf-8"},
		{"photo.png.exe", "unsupported", "application/octet-stream"},
		{"backup.zip", "zip", "application/zip"},
	} {
		format := classifyPreview(input.key)
		if format.kind != input.kind || format.contentType != input.contentType {
			t.Errorf("%s classification = %#v", input.key, format)
		}
	}
}

func TestFilePreviewSignsPrivateSpecialKeysWithoutRemoteRequests(t *testing.T) {
	var requestCount atomic.Int64
	mockS3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockS3.Close()
	account := setupPreviewAccount(t, mockS3.URL)
	key := "folder/中文 name + #?%2F.png"
	preview, err := GetFilePreview(context.Background(), account.ID, key)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Kind != "image" || preview.ContentType != "image/png" || preview.Text != nil || requestCount.Load() != 0 {
		t.Fatalf("unexpected media preview or remote request: %#v, calls=%d", preview, requestCount.Load())
	}
	endpoint, _ := url.Parse(mockS3.URL)
	for _, input := range []struct{ address, disposition string }{{preview.URL, "inline"}, {preview.DownloadURL, "attachment"}} {
		address, err := url.Parse(input.address)
		if err != nil {
			t.Fatal(err)
		}
		query := address.Query()
		if address.Host != endpoint.Host || address.Path != "/bucket/"+key || query.Get("X-Amz-Expires") != "900" || query.Get("X-Amz-Signature") == "" || query.Get("response-content-type") != "image/png" {
			t.Fatalf("incorrect signed URL: %s", input.address)
		}
		disposition, params, err := mime.ParseMediaType(query.Get("response-content-disposition"))
		if err != nil || disposition != input.disposition || params["filename"] != path.Base(key) {
			t.Fatalf("incorrect disposition: %s", query.Get("response-content-disposition"))
		}
	}
}

func TestFilePreviewTextIsBoundedAndReturnedWithoutExecutingHTML(t *testing.T) {
	for _, input := range []struct {
		name, payload string
		ignoreRange   bool
		truncated     bool
	}{
		{"small-html", "<script>alert('only text')</script>", false, false},
		{"large-range", strings.Repeat("x", maxPreviewTextBytes+1000), false, true},
		{"range-ignored", strings.Repeat("x", maxPreviewTextBytes*4), true, true},
		{"exact-limit", strings.Repeat("x", maxPreviewTextBytes), false, false},
	} {
		t.Run(input.name, func(t *testing.T) {
			var requestCount atomic.Int64
			mockS3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount.Add(1)
				if r.Method != http.MethodGet || r.Header.Get("Range") != fmt.Sprintf("bytes=0-%d", maxPreviewTextBytes) {
					t.Errorf("unexpected request: %s, Range=%s", r.Method, r.Header.Get("Range"))
				}
				payload := input.payload
				if !input.ignoreRange {
					end := min(len(payload), maxPreviewTextBytes+1)
					w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", end-1, len(payload)))
					payload = payload[:end]
					w.WriteHeader(http.StatusPartialContent)
				}
				w.Write([]byte(payload))
			}))
			defer mockS3.Close()
			account := setupPreviewAccount(t, mockS3.URL)
			preview, err := GetFilePreview(context.Background(), account.ID, "safe.html")
			if err != nil {
				t.Fatal(err)
			}
			if preview.Kind != "text" || preview.ContentType != "text/plain; charset=utf-8" || preview.Text == nil || preview.Truncated != input.truncated || requestCount.Load() != 1 {
				t.Fatalf("unexpected preview: %#v, calls=%d", preview, requestCount.Load())
			}
			expected := input.payload[:min(len(input.payload), maxPreviewTextBytes)]
			if *preview.Text != expected {
				t.Fatalf("wrong text preview length=%d expected=%d", len(*preview.Text), len(expected))
			}
		})
	}
}

func TestFilePreviewHandlesEmptyTextInvalidRange(t *testing.T) {
	var requestCount atomic.Int64
	mockS3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			w.Write([]byte(`<Error><Code>InvalidRange</Code><Message>empty object</Message></Error>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockS3.Close()
	account := setupPreviewAccount(t, mockS3.URL)
	preview, err := GetFilePreview(context.Background(), account.ID, "empty.txt")
	if err != nil || preview.Text == nil || *preview.Text != "" || preview.Truncated || requestCount.Load() != 2 {
		t.Fatalf("empty preview = %#v, error=%v, calls=%d", preview, err, requestCount.Load())
	}
}

func TestFilePreviewFallsBackToSafeMetadataTypesWithoutExtension(t *testing.T) {
	for _, input := range []struct {
		name, metadataType, kind, contentType string
	}{
		{"pdf", "application/pdf", "pdf", "application/pdf"},
		{"png", "Image/PNG", "image", "image/png"},
		{"video", "video/webm", "video", "video/webm"},
		{"audio", "audio/mpeg", "audio", "audio/mpeg"},
		{"plain-text", "text/plain; charset=utf-8", "text", "text/plain; charset=utf-8"},
		{"json", "application/json", "text", "application/json"},
		{"safe-html", "text/html", "text", "text/plain; charset=utf-8"},
		{"safe-svg", "image/svg+xml", "text", "text/plain; charset=utf-8"},
		{"unknown-binary", "application/octet-stream", "unsupported", "application/octet-stream"},
		{"unknown-subtype", "image/not-a-supported-type", "unsupported", "application/octet-stream"},
		{"malformed-type", "not a mime type", "unsupported", "application/octet-stream"},
	} {
		t.Run(input.name, func(t *testing.T) {
			var headCount, getCount atomic.Int64
			key := "folder/opaque 中文 name + #?%2F"
			payload := "<script>only source text</script>"
			mockS3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/bucket/"+key {
					t.Errorf("metadata request changed the object key: %s", r.URL.Path)
				}
				switch r.Method {
				case http.MethodHead:
					headCount.Add(1)
					w.Header().Set("Content-Type", input.metadataType)
					w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
					w.WriteHeader(http.StatusOK)
				case http.MethodGet:
					getCount.Add(1)
					if input.kind != "text" || r.Header.Get("Range") != fmt.Sprintf("bytes=0-%d", maxPreviewTextBytes) {
						t.Errorf("unexpected fallback content request: kind=%s Range=%s", input.kind, r.Header.Get("Range"))
					}
					w.Write([]byte(payload))
				default:
					t.Errorf("unexpected metadata method: %s", r.Method)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer mockS3.Close()
			account := setupPreviewAccount(t, mockS3.URL)
			preview, err := GetFilePreview(context.Background(), account.ID, key)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Kind != input.kind || preview.ContentType != input.contentType || headCount.Load() != 1 {
				t.Fatalf("wrong metadata preview: %#v, HEAD calls=%d", preview, headCount.Load())
			}
			address, _ := url.Parse(preview.URL)
			if address.Query().Get("response-content-type") != input.contentType {
				t.Fatal("signed URL content type did not match the safe preview type")
			}
			if input.kind == "text" {
				if getCount.Load() != 1 || preview.Text == nil || *preview.Text != payload {
					t.Fatalf("wrong fallback text content: %#v, GET calls=%d", preview, getCount.Load())
				}
			} else if getCount.Load() != 0 || preview.Text != nil {
				t.Fatalf("media or binary preview fetched content: %#v, GET calls=%d", preview, getCount.Load())
			}
		})
	}
}
