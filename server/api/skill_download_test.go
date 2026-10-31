package api

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func testSkillSources() fstest.MapFS {
	return fstest.MapFS{
		"SKILL.md":                         {Data: []byte("---\nname: fileflow\ndescription: FileFlow operations\n---\n操作说明\n")},
		"agents/openai.yaml":               {Data: []byte("interface:\n  display_name: FileFlow\n")},
		"references/operations.md":         {Data: []byte("# 操作与额度\n")},
		"scripts/fileflow.py":              {Data: []byte("#!/usr/bin/env python3\nprint('FileFlow')\n")},
		"scripts/test_fileflow.py":         {Data: []byte("private test fixture")},
		"scripts/__pycache__/fileflow.pyc": {Data: []byte("compiled local cache")},
		".env":                             {Data: []byte("FILEFLOW_TOKEN=must-not-be-in-bundle")},
		"state.json":                       {Data: []byte("private runtime state")},
		"../secret.md":                     {Data: []byte("outside skill root")},
		"references/../secret.md":          {Data: []byte("traversal source")},
	}
}

func TestSkillDownloadContainsOnlyInstallableSources(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sources := testSkillSources()
	router := gin.New()
	if err := SetupSkillRouter(router, sources); err != nil {
		t.Fatal(err)
	}
	// Installation assets are public. No identity or configured database is needed.
	request := httptest.NewRequest(http.MethodGet, "/api/skill/download?file=../secret.md", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("download: %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Disposition") != `attachment; filename="fileflow-skill.zip"` || response.Header().Get("Content-Length") != strconv.Itoa(response.Body.Len()) {
		t.Fatalf("download headers: %v", response.Header())
	}
	archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 4 {
		t.Fatalf("archive file count = %d, want 4", len(archive.File))
	}
	seen := map[string]bool{}
	for _, file := range archive.File {
		name := strings.TrimPrefix(file.Name, "fileflow/")
		expected, ok := sources[name]
		if !ok || !strings.HasPrefix(file.Name, "fileflow/") || !fs.ValidPath(file.Name) {
			t.Fatalf("unexpected archive entry %q", file.Name)
		}
		if name != "SKILL.md" && name != "agents/openai.yaml" && name != "references/operations.md" && name != "scripts/fileflow.py" {
			t.Fatalf("non-installable file distributed: %q", file.Name)
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil || !bytes.Equal(content, expected.Data) {
			t.Fatalf("archive entry bytes differ for %q: %v", file.Name, err)
		}
		if name == "scripts/fileflow.py" && file.Mode().Perm() != 0755 {
			t.Fatalf("script mode = %v", file.Mode())
		}
		seen[name] = true
	}
	if len(seen) != 4 {
		t.Fatal("archive has missing or repeated entries")
	}
	// The snapshot and cache validator are stable across downloads.
	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/skill/download", nil))
	if !bytes.Equal(response.Body.Bytes(), second.Body.Bytes()) {
		t.Fatal("static bundle changed between requests")
	}
	for _, validator := range []string{response.Header().Get("ETag"), "W/" + response.Header().Get("ETag"), `"another-bundle", ` + response.Header().Get("ETag"), "*"} {
		request := httptest.NewRequest(http.MethodGet, "/api/skill/download", nil)
		request.Header.Set("If-None-Match", validator)
		revalidated := httptest.NewRecorder()
		router.ServeHTTP(revalidated, request)
		if revalidated.Code != http.StatusNotModified || revalidated.Body.Len() != 0 {
			t.Fatalf("cache revalidation %q: %d", validator, revalidated.Code)
		}
	}
}

func TestSkillBundleRejectsMissingOrInvalidRequiredFiles(t *testing.T) {
	for _, missing := range skillBundleFiles {
		t.Run("missing_"+missing, func(t *testing.T) {
			sources := testSkillSources()
			delete(sources, missing)
			router := gin.New()
			if err := SetupSkillRouter(router, sources); err == nil {
				t.Fatal("missing source produced an installable bundle")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/skill/download", nil))
			if response.Code != http.StatusNotFound {
				t.Fatal("partial bundle route was registered")
			}
		})
	}
	for _, mode := range []fs.FileMode{fs.ModeDir | 0755, fs.ModeSymlink | 0644} {
		sources := testSkillSources()
		sources["scripts/fileflow.py"] = &fstest.MapFile{Data: []byte("target"), Mode: mode}
		if _, err := buildSkillBundle(sources); err == nil {
			t.Fatalf("non-regular source mode %v was accepted", mode)
		}
	}
	sources := testSkillSources()
	sources["SKILL.md"].Data = nil
	if _, err := buildSkillBundle(sources); err == nil {
		t.Fatal("empty skill entrypoint was accepted")
	}
	if _, err := buildSkillBundle(nil); err == nil {
		t.Fatal("nil source was accepted")
	}
}
