package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/inventory"
)

const testInventoryYAML = `
servers:
  archive:
    hostname: 10.0.0.5
    user: hermes
    key: ~/.ssh/archive
    tags: [linux, ethereum]
  pve01:
    hostname: 10.0.0.2
    user: hermes
    proxyjump: jumpbox
    tags: [proxmox]
`

// newTestSession spins up a real MCP server (with all tools registered)
// wired to an in-process client over the SDK's in-memory transport, proving
// the tools work through the actual MCP protocol rather than as bare Go
// function calls.
func newTestSession(t *testing.T, inv *inventory.Inventory) (*mcp.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterListServers(server, inv)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	go func() {
		_, _ = server.Connect(ctx, serverTransport, nil)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}
	return session, func() { _ = session.Close() }
}

func TestListServers_ViaMCPProtocol(t *testing.T) {
	inv, err := inventory.Parse([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("parse inventory: %v", err)
	}

	session, closeFn := newTestSession(t, inv)
	defer closeFn()
	ctx := context.Background()

	toolsList, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(toolsList.Tools) != 1 || toolsList.Tools[0].Name != "list_servers" {
		t.Fatalf("expected exactly [list_servers], got %+v", toolsList.Tools)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_servers"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %+v", result.Content)
	}

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var out ListServersOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if len(out.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d: %+v", len(out.Servers), out.Servers)
	}
	if out.Servers[0].Name != "archive" || out.Servers[1].Name != "pve01" {
		t.Errorf("unexpected server order/names: %+v", out.Servers)
	}
	if out.Servers[0].Hostname != "10.0.0.5" {
		t.Errorf("unexpected hostname: %s", out.Servers[0].Hostname)
	}
}

func TestListServers_TagFilter_ViaMCPProtocol(t *testing.T) {
	inv, err := inventory.Parse([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("parse inventory: %v", err)
	}

	session, closeFn := newTestSession(t, inv)
	defer closeFn()
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_servers",
		Arguments: map[string]any{"tag": "proxmox"},
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %+v", result.Content)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ListServersOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if len(out.Servers) != 1 || out.Servers[0].Name != "pve01" {
		t.Fatalf("expected [pve01], got %+v", out.Servers)
	}
}
