package service

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"fileflow/server/store"
)

type relayBoundedReader struct {
	remaining, read int64
}

func (reader *relayBoundedReader) Read(output []byte) (int, error) {
	if reader.remaining <= 0 {
		return 0, io.EOF
	}
	count := min(int64(len(output)), reader.remaining)
	for i := int64(0); i < count; i++ {
		output[i] = 'x'
	}
	reader.remaining -= count
	reader.read += count
	return int(count), nil
}

func TestRelayStreamingStopsAtRemainingCapacityAndRemovesTemporaryFiles(t *testing.T) {
	account, actor, mock := setupMCPFileOperation(t)
	account.Quota.MaxSizeBytes = 9 // Six existing bytes leave three for this upload.
	if err := store.UpdateAccount(account); err != nil {
		t.Fatal(err)
	}
	spoolDirectory := t.TempDir()
	t.Setenv("TMP", spoolDirectory)
	t.Setenv("TEMP", spoolDirectory)
	t.Setenv("TMPDIR", spoolDirectory)
	body := &relayBoundedReader{remaining: 1 << 30}
	_, _, err := UploadRelay(context.Background(), actor, UploadRequest{Filename: "large.bin", IDGroup: account.ID}, body)
	if !errors.Is(err, store.ErrUploadQuotaExceeded) || body.read != 4 || mock.heads != 0 || mock.copies != 0 || mock.puts != 0 {
		t.Fatalf("stream exceeded bounded capacity or wrote to R2: read=%d err=%v", body.read, err)
	}
	entries, err := os.ReadDir(spoolDirectory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed upload retained temporary file: %v %v", entries, err)
	}
}

func TestRelayUploadRejectsMoreThanFiveGiBBeforeReadingBody(t *testing.T) {
	account, actor, mock := setupMCPFileOperation(t)
	size := store.MaxUploadFileSize + 1
	body := &relayBoundedReader{remaining: size}
	_, _, err := UploadRelay(context.Background(), actor, UploadRequest{Filename: "oversize.bin", IDGroup: account.ID, Size: &size}, body)
	if !errors.Is(err, store.ErrInvalidUploadSession) || body.read != 0 || mock.heads != 0 || len(store.ListUploadSessions()) != 0 {
		t.Fatalf("oversized upload read or reserved work: read=%d err=%v", body.read, err)
	}
}

func TestRelayStreamingRefusesShorterBodyThanDeclared(t *testing.T) {
	account, actor, mock := setupMCPFileOperation(t)
	size := int64(3)
	body := &relayBoundedReader{remaining: 2}
	_, _, err := UploadRelay(context.Background(), actor, UploadRequest{Filename: "short.bin", IDGroup: account.ID, Size: &size}, body)
	if !errors.Is(err, store.ErrInvalidUploadSession) || mock.heads != 0 || mock.copies != 0 || len(store.ListUploadSessions()) != 0 {
		t.Fatalf("short body uploaded or reserved work: %v", err)
	}
}
