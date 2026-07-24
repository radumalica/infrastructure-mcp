package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRunHTTP_ServesHealthzAndMCP(t *testing.T) {
	const addr = "127.0.0.1:18099"

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runHTTP(ctx, server, logger, addr) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("runHTTP did not shut down within 2s of context cancellation")
		}
	})

	waitForHealthz(t, "http://"+addr+"/healthz")

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// /mcp must be routed to the MCP handler, not 404. A GET without an
	// established session is expected to be rejected by the handler
	// itself (not by the mux failing to route it at all).
	resp2, err := http.Get("http://" + addr + "/mcp")
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode == http.StatusNotFound {
		t.Error("/mcp was not routed to the MCP handler")
	}
}

func TestRunHealthcheck(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer ok.Close()
	if err := runHealthcheck(ok.URL); err != nil {
		t.Errorf("expected success for a 200 response, got %v", err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer bad.Close()
	if err := runHealthcheck(bad.URL); err == nil {
		t.Error("expected failure for a 503 response")
	}

	if err := runHealthcheck("http://127.0.0.1:1"); err == nil {
		t.Error("expected failure when the connection is refused")
	}
}

func waitForHealthz(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become ready in time", url)
}
