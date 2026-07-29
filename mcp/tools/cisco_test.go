package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/cisco"
	"infrastructure-mcp/internal/inventory"
)

type fakeCiscoDiagnostics struct {
	config     string
	version    cisco.VersionInfo
	interfaces []cisco.InterfaceEntry
	inventory  []cisco.InventoryEntry
	logs       []string
	err        error
	sawDevice  string
	sawLimit   int
}

func (f *fakeCiscoDiagnostics) Backup(_ context.Context, target string) (string, error) {
	f.sawDevice = target
	return f.config, f.err
}
func (f *fakeCiscoDiagnostics) ShowVersion(_ context.Context, target string) (cisco.VersionInfo, error) {
	f.sawDevice = target
	return f.version, f.err
}
func (f *fakeCiscoDiagnostics) Interfaces(_ context.Context, target string) ([]cisco.InterfaceEntry, error) {
	f.sawDevice = target
	return f.interfaces, f.err
}
func (f *fakeCiscoDiagnostics) Inventory(_ context.Context, target string) ([]cisco.InventoryEntry, error) {
	f.sawDevice = target
	return f.inventory, f.err
}
func (f *fakeCiscoDiagnostics) Logs(_ context.Context, target string, limit int) ([]string, error) {
	f.sawDevice, f.sawLimit = target, limit
	return f.logs, f.err
}

func newCiscoSession(t *testing.T, diag *fakeCiscoDiagnostics) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterCiscoBackup(server, testLogger(), diag)
	RegisterCiscoVersion(server, testLogger(), diag)
	RegisterCiscoInterfaces(server, testLogger(), diag)
	RegisterCiscoInventory(server, testLogger(), diag)
	RegisterCiscoLogs(server, testLogger(), diag)

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

func TestCiscoBackup_ViaMCPProtocol(t *testing.T) {
	diag := &fakeCiscoDiagnostics{config: "hostname router1\n!\n"}
	session := newCiscoSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_backup",
		Arguments: map[string]any{"device": "core"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawDevice != "core" {
		t.Errorf("sawDevice = %q, want core", diag.sawDevice)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out CiscoBackupOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Config != "hostname router1\n!\n" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestCiscoVersion_ViaMCPProtocol(t *testing.T) {
	diag := &fakeCiscoDiagnostics{version: cisco.VersionInfo{Hostname: "router1", VersionLine: "Cisco IOS Software, ...", Uptime: "3 weeks"}}
	session := newCiscoSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_version",
		Arguments: map[string]any{"device": "core"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out CiscoVersionOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Hostname != "router1" || out.Uptime != "3 weeks" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestCiscoInterfaces_ViaMCPProtocol(t *testing.T) {
	diag := &fakeCiscoDiagnostics{interfaces: []cisco.InterfaceEntry{
		{Interface: "FastEthernet0/0", IPAddress: "192.168.1.1", Status: "up", Protocol: "up"},
	}}
	session := newCiscoSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_interfaces",
		Arguments: map[string]any{"device": "core"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out CiscoInterfacesOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Interfaces) != 1 || out.Interfaces[0].Interface != "FastEthernet0/0" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestCiscoInventory_ViaMCPProtocol(t *testing.T) {
	diag := &fakeCiscoDiagnostics{inventory: []cisco.InventoryEntry{
		{Name: "1", Description: "2911 chassis", PID: "CISCO2911/K9", SerialNumber: "FTX1512Q1EF"},
	}}
	session := newCiscoSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_inventory",
		Arguments: map[string]any{"device": "core"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out CiscoInventoryOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Entries) != 1 || out.Entries[0].SerialNumber != "FTX1512Q1EF" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestCiscoLogs_ViaMCPProtocol(t *testing.T) {
	diag := &fakeCiscoDiagnostics{logs: []string{"line1", "line2"}}
	session := newCiscoSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_logs",
		Arguments: map[string]any{"device": "core", "limit": 2},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawLimit != 2 {
		t.Errorf("sawLimit = %d, want 2", diag.sawLimit)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out CiscoLogsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Lines) != 2 {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestCiscoTools_ErrorPath(t *testing.T) {
	diag := &fakeCiscoDiagnostics{err: errors.New("boom")}
	session := newCiscoSession(t, diag)
	ctx := context.Background()

	cases := []struct {
		tool string
		args map[string]any
	}{
		{"cisco_backup", map[string]any{"device": "core"}},
		{"cisco_version", map[string]any{"device": "core"}},
		{"cisco_interfaces", map[string]any{"device": "core"}},
		{"cisco_inventory", map[string]any{"device": "core"}},
		{"cisco_logs", map[string]any{"device": "core"}},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool transport error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result for %s", tc.tool)
			}
		})
	}
}

func TestCiscoBackup_DeviceNotFound(t *testing.T) {
	diag := &fakeCiscoDiagnostics{err: inventory.ErrNotFound}
	session := newCiscoSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_backup",
		Arguments: map[string]any{"device": "does-not-exist"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for a missing device")
	}
}

func TestCiscoBackup_WrongVendor(t *testing.T) {
	diag := &fakeCiscoDiagnostics{err: cisco.ErrWrongVendor}
	session := newCiscoSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "cisco_backup",
		Arguments: map[string]any{"device": "mikrotik-router"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for a non-cisco device")
	}
}
