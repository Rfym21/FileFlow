package main

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublicDocumentPagesReturnSPAWithoutGuideRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	setupStaticFiles(router)
	index, err := fs.ReadFile(staticFiles, "client/dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/guide", "/proxy-guide", "/api-docs", "/webdav-docs", "/mcp-docs", "/skill-docs"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Header().Get("Location") != "" || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("public page %s: status=%d headers=%v", path, response.Code, response.Header())
			}
			if !bytes.Equal(response.Body.Bytes(), index) {
				t.Fatalf("public page %s returned a directory or non-SPA response", path)
			}
		})
	}
}

func TestPublicGuideAssetsAndUnknownAPIKeepDistinctResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	setupStaticFiles(router)
	image, err := fs.ReadFile(staticFiles, "client/dist/assets/guide/step1.png")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/guide/step1.png", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || !bytes.Equal(response.Body.Bytes(), image) {
		t.Fatalf("guide image: status=%d headers=%v", response.Code, response.Header())
	}
	unknownAPI := httptest.NewRecorder()
	router.ServeHTTP(unknownAPI, httptest.NewRequest(http.MethodGet, "/api/unknown-document-route", nil))
	if unknownAPI.Code != http.StatusNotFound || !strings.HasPrefix(unknownAPI.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unknown API unexpectedly fell back to public SPA: %d %s", unknownAPI.Code, unknownAPI.Body.String())
	}
}
