package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMCPCORSPreflightStopsBeforeSPA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(corsMiddleware())
	spaReached := false
	router.NoRoute(func(c *gin.Context) {
		spaReached = true
		c.String(http.StatusOK, "SPA")
	})
	request := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "Authorization,MCP-Protocol-Version")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || spaReached || response.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Fatalf("invalid MCP preflight: %d %v SPA=%v", response.Code, response.Header(), spaReached)
	}
}
