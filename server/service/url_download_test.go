package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRemoteUploadsCannotReachServerOrPrivateNetworks(t *testing.T) {
	visited := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visited = true
	}))
	defer server.Close()
	for _, address := range []string{
		server.URL, "http://localhost/secret", "http://10.0.0.1/", "http://169.254.169.254/latest/meta-data/",
		"http://[::1]/", "http://[::ffff:127.0.0.1]/", "http://192.168.0.1/", "file:///etc/passwd",
		"http://user:password@example.com/",
	} {
		response, err := DownloadPublicURL(address)
		if response != nil {
			response.Body.Close()
		}
		if !errors.Is(err, ErrUnsafeDownloadURL) {
			t.Fatalf("unsafe address %s: %v", address, err)
		}
	}
	if visited {
		t.Fatal("remote upload reached a server-local HTTP endpoint")
	}
}

func TestRemoteUploadDNSIsValidatedAndDialedWithoutReResolution(t *testing.T) {
	ctx := context.Background()
	for _, ips := range [][]net.IPAddr{
		{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("127.0.0.1")}},
		{{IP: net.ParseIP("100.64.0.1")}},
		{{IP: net.ParseIP("64:ff9b::7f00:1")}},
		{{IP: net.ParseIP("fd00::1")}},
	} {
		called := false
		dial := publicDownloadDial(func(context.Context, string) ([]net.IPAddr, error) { return ips, nil },
			func(context.Context, string, string) (net.Conn, error) { called = true; return nil, nil })
		if _, err := dial(ctx, "tcp", "example.com:80"); !errors.Is(err, ErrUnsafeDownloadURL) || called {
			t.Fatalf("unsafe DNS result was dialed: %v, called %v", err, called)
		}
	}
	resolved, connected := 0, ""
	dial := publicDownloadDial(func(context.Context, string) ([]net.IPAddr, error) {
		resolved++
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}, func(_ context.Context, _, address string) (net.Conn, error) { connected = address; return nil, nil })
	if _, err := dial(ctx, "tcp", "example.com:443"); err != nil || resolved != 1 || connected != "8.8.8.8:443" {
		t.Fatalf("connection was not pinned: resolved=%d target=%s err=%v", resolved, connected, err)
	}
}

func TestRemoteUploadRejectsUnsafeRedirects(t *testing.T) {
	for _, address := range []string{"http://127.0.0.1/", "https://169.254.169.254/", "ftp://example.com/file"} {
		parsed, _ := url.Parse(address)
		if err := publicDownloadRedirect(&http.Request{URL: parsed}, nil); !errors.Is(err, ErrUnsafeDownloadURL) {
			t.Fatalf("unsafe redirect allowed: %s", address)
		}
	}
	parsed, _ := url.Parse("https://example.com/file")
	if err := publicDownloadRedirect(&http.Request{URL: parsed}, nil); err != nil {
		t.Fatal("public redirect was blocked")
	}
}
