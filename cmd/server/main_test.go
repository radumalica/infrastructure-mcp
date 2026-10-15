package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/ssh"
)

type stubCommandRunner struct{}

func (stubCommandRunner) Run(context.Context, string, string) (ssh.Result, error) {
	return ssh.Result{}, nil
}

func TestRegisterRunCommand(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		wantPresent bool
	}{
		{name: "disabled by default", enabled: false, wantPresent: false},
		{name: "explicitly enabled", enabled: true, wantPresent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			registerRunCommand(server, logger, stubCommandRunner{}, tt.enabled)
			registerScopedExecTools(server, logger, nil, nil, tt.enabled)

			clientTransport, serverTransport := mcp.NewInMemoryTransports()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			type connectResult struct {
				session *mcp.ServerSession
				err     error
			}
			serverConnected := make(chan connectResult, 1)
			go func() {
				session, err := server.Connect(ctx, serverTransport, nil)
				serverConnected <- connectResult{session: session, err: err}
			}()

			client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
			session, err := client.Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatalf("connect failed: %v", err)
			}
			defer session.Close()

			var connected connectResult
			select {
			case connected = <-serverConnected:
				if connected.err != nil {
					t.Fatalf("server connect failed: %v", connected.err)
				}
				defer connected.session.Close()
			case <-ctx.Done():
				t.Fatalf("server connect did not complete: %v", ctx.Err())
			}

			result, err := session.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools failed: %v", err)
			}

			registered := make(map[string]bool, len(result.Tools))
			for _, tool := range result.Tools {
				registered[tool.Name] = true
			}
			for _, name := range []string{"run_command", "docker_exec", "kubectl_exec"} {
				if registered[name] != tt.wantPresent {
					t.Errorf("%s present = %v, want %v", name, registered[name], tt.wantPresent)
				}
			}
		})
	}
}

func TestRunCommandFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "disabled by default", want: false},
		{name: "explicitly enabled", args: []string{"-enable-run-command"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := flag.NewFlagSet("test", flag.ContinueOnError)
			enabled := registerRunCommandFlag(flags)
			if err := flags.Parse(tt.args); err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if *enabled != tt.want {
				t.Errorf("enable-run-command = %v, want %v", *enabled, tt.want)
			}
		})
	}
}

func TestRunHTTP_ServesHealthzAndMCP(t *testing.T) {
	const addr = "127.0.0.1:18099"

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runHTTP(ctx, server, logger, addr, "") }()
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

func TestRunHTTP_RequiresBearerTokenWhenSet(t *testing.T) {
	const addr = "127.0.0.1:18100"
	const token = "s3cr3t"

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runHTTP(ctx, server, logger, addr, token) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("runHTTP did not shut down within 2s of context cancellation")
		}
	})

	waitForHealthz(t, "http://"+addr+"/healthz")

	// /healthz must stay open even with a token configured.
	healthResp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Errorf("healthz status = %d, want %d", healthResp.StatusCode, http.StatusOK)
	}

	// No Authorization header at all.
	noAuthResp, err := http.Get("http://" + addr + "/mcp")
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	defer noAuthResp.Body.Close()
	if noAuthResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-auth /mcp status = %d, want %d", noAuthResp.StatusCode, http.StatusUnauthorized)
	}

	// Wrong token.
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/mcp", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	wrongResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /mcp with wrong token: %v", err)
	}
	defer wrongResp.Body.Close()
	if wrongResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong-token /mcp status = %d, want %d", wrongResp.StatusCode, http.StatusUnauthorized)
	}

	// Correct token reaches the MCP handler (not rejected as 401).
	req2, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/mcp", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	okResp, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("GET /mcp with correct token: %v", err)
	}
	defer okResp.Body.Close()
	if okResp.StatusCode == http.StatusUnauthorized {
		t.Error("correct-token /mcp request was rejected as unauthorized")
	}
}

func TestResolveHTTPToken(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		allowAnonymous bool
		wantErr        bool
	}{
		{"token set", "abc", false, false},
		{"empty, anonymous allowed", "", true, false},
		{"empty, anonymous not allowed", "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveHTTPToken(tt.token, tt.allowAnonymous, ":8080")
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveHTTPToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.token {
				t.Errorf("resolveHTTPToken() = %q, want %q", got, tt.token)
			}
		})
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
