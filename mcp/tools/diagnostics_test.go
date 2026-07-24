package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/linux"
)

type fakeDiagnostics struct {
	uptime         linux.UptimeInfo
	disks          []linux.DiskUsage
	memory         linux.MemoryUsage
	failedServices []linux.FailedService
	cpu            linux.CPUUsage
	reboot         linux.RebootRequired
	processes      []linux.ProcessInfo
	journalEntries []linux.JournalEntry
	kernelVersion  string
	err            error
}

type structuredToolError struct {
	Message        string `json:"message"`
	Recommendation string `json:"recommendation"`
	Retryable      bool   `json:"retryable"`
	Category       string `json:"category"`
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
func (f *fakeDiagnostics) FailedServices(context.Context, string) ([]linux.FailedService, error) {
	return f.failedServices, f.err
}
func (f *fakeDiagnostics) CPUUsage(context.Context, string) (linux.CPUUsage, error) {
	return f.cpu, f.err
}
func (f *fakeDiagnostics) RebootRequired(context.Context, string) (linux.RebootRequired, error) {
	return f.reboot, f.err
}
func (f *fakeDiagnostics) RunningProcesses(context.Context, string, int) ([]linux.ProcessInfo, error) {
	return f.processes, f.err
}
func (f *fakeDiagnostics) JournalErrors(context.Context, string, int) ([]linux.JournalEntry, error) {
	return f.journalEntries, f.err
}
func (f *fakeDiagnostics) KernelVersion(context.Context, string) (string, error) {
	return f.kernelVersion, f.err
}

func newDiagnosticsSession(t *testing.T, diag *fakeDiagnostics) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterUptime(server, testLogger(), diag)
	RegisterDiskUsage(server, testLogger(), diag)
	RegisterMemoryUsage(server, testLogger(), diag)
	RegisterFailedServices(server, testLogger(), diag)
	RegisterCPUUsage(server, testLogger(), diag)
	RegisterRebootRequired(server, testLogger(), diag)
	RegisterRunningProcesses(server, testLogger(), diag)
	RegisterJournalErrors(server, testLogger(), diag)
	RegisterKernelVersion(server, testLogger(), diag)

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

// TestDiagnosticsTools_ErrorPath verifies that a failure from the
// diagnostics layer reaches the client as the structured
// message/recommendation/retryable/category envelope required by
// README.md's Error Handling section, not a raw Go error string.
func TestDiagnosticsTools_ErrorPath(t *testing.T) {
	underlying := errors.New("ssh: dial archive: connection refused")
	diag := &fakeDiagnostics{err: underlying}
	session := newDiagnosticsSession(t, diag)
	ctx := context.Background()

	tools := []string{
		"uptime", "disk_usage", "memory_usage",
		"failed_services", "cpu_usage", "reboot_required",
		"running_processes", "journal_errors", "kernel_version",
	}
	for _, name := range tools {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      name,
				Arguments: map[string]any{"server": "archive"},
			})
			if err != nil {
				t.Fatalf("CallTool transport error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected IsError=true for a failing diagnostics call")
			}
			if len(result.Content) == 0 {
				t.Fatal("expected error content")
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
			if envelope.Message == "" {
				t.Error("expected non-empty message")
			}
		})
	}
}

func TestFailedServices_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDiagnostics{failedServices: []linux.FailedService{
		{Unit: "nginx.service", Load: "loaded", Active: "failed", Sub: "failed", Description: "A web server"},
	}}
	session := newDiagnosticsSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "failed_services",
		Arguments: map[string]any{"server": "archive"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out FailedServicesOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Services) != 1 || out.Status != "warning" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestFailedServices_NoneIsOK(t *testing.T) {
	diag := &fakeDiagnostics{}
	session := newDiagnosticsSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "failed_services",
		Arguments: map[string]any{"server": "archive"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var out FailedServicesOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "ok" {
		t.Errorf("Status = %q, want ok", out.Status)
	}
}

func TestCPUUsage_SeverityLevels(t *testing.T) {
	tests := []struct {
		name       string
		pct        float64
		wantStatus string
	}{
		{"low usage is ok", 40, "ok"},
		{"80 percent warns", 80, "warning"},
		{"95 percent is critical", 95, "critical"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := &fakeDiagnostics{cpu: linux.CPUUsage{UsedPercent: tt.pct}}
			session := newDiagnosticsSession(t, diag)
			ctx := context.Background()

			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "cpu_usage",
				Arguments: map[string]any{"server": "archive"},
			})
			if err != nil || result.IsError {
				t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
			}
			raw, _ := json.Marshal(result.StructuredContent)
			var out CPUUsageOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if out.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", out.Status, tt.wantStatus)
			}
		})
	}
}

func TestRebootRequired_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDiagnostics{reboot: linux.RebootRequired{
		Required: true, Reason: "kernel mismatch", RunningKernel: "6.8.0-1", NewestKernel: "6.8.0-2",
	}}
	session := newDiagnosticsSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "reboot_required",
		Arguments: map[string]any{"server": "archive"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var out RebootRequiredOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Required || out.Status != "warning" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestRunningProcesses_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDiagnostics{processes: []linux.ProcessInfo{
		{PID: 1, PPID: 0, User: "root", CPUPercent: 1.5, MemPercent: 0.5, Command: "init"},
	}}
	session := newDiagnosticsSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "running_processes",
		Arguments: map[string]any{"server": "archive", "limit": 5},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var out RunningProcessesOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Processes) != 1 || out.Processes[0].Command != "init" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestJournalErrors_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDiagnostics{journalEntries: []linux.JournalEntry{
		{Timestamp: time.Unix(1700000000, 0).UTC(), Unit: "nginx.service", Priority: "3", Message: "boom"},
	}}
	session := newDiagnosticsSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "journal_errors",
		Arguments: map[string]any{"server": "archive"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var out JournalErrorsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Entries) != 1 || out.Entries[0].Message != "boom" || out.Status != "warning" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestKernelVersion_ViaMCPProtocol(t *testing.T) {
	diag := &fakeDiagnostics{kernelVersion: "6.8.0-1-generic"}
	session := newDiagnosticsSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "kernel_version",
		Arguments: map[string]any{"server": "archive"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var out KernelVersionOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.KernelRelease != "6.8.0-1-generic" {
		t.Errorf("KernelRelease = %q, want %q", out.KernelRelease, "6.8.0-1-generic")
	}
}
