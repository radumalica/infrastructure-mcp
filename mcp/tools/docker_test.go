package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/docker"
	"infrastructure-mcp/internal/ssh"
)

type fakeDockerDiagnostics struct {
	containers   []docker.Container
	images       []docker.Image
	stats        []docker.ContainerStats
	logs         string
	restartErr   error
	err          error
	sawAll       bool
	sawContainer string
	sawTail      int
}

func (f *fakeDockerDiagnostics) Ps(_ context.Context, _ string, all bool) ([]docker.Container, error) {
	f.sawAll = all
	return f.containers, f.err
}
func (f *fakeDockerDiagnostics) Images(context.Context, string) ([]docker.Image, error) {
	return f.images, f.err
}
func (f *fakeDockerDiagnostics) Stats(_ context.Context, _, container string) ([]docker.ContainerStats, error) {
	f.sawContainer = container
	return f.stats, f.err
}
func (f *fakeDockerDiagnostics) Logs(_ context.Context, _, container string, tail int) (string, error) {
	f.sawContainer = container
	f.sawTail = tail
	return f.logs, f.err
}
func (f *fakeDockerDiagnostics) Restart(_ context.Context, _, container string) error {
	f.sawContainer = container
	if f.restartErr != nil {
		return f.restartErr
	}
	return f.err
}

func newDockerSession(t *testing.T, diag *fakeDockerDiagnostics) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterDockerPs(server, testLogger(), diag)
	RegisterDockerImages(server, testLogger(), diag)
	RegisterDockerStats(server, testLogger(), diag)
	RegisterDockerLogs(server, testLogger(), diag)
	RegisterDockerRestart(server, testLogger(), diag)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go func() { _, _ = server.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestDockerPs_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDockerDiagnostics{containers: []docker.Container{
		{ID: "abc123", Image: "nginx", Names: "web", State: "running"},
	}}
	session := newDockerSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "docker_ps",
		Arguments: map[string]any{"server": "archive", "all": true},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if !diag.sawAll {
		t.Error("expected all=true to reach the diagnostics layer")
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out DockerPsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Containers) != 1 || out.Containers[0].Names != "web" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestDockerImages_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDockerDiagnostics{images: []docker.Image{
		{ID: "sha256:abc", Repository: "nginx", Tag: "latest", Size: "142MB"},
	}}
	session := newDockerSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "docker_images",
		Arguments: map[string]any{"server": "archive"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var out DockerImagesOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Images) != 1 || out.Images[0].Repository != "nginx" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestDockerStats_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDockerDiagnostics{stats: []docker.ContainerStats{
		{ID: "abc123", Name: "web", CPUPercent: "1.5%"},
	}}
	session := newDockerSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "docker_stats",
		Arguments: map[string]any{"server": "archive", "container": "web"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawContainer != "web" {
		t.Errorf("sawContainer = %q, want web", diag.sawContainer)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var out DockerStatsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Stats) != 1 || out.Stats[0].Name != "web" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestDockerLogs_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDockerDiagnostics{logs: "line1\nline2\n"}
	session := newDockerSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "docker_logs",
		Arguments: map[string]any{"server": "archive", "container": "web", "tail": 50},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawTail != 50 {
		t.Errorf("sawTail = %d, want 50", diag.sawTail)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var out DockerLogsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Logs != "line1\nline2\n" || out.LineCount != 2 {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestDockerRestart_RequiresConfirmation(t *testing.T) {
	diag := &fakeDockerDiagnostics{}
	session := newDockerSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "docker_restart",
		Arguments: map[string]any{"server": "archive", "container": "web"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawContainer != "" {
		t.Error("expected Restart not to be called without confirmation")
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out DockerRestartOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "confirmation_required" {
		t.Errorf("Status = %q, want confirmation_required", out.Status)
	}
}

func TestDockerRestart_Confirmed(t *testing.T) {
	diag := &fakeDockerDiagnostics{}
	session := newDockerSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "docker_restart",
		Arguments: map[string]any{"server": "archive", "container": "web", "confirm": true},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawContainer != "web" {
		t.Error("expected Restart to be called once confirmed")
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out DockerRestartOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "restarted" {
		t.Errorf("Status = %q, want restarted", out.Status)
	}
}

func TestDockerRestart_ConfirmedButFails(t *testing.T) {
	diag := &fakeDockerDiagnostics{restartErr: errors.New("docker: no such container")}
	session := newDockerSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "docker_restart",
		Arguments: map[string]any{"server": "archive", "container": "web", "confirm": true},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for a failing restart")
	}
}

// fakeDockerRunner implements docker.Runner directly (rather than
// DockerDiagnostics), so TestDockerLogs_RejectsShellInjection can drive a
// real *docker.Client end-to-end: MCP protocol -> tool handler ->
// docker.Client.Logs -> validateContainerRef -> toolerr -> structured
// envelope. This is the one place that composition, not just each link,
// gets exercised.
type fakeDockerRunner struct{}

func (fakeDockerRunner) Run(context.Context, string, string) (ssh.Result, error) {
	return ssh.Result{}, errors.New("fakeDockerRunner: should never be called for a rejected input")
}

// TestDockerLogs_RejectsShellInjection proves the no-shell-interpolation
// guarantee end-to-end: a container reference containing shell
// metacharacters must never reach the remote command, and must surface as
// the structured invalid_input envelope, not a raw error or a silent
// pass-through.
func TestDockerLogs_RejectsShellInjection(t *testing.T) {
	ctx := context.Background()
	realClient := docker.New(fakeDockerRunner{})

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterDockerLogs(server, testLogger(), realClient)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go func() { _, _ = server.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "docker_logs",
		Arguments: map[string]any{"server": "archive", "container": "web; rm -rf /"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for a shell-metacharacter container reference")
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var envelope structuredToolError
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("error content is not the structured JSON envelope: %v (%s)", err, text.Text)
	}
	if envelope.Category != "invalid_input" {
		t.Errorf("Category = %q, want invalid_input", envelope.Category)
	}
}

// TestDockerTools_ErrorPath verifies that a failure from the docker
// diagnostics layer reaches the client as the structured envelope
// required by README.md's Error Handling section.
func TestDockerTools_ErrorPath(t *testing.T) {
	underlying := errors.New("docker: dial archive: connection refused")
	diag := &fakeDockerDiagnostics{err: underlying}
	session := newDockerSession(t, diag)
	ctx := context.Background()

	tests := []struct {
		name string
		args map[string]any
	}{
		{"docker_ps", map[string]any{"server": "archive"}},
		{"docker_images", map[string]any{"server": "archive"}},
		{"docker_stats", map[string]any{"server": "archive"}},
		{"docker_logs", map[string]any{"server": "archive", "container": "web"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tt.name, Arguments: tt.args})
			if err != nil {
				t.Fatalf("CallTool transport error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected IsError=true for a failing diagnostics call")
			}
			text, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				t.Fatalf("expected TextContent, got %T", result.Content[0])
			}
			var envelope structuredToolError
			if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
				t.Fatalf("error content is not the structured JSON envelope: %v (%s)", err, text.Text)
			}
			if envelope.Category == "" || envelope.Message == "" {
				t.Errorf("incomplete envelope: %+v", envelope)
			}
		})
	}
}
