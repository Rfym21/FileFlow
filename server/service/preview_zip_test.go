package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fileflow/server/store"
)

type zipTestEntry struct {
	name, contents string
	method         uint16
}

func makePreviewTestZIP(t *testing.T, entries []zipTestEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		file, err := writer.CreateHeader(&zip.FileHeader{Name: entry.name, Method: entry.method, Modified: time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entry.contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

type zipMockStats struct {
	heads, gets, rangeBytes atomic.Int64
}

func mockPreviewZIP(t *testing.T, contents []byte, mimeType string, ignoreRange bool) (*httptest.Server, *zipMockStats) {
	t.Helper()
	stats := &zipMockStats{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			stats.heads.Add(1)
			w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Content-Length", strconv.Itoa(len(contents)))
			w.Header().Set("ETag", `"unchanged"`)
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("ZIP preview made unexpected request %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		stats.gets.Add(1)
		if r.Header.Get("If-Match") != `"unchanged"` {
			t.Error("ZIP metadata ranges should retain the original object version")
		}
		if ignoreRange {
			w.Write(contents)
			return
		}
		var start, end int64
		if count, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil || count != 2 || start < 0 || end < start || end >= int64(len(contents)) {
			t.Errorf("invalid ZIP range %s", r.Header.Get("Range"))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		stats.rangeBytes.Add(end - start + 1)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(contents)))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(contents[start : end+1])
	}))
	return server, stats
}

func TestZIPPreviewListsOnlyDirectoryMetadata(t *testing.T) {
	t.Setenv("GODEBUG", "zipinsecurepath=0")
	contents := makePreviewTestZIP(t, []zipTestEntry{
		{name: "folder/"},
		{name: "folder/中文 + #?.txt", contents: strings.Repeat("compress me", 200), method: zip.Deflate},
		{name: "../../outside.txt", contents: "only a listed name", method: zip.Store},
		{name: "<script>.html", contents: "not opened", method: zip.Store},
	})
	server, stats := mockPreviewZIP(t, contents, "application/zip", false)
	defer server.Close()
	account := setupPreviewAccount(t, server.URL)
	preview, err := GetFilePreview(context.Background(), account.ID, "archive.zip")
	if err != nil || preview.ArchiveError != "" || preview.Kind != "zip" || preview.EntryCount != 4 || len(preview.Entries) != 4 || preview.Truncated || preview.URL == "" || preview.DownloadURL == "" {
		t.Fatalf("ZIP preview = %#v, error=%v", preview, err)
	}
	if stats.heads.Load() != 1 || stats.gets.Load() == 0 || stats.rangeBytes.Load() > maxZIPMetadataBytes {
		t.Fatalf("unexpected ZIP metadata requests: HEAD=%d GET=%d bytes=%d", stats.heads.Load(), stats.gets.Load(), stats.rangeBytes.Load())
	}
	if !preview.Entries[0].IsDir || preview.Entries[0].Size != 0 || preview.Entries[1].Name != "folder/中文 + #?.txt" || preview.Entries[1].Size != 2200 || preview.Entries[1].CompressedSize >= preview.Entries[1].Size || preview.Entries[1].ModifiedAt != "2026-10-31T08:00:00Z" {
		t.Fatalf("incorrect ZIP entry metadata: %#v", preview.Entries)
	}
	if preview.Entries[2].Name != "../../outside.txt" || preview.Entries[3].Name != "<script>.html" {
		t.Fatal("ZIP preview should list the original names without extracting or normalizing paths")
	}
}

func TestZIPPreviewUsesMetadataRangesRatherThanWholeArchive(t *testing.T) {
	contents := makePreviewTestZIP(t, []zipTestEntry{{name: "large.bin", contents: strings.Repeat("x", 2*maxZIPMetadataBytes), method: zip.Store}})
	server, stats := mockPreviewZIP(t, contents, "application/zip", false)
	defer server.Close()
	account := setupPreviewAccount(t, server.URL)
	preview, err := GetFilePreview(context.Background(), account.ID, "large.zip")
	if err != nil || preview.ArchiveError != "" || len(preview.Entries) != 1 || preview.Entries[0].Size != 2*maxZIPMetadataBytes {
		t.Fatalf("large ZIP metadata = %#v, error=%v", preview, err)
	}
	if stats.rangeBytes.Load() >= 3*zipPreviewBlockBytes {
		t.Fatalf("ZIP preview read file payload rather than a small metadata window: %d bytes", stats.rangeBytes.Load())
	}
}

func TestZIPPreviewTruncatesDirectoryToOneThousandEntries(t *testing.T) {
	entries := make([]zipTestEntry, maxZIPPreviewEntries+1)
	for i := range entries {
		entries[i].name = fmt.Sprintf("folder/file-%04d.txt", i)
	}
	contents := makePreviewTestZIP(t, entries)
	server, _ := mockPreviewZIP(t, contents, "application/zip", false)
	defer server.Close()
	account := setupPreviewAccount(t, server.URL)
	preview, err := GetFilePreview(context.Background(), account.ID, "many.zip")
	if err != nil || preview.ArchiveError != "" || !preview.Truncated || preview.EntryCount != len(entries) || len(preview.Entries) != maxZIPPreviewEntries {
		t.Fatalf("truncated ZIP = %#v, error=%v", preview, err)
	}
}

func TestZIPPreviewPreservesDownloadForInvalidOrUnboundedArchives(t *testing.T) {
	normal := makePreviewTestZIP(t, []zipTestEntry{{name: "one.txt", contents: "one"}})
	oversizedDirectory := append([]byte(nil), normal...)
	binary.LittleEndian.PutUint32(oversizedDirectory[len(oversizedDirectory)-10:], maxZIPMetadataBytes+1)
	tooManyEntries := append([]byte(nil), normal...)
	binary.LittleEndian.PutUint16(tooManyEntries[len(tooManyEntries)-14:], maxZIPDirectoryEntries+1)
	binary.LittleEndian.PutUint16(tooManyEntries[len(tooManyEntries)-12:], maxZIPDirectoryEntries+1)
	falseCountDirectory := make([]byte, 46*(maxZIPDirectoryEntries+1)+22)
	for offset := 0; offset < len(falseCountDirectory)-22; offset += 46 {
		binary.LittleEndian.PutUint32(falseCountDirectory[offset:], 0x02014b50)
	}
	footer := falseCountDirectory[len(falseCountDirectory)-22:]
	binary.LittleEndian.PutUint32(footer, 0x06054b50)
	binary.LittleEndian.PutUint16(footer[8:], 1)
	binary.LittleEndian.PutUint16(footer[10:], 1)
	binary.LittleEndian.PutUint32(footer[12:], uint32(len(falseCountDirectory)-22))
	for _, input := range []struct {
		name, errorContains string
		contents            []byte
		ignoreRange         bool
	}{
		{"corrupt", "ZIP", []byte("this is not a ZIP archive"), false},
		{"ignored-range", "范围读取", normal, true},
		{"oversized-directory", "8 MiB", oversizedDirectory, false},
		{"too-many-entries", "10000", tooManyEntries, false},
		{"false-footer-count", "10000", falseCountDirectory, false},
	} {
		t.Run(input.name, func(t *testing.T) {
			server, _ := mockPreviewZIP(t, input.contents, "application/zip", input.ignoreRange)
			defer server.Close()
			account := setupPreviewAccount(t, server.URL)
			preview, err := GetFilePreview(context.Background(), account.ID, "invalid.zip")
			if err != nil || preview.Kind != "zip" || !strings.Contains(preview.ArchiveError, input.errorContains) || preview.URL == "" || preview.DownloadURL == "" {
				t.Fatalf("ZIP must keep download available: %#v, error=%v", preview, err)
			}
		})
	}
}

func makePreviewZIP64Fixture(normal []byte, padding int) []byte {
	endOffset := len(normal) - 22
	footer := append([]byte(nil), normal[endOffset:]...)
	footer64 := make([]byte, 56)
	binary.LittleEndian.PutUint32(footer64, 0x06064b50)
	binary.LittleEndian.PutUint64(footer64[4:], 44)
	binary.LittleEndian.PutUint16(footer64[12:], 45)
	binary.LittleEndian.PutUint16(footer64[14:], 45)
	binary.LittleEndian.PutUint64(footer64[24:], uint64(binary.LittleEndian.Uint16(footer[8:])))
	binary.LittleEndian.PutUint64(footer64[32:], uint64(binary.LittleEndian.Uint16(footer[10:])))
	binary.LittleEndian.PutUint64(footer64[40:], uint64(binary.LittleEndian.Uint32(footer[12:])))
	binary.LittleEndian.PutUint64(footer64[48:], uint64(binary.LittleEndian.Uint32(footer[16:])))
	locator := make([]byte, 20)
	binary.LittleEndian.PutUint32(locator, 0x07064b50)
	binary.LittleEndian.PutUint64(locator[8:], uint64(endOffset+padding))
	binary.LittleEndian.PutUint32(locator[16:], 1)
	binary.LittleEndian.PutUint16(footer[8:], 0xffff)
	binary.LittleEndian.PutUint16(footer[10:], 0xffff)
	binary.LittleEndian.PutUint32(footer[12:], 0xffffffff)
	binary.LittleEndian.PutUint32(footer[16:], 0xffffffff)
	contents := append([]byte(nil), normal[:endOffset]...)
	contents = append(contents, make([]byte, padding)...)
	contents = append(contents, footer64...)
	contents = append(contents, locator...)
	return append(contents, footer...)
}

func TestZIPPreviewSupportsZIP64FooterAndMIMEFallback(t *testing.T) {
	normal := makePreviewTestZIP(t, []zipTestEntry{{name: "ZIP64.txt", contents: "small valid ZIP64 archive", method: zip.Deflate}})
	contents := makePreviewZIP64Fixture(normal, 0)
	for _, mimeType := range []string{"application/zip", "application/x-zip-compressed"} {
		t.Run(mimeType, func(t *testing.T) {
			server, stats := mockPreviewZIP(t, contents, mimeType, false)
			defer server.Close()
			account := setupPreviewAccount(t, server.URL)
			preview, err := GetFilePreview(context.Background(), account.ID, "opaque-no-extension")
			if err != nil || preview.ArchiveError != "" || preview.Kind != "zip" || preview.EntryCount != 1 || len(preview.Entries) != 1 || preview.Entries[0].Name != "ZIP64.txt" || stats.heads.Load() != 1 {
				t.Fatalf("ZIP64 MIME preview = %#v, error=%v HEAD=%d", preview, err, stats.heads.Load())
			}
		})
	}
}

func TestZIPPreviewValidatesOffsetCandidatesForSFXAndPadding(t *testing.T) {
	normal := makePreviewTestZIP(t, []zipTestEntry{{name: "hello.txt", contents: "hello payload", method: zip.Deflate}})
	prefix := bytes.Repeat([]byte{'A'}, 4096)
	oldOffset := int(binary.LittleEndian.Uint32(normal[len(normal)-6:]))
	decoyPrefix := append([]byte(nil), prefix...)
	binary.LittleEndian.PutUint32(decoyPrefix[oldOffset:], 0x02014b50)
	binary.LittleEndian.PutUint16(decoyPrefix[oldOffset+28:], 0xffff)
	padded := append([]byte(nil), normal[:len(normal)-22]...)
	padded = append(padded, make([]byte, 128)...)
	padded = append(padded, normal[len(normal)-22:]...)
	for _, input := range []struct {
		name     string
		contents []byte
	}{
		{"ordinary-sfx", append(append([]byte(nil), prefix...), normal...)},
		{"sfx-with-invalid-decoy-header", append(decoyPrefix, normal...)},
		{"directory-padding", padded},
		{"zip64-directory-padding", makePreviewZIP64Fixture(normal, 128)},
	} {
		t.Run(input.name, func(t *testing.T) {
			// Prove these fixtures are supported archives with readable real content.
			reference, err := zip.NewReader(bytes.NewReader(input.contents), int64(len(input.contents)))
			if err != nil || len(reference.File) != 1 {
				t.Fatalf("reference ZIP fixture failed: %v", err)
			}
			file, err := reference.File[0].Open()
			if err != nil {
				t.Fatal(err)
			}
			payload, err := io.ReadAll(file)
			file.Close()
			if err != nil || string(payload) != "hello payload" {
				t.Fatalf("reference ZIP content failed: %q, %v", payload, err)
			}
			server, stats := mockPreviewZIP(t, input.contents, "application/zip", false)
			defer server.Close()
			account := setupPreviewAccount(t, server.URL)
			preview, err := GetFilePreview(context.Background(), account.ID, "supported.zip")
			if err != nil || preview.ArchiveError != "" || preview.EntryCount != 1 || len(preview.Entries) != 1 || preview.Entries[0].Name != "hello.txt" {
				t.Fatalf("supported ZIP was rejected: %#v, %v", preview, err)
			}
			if stats.rangeBytes.Load() > maxZIPMetadataBytes {
				t.Fatal("offset validation bypassed the range reading budget")
			}
		})
	}
}

func TestZIPRangeReaderHasFixedBudgetAndCachesBlocks(t *testing.T) {
	const size int64 = 2 * maxZIPMetadataBytes
	var gets atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets.Add(1)
		var start, end int64
		fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(make([]byte, end-start+1))
	}))
	defer server.Close()
	account := &store.Account{Endpoint: server.URL, BucketName: "bucket", AccessKeyId: "mock-access", SecretAccessKey: "mock-secret"}
	reader := &zipRangeReaderAt{ctx: context.Background(), client: newDirectS3Client(account), bucket: account.BucketName, key: "metadata", size: size, blocks: make(map[int64][]byte)}
	buffer := make([]byte, 1)
	for i := 0; i < maxZIPMetadataBytes/zipPreviewBlockBytes; i++ {
		if _, err := reader.ReadAt(buffer, int64(i*zipPreviewBlockBytes)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := reader.ReadAt(buffer, maxZIPMetadataBytes); err == nil || !strings.Contains(err.Error(), "8 MiB") {
		t.Fatalf("unbounded metadata read error = %v", err)
	}
	if _, err := reader.ReadAt(buffer, 0); err != nil || reader.readBytes != maxZIPMetadataBytes || gets.Load() != maxZIPMetadataBytes/zipPreviewBlockBytes {
		t.Fatalf("cached block exceeded budget: bytes=%d GET=%d error=%v", reader.readBytes, gets.Load(), err)
	}
}

func TestZIPPreviewReturnsArchiveErrorWhenMetadataUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	account := setupPreviewAccount(t, server.URL)
	preview, err := GetFilePreview(context.Background(), account.ID, "unavailable.zip")
	if err != nil || preview.ArchiveError == "" || preview.URL == "" || preview.DownloadURL == "" {
		t.Fatalf("metadata failure lost ZIP download: %#v, error=%v", preview, err)
	}
}

func TestZIPPreviewHandlesEmptyArchive(t *testing.T) {
	contents := makePreviewTestZIP(t, nil)
	server, _ := mockPreviewZIP(t, contents, "application/zip", false)
	defer server.Close()
	account := setupPreviewAccount(t, server.URL)
	preview, err := GetFilePreview(context.Background(), account.ID, "empty.zip")
	if err != nil || preview.ArchiveError != "" || preview.EntryCount != 0 || len(preview.Entries) != 0 || preview.Truncated {
		t.Fatalf("empty ZIP = %#v, error=%v", preview, err)
	}
}
