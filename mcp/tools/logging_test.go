package tools

import (
	"context"
	"log/slog"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/inventory"
)

// capturingHandler records every slog.Record it receives, so tests can
// assert on the structured fields a tool execution logged.
type capturingHandler struct {
	records *[]slog.Record
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	*h.records = append(*h.records, r)
	return nil
}
func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

func attrMap(r slog.Record) map[string]any {
	m := map[string]any{}
	r.Attrs(func(a slog.Attr) bool {
		m[a.Key] = a.Value.Any()
		return true
	})
	return m
}

func TestListServers_LogsExecution(t *testing.T) {
	var records []slog.Record
	logger := slog.New(&capturingHandler{records: &records})

	inv, err := inventory.Parse([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("parse inventory: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterListServers(server, logger, inv)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	go func() { _, _ = server.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "my-agent", Version: "1.2.3"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer session.Close()

	if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_servers"}); err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected exactly 1 log record, got %d", len(records))
	}
	attrs := attrMap(records[0])

	if attrs["tool"] != "list_servers" {
		t.Errorf("tool = %v, want list_servers", attrs["tool"])
	}
	if attrs["user"] != "my-agent" {
		t.Errorf("user = %v, want my-agent (the connecting MCP client's name)", attrs["user"])
	}
	if attrs["target"] != "*" {
		t.Errorf("target = %v, want * (no tag filter)", attrs["target"])
	}
	if attrs["result"] != "ok" {
		t.Errorf("result = %v, want ok", attrs["result"])
	}
	if _, ok := attrs["duration_ms"]; !ok {
		t.Error("expected duration_ms to be logged")
	}
}
