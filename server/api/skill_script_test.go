package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fileflow/server/store"
)

func skillTestPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		binary, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = exec.CommandContext(ctx, binary, "-c", "import sys; sys.exit(0 if sys.version_info >= (3, 10) else 1)").Run()
		cancel()
		if err == nil {
			return binary
		}
	}
	t.Skip("Python 3.10+ is not installed; the FileFlow server does not require Python")
	return ""
}

func TestSkillScriptUploadsLargerThanMCPContentLimitThroughRealAPI(t *testing.T) {
	python := skillTestPython(t)
	router, _, _ := setupExpirationAPI(t)
	account, remote := relayTestAccount(t)
	account.Quota.MaxSizeBytes = 20 * 1024 * 1024
	account.Permissions.ClientUpload = false // The script uses its API Token channel.
	if err := store.UpdateAccount(account); err != nil {
		t.Fatal(err)
	}
	member, _ := teamUser(t, "skill-upload-member", store.RoleMember)
	token := &store.Token{Name: "skill-script-token", UserID: member.ID, Permissions: []string{"read", "write"}}
	if err := store.CreateToken(token); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	script, err := filepath.Abs(filepath.Join("..", "..", "skills", "fileflow", "scripts", "fileflow.py"))
	if err != nil {
		t.Fatal(err)
	}
	const size = 9 * 1024 * 1024
	payload := bytes.Repeat([]byte("FileFlow skill payload\n"), size/22+1)[:size]
	local := filepath.Join(t.TempDir(), "团队上传报告.txt")
	if err := os.WriteFile(local, payload, 0600); err != nil {
		t.Fatal(err)
	}
	const requestID = "skill-real-api-9m"
	invoke := func() map[string]any {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, python, script, "upload", local, "--account", account.ID, "--path", "skill-tests", "--keep-name", "--expiration-days", "0", "--request-id", requestID)
		for _, variable := range os.Environ() {
			name := strings.SplitN(variable, "=", 2)[0]
			if !strings.EqualFold(name, "FILEFLOW_URL") && !strings.EqualFold(name, "FILEFLOW_TOKEN") && !strings.EqualFold(name, "FILEFLOW_MCP_TOKEN") {
				command.Env = append(command.Env, variable)
			}
		}
		command.Env = append(command.Env, "FILEFLOW_URL="+server.URL, "FILEFLOW_TOKEN="+token.Token)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		output, err := command.Output()
		if err != nil {
			t.Fatalf("skill upload failed: %v; stdout=%s; stderr=%s", err, output, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("skill emitted unexpected stderr: %s", stderr.String())
		}
		if bytes.Contains(output, []byte(token.Token)) {
			t.Fatal("the script printed its bearer credential")
		}
		var result map[string]any
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("invalid script result: %v", err)
		}
		if result["status"] != float64(200) || result["isError"] != false || result["requestId"] != requestID {
			t.Fatalf("unexpected skill result: %s", output)
		}
		return result
	}
	first := invoke()
	data, ok := first["data"].(map[string]any)
	if !ok || data["key"] != "skill-tests/团队上传报告.txt" || data["size"] != float64(size) || data["id"] != account.ID {
		t.Fatalf("script upload lost its file name, account or size: %+v", first)
	}
	key := data["key"].(string)
	digest := sha256.Sum256(payload)
	remote.Lock()
	object, exists := remote.objects[key]
	puts, heads, objectCount := remote.puts, remote.heads, len(remote.objects)
	remote.Unlock()
	if !exists || objectCount != 1 || puts != 1 || heads != 2 || !bytes.Equal(object.body, payload) || object.digest != hex.EncodeToString(digest[:]) {
		t.Fatalf("script did not pass through verified relay upload: exists=%v objects=%d puts=%d heads=%d digest=%s", exists, objectCount, puts, heads, object.digest)
	}
	usage, err := store.GetAccountByID(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Usage.SizeBytes != size || usage.Usage.ClassAOps != 1 || usage.Usage.ClassBOps != 2 {
		t.Fatalf("script bypassed upload quota/counting: %+v", usage.Usage)
	}
	entries := store.GetAuditEntries(10)
	if len(entries) != 1 || entries[0].UserID != member.ID || entries[0].Status != 200 || entries[0].Target != account.ID+"/"+key {
		t.Fatalf("script upload lost its audit identity: %+v", entries)
	}
	second := invoke()
	if second["data"].(map[string]any)["key"] != key {
		t.Fatal("same requestId created a second file")
	}
	remote.Lock()
	puts, heads, objectCount = remote.puts, remote.heads, len(remote.objects)
	remote.Unlock()
	if puts != 1 || heads != 2 || objectCount != 1 {
		t.Fatalf("idempotent script retry repeated R2 work: puts=%d heads=%d objects=%d", puts, heads, objectCount)
	}
	after, _ := store.GetAccountByID(account.ID)
	if after.Usage != usage.Usage || len(store.GetAuditEntries(10)) != len(entries) {
		t.Fatal("idempotent script retry double-counted capacity, operations or audit")
	}
}
