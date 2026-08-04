package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/backupstore"
)

// newCiscoBackupDiffSession wires cisco_backup_diff against diag and a
// real *backupstore.Store rooted at t.TempDir() — the store's own logic
// is unit-tested in internal/backupstore; this only needs to prove the
// MCP-protocol wiring (diag.Backup's output reaches store.SaveAndDiff,
// and the result reaches the tool's structured output).
func newCiscoBackupDiffSession(t *testing.T, diag *fakeCiscoDiagnostics) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterCiscoBackupDiff(server, testLogger(), diag, backupstore.New(t.TempDir()))

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

func TestCiscoBackupDiff_FirstSnapshot(t *testing.T) {
	diag := &fakeCiscoDiagnostics{config: "hostname router1\n!\n"}
	session := newCiscoBackupDiffSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_backup_diff",
		Arguments: map[string]any{"device": "core-sw"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out CiscoBackupDiffOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.FirstSnapshot || out.Changed || out.Diff != "" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestCiscoBackupDiff_DetectsChange(t *testing.T) {
	diag := &fakeCiscoDiagnostics{config: "hostname router1\ninterface Gi0/1\n!\n"}
	session := newCiscoBackupDiffSession(t, diag)
	ctx := context.Background()

	if _, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_backup_diff",
		Arguments: map[string]any{"device": "core-sw"},
	}); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	diag.config = "hostname router1\ninterface Gi0/2\n!\n"
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_backup_diff",
		Arguments: map[string]any{"device": "core-sw"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out CiscoBackupDiffOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.FirstSnapshot || !out.Changed || out.Diff == "" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestCiscoBackupDiff_BackupError(t *testing.T) {
	diag := &fakeCiscoDiagnostics{err: errors.New("boom")}
	session := newCiscoBackupDiffSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_backup_diff",
		Arguments: map[string]any{"device": "core-sw"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
}
