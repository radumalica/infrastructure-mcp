package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/ssh"
)

type fakeCommandRunner struct {
	result ssh.Result
	err    error
}

func (f *fakeCommandRunner) Run(_ context.Context, server, command string) (ssh.Result, error) {
	return f.result, f.err
}

func TestRunCommand_ViaMCPProtocol(t *testing.T) {
	runner := &fakeCommandRunner{result: ssh.Result{Stdout: "hello\n", ExitCode: 0}}

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterRunCommand(server, testLogger(), runner)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	go func() { _, _ = server.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "run_command",
		Arguments: map[string]any{"server": "archive", "command": "echo hello"},
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %+v", result.Content)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out RunCommandOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.Stdout != "hello\n" {
		t.Errorf("unexpected stdout: %q", out.Stdout)
	}
	if out.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestRunCommand_ErrorPath_ReturnsStructuredEnvelope(t *testing.T) {
	runner := &fakeCommandRunner{err: errors.New("ssh: dial archive: connection refused")}

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterRunCommand(server, testLogger(), runner)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	go func() { _, _ = server.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "run_command",
		Arguments: map[string]any{"server": "archive", "command": "uptime"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var envelope structuredToolError
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("error content is not the structured JSON envelope: %v (%s)", err, text.Text)
	}
	if envelope.Category == "" {
		t.Error("expected non-empty category")
	}
}
