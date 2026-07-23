package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/linux"
)

type fakeDiagnostics struct {
	uptime linux.UptimeInfo
	disks  []linux.DiskUsage
	memory linux.MemoryUsage
	err    error
}

func (f *fakeDiagnostics) Uptime(context.Context, string) (linux.UptimeInfo, error) {
	return f.uptime, f.err
}
func (f *fakeDiagnostics) DiskUsage(context.Context, string) ([]linux.DiskUsage, error) {
	return f.disks, f.err
}
func (f *fakeDiagnostics) MemoryUsage(context.Context, string) (linux.MemoryUsage, error) {
	return f.memory, f.err
}

func newDiagnosticsSession(t *testing.T, diag *fakeDiagnostics) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterUptime(server, diag)
	RegisterDiskUsage(server, diag)
	RegisterMemoryUsage(server, diag)

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

func TestUptime_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDiagnostics{uptime: linux.UptimeInfo{
		Uptime: 3661 * time.Second,
		Load1:  0.5, Load5: 0.4, Load15: 0.3,
	}}
	session := newDiagnosticsSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "uptime",
		Arguments: map[string]any{"server": "archive"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out UptimeOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.UptimeSeconds != 3661 {
		t.Errorf("UptimeSeconds = %v, want 3661", out.UptimeSeconds)
	}
}

func TestDiskUsage_SeverityLevels(t *testing.T) {
	tests := []struct {
		name       string
		usedPct    int
		wantStatus string
	}{
		{"low usage is ok", 40, "ok"},
		{"75 percent warns", 75, "warning"},
		{"90 percent is critical", 90, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := &fakeDiagnostics{disks: []linux.DiskUsage{
				{Filesystem: "/dev/sda1", MountPoint: "/", TotalKB: 1000, UsedKB: int64(tt.usedPct * 10), UsedPercent: tt.usedPct},
			}}
			session := newDiagnosticsSession(t, diag)
			ctx := context.Background()

			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "disk_usage",
				Arguments: map[string]any{"server": "archive"},
			})
			if err != nil || result.IsError {
				t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
			}

			raw, _ := json.Marshal(result.StructuredContent)
			var out DiskUsageOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(out.Filesystems) != 1 {
				t.Fatalf("expected 1 filesystem, got %d", len(out.Filesystems))
			}
			if out.Filesystems[0].Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", out.Filesystems[0].Status, tt.wantStatus)
			}
			if tt.wantStatus != "ok" && out.Filesystems[0].Recommendation == "" {
				t.Error("expected non-empty recommendation for non-ok status")
			}
		})
	}
}

func TestMemoryUsage_SeverityLevels(t *testing.T) {
	tests := []struct {
		name       string
		usedPct    float64
		wantStatus string
	}{
		{"low usage is ok", 50, "ok"},
		{"85 percent warns", 85, "warning"},
		{"95 percent is critical", 95, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := &fakeDiagnostics{memory: linux.MemoryUsage{
				TotalKB: 1000, UsedPercent: tt.usedPct,
			}}
			session := newDiagnosticsSession(t, diag)
			ctx := context.Background()

			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "memory_usage",
				Arguments: map[string]any{"server": "archive"},
			})
			if err != nil || result.IsError {
				t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
			}

			raw, _ := json.Marshal(result.StructuredContent)
			var out MemoryUsageOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if out.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", out.Status, tt.wantStatus)
			}
		})
	}
}
