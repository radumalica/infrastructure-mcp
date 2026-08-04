package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/proxmox"
)

type fakeProxmoxDiagnostics struct {
	nodes    []proxmox.NodeEntry
	vms      []proxmox.VMEntry
	tasks    []proxmox.TaskEntry
	err      error
	sawNode  string
	sawVMID  int
	sawType  string
	sawSnap  string
	startCnt int
	stopCnt  int
}

func (f *fakeProxmoxDiagnostics) ListNodes(_ context.Context, _ string) ([]proxmox.NodeEntry, error) {
	return f.nodes, f.err
}
func (f *fakeProxmoxDiagnostics) ListVMs(_ context.Context, _, node string) ([]proxmox.VMEntry, error) {
	f.sawNode = node
	return f.vms, f.err
}
func (f *fakeProxmoxDiagnostics) ListTasks(_ context.Context, _, node string, _ int) ([]proxmox.TaskEntry, error) {
	f.sawNode = node
	return f.tasks, f.err
}
func (f *fakeProxmoxDiagnostics) StartVM(_ context.Context, _, node, guestType string, vmid int) (string, error) {
	f.sawNode, f.sawType, f.sawVMID = node, guestType, vmid
	f.startCnt++
	return "UPID:pve01:...", f.err
}
func (f *fakeProxmoxDiagnostics) StopVM(_ context.Context, _, node, guestType string, vmid int) (string, error) {
	f.sawNode, f.sawType, f.sawVMID = node, guestType, vmid
	f.stopCnt++
	return "UPID:pve01:...", f.err
}
func (f *fakeProxmoxDiagnostics) Snapshot(_ context.Context, _, node, guestType string, vmid int, snapname string) (string, error) {
	f.sawNode, f.sawType, f.sawVMID, f.sawSnap = node, guestType, vmid, snapname
	return "UPID:pve01:...", f.err
}

func newProxmoxSession(t *testing.T, diag *fakeProxmoxDiagnostics) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterProxmoxNodes(server, testLogger(), diag)
	RegisterProxmoxVMs(server, testLogger(), diag)
	RegisterProxmoxTasks(server, testLogger(), diag)
	RegisterProxmoxStartVM(server, testLogger(), diag)
	RegisterProxmoxStopVM(server, testLogger(), diag)
	RegisterProxmoxSnapshot(server, testLogger(), diag)

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

func TestProxmoxNodes_ViaMCPProtocol(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{nodes: []proxmox.NodeEntry{
		{Node: "pve01", Status: "online", MaxCPU: 8},
	}}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_nodes",
		Arguments: map[string]any{"instance": "lab"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ProxmoxNodesOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Nodes) != 1 || out.Nodes[0].Node != "pve01" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestProxmoxVMs_ViaMCPProtocol(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{vms: []proxmox.VMEntry{
		{VMID: 100, Name: "web01", Type: "qemu", Status: "running"},
	}}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_vms",
		Arguments: map[string]any{"instance": "lab", "node": "pve01"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawNode != "pve01" {
		t.Errorf("sawNode = %q, want pve01", diag.sawNode)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ProxmoxVMsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.VMs) != 1 || out.VMs[0].VMID != 100 {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestProxmoxTasks_ViaMCPProtocol(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{tasks: []proxmox.TaskEntry{
		{UPID: "UPID:pve01:...", Status: "OK"},
	}}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_tasks",
		Arguments: map[string]any{"instance": "lab", "node": "pve01"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ProxmoxTasksOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Tasks) != 1 || out.Tasks[0].Status != "OK" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestProxmoxStartVM_RequiresConfirm(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_start_vm",
		Arguments: map[string]any{"instance": "lab", "node": "pve01", "vmid": 100},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.startCnt != 0 {
		t.Errorf("expected StartVM not to be called without confirm")
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ProxmoxStartVMOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "confirmation_required" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestProxmoxStartVM_DryRun(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_start_vm",
		Arguments: map[string]any{"instance": "lab", "node": "pve01", "vmid": 100, "confirm": true, "dry_run": true},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.startCnt != 0 {
		t.Error("expected StartVM not to be called on a dry run, even with confirm: true")
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ProxmoxStartVMOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "dry_run" || !strings.Contains(out.Message, "/nodes/pve01/qemu/100/status/start") {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestProxmoxStopVM_DryRun(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_stop_vm",
		Arguments: map[string]any{"instance": "lab", "node": "pve01", "vmid": 100, "dry_run": true},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.stopCnt != 0 {
		t.Error("expected StopVM not to be called on a dry run")
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ProxmoxStopVMOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "dry_run" || !strings.Contains(out.Message, "/nodes/pve01/qemu/100/status/stop") {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestProxmoxSnapshot_DryRun(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_snapshot",
		Arguments: map[string]any{"instance": "lab", "node": "pve01", "vmid": 100, "type": "lxc", "name": "pre-upgrade", "dry_run": true},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawSnap != "" {
		t.Error("expected Snapshot not to be called on a dry run")
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ProxmoxSnapshotOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "dry_run" || !strings.Contains(out.Message, "/nodes/pve01/lxc/100/snapshot") || !strings.Contains(out.Message, "pre-upgrade") {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestProxmoxStopVM_Confirmed(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_stop_vm",
		Arguments: map[string]any{"instance": "lab", "node": "pve01", "vmid": 100, "confirm": true},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.stopCnt != 1 || diag.sawVMID != 100 || diag.sawType != "qemu" {
		t.Errorf("unexpected StopVM call: cnt=%d vmid=%d type=%s", diag.stopCnt, diag.sawVMID, diag.sawType)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ProxmoxStopVMOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "stop_requested" || out.UPID != "UPID:pve01:..." {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestProxmoxSnapshot_Confirmed(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_snapshot",
		Arguments: map[string]any{"instance": "lab", "node": "pve01", "vmid": 100, "type": "lxc", "name": "pre-upgrade", "confirm": true},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawType != "lxc" || diag.sawSnap != "pre-upgrade" {
		t.Errorf("unexpected Snapshot call: type=%s snap=%s", diag.sawType, diag.sawSnap)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out ProxmoxSnapshotOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "snapshot_requested" || out.UPID != "UPID:pve01:..." {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestProxmoxTools_ErrorPath(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{err: errors.New("boom")}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	cases := []struct {
		tool string
		args map[string]any
	}{
		{"proxmox_nodes", map[string]any{"instance": "lab"}},
		{"proxmox_vms", map[string]any{"instance": "lab", "node": "pve01"}},
		{"proxmox_tasks", map[string]any{"instance": "lab", "node": "pve01"}},
		{"proxmox_start_vm", map[string]any{"instance": "lab", "node": "pve01", "vmid": 100, "confirm": true}},
		{"proxmox_stop_vm", map[string]any{"instance": "lab", "node": "pve01", "vmid": 100, "confirm": true}},
		{"proxmox_snapshot", map[string]any{"instance": "lab", "node": "pve01", "vmid": 100, "name": "x", "confirm": true}},
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

func TestProxmoxNodes_InstanceNotFound(t *testing.T) {
	diag := &fakeProxmoxDiagnostics{err: inventory.ErrNotFound}
	session := newProxmoxSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "proxmox_nodes",
		Arguments: map[string]any{"instance": "does-not-exist"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for a missing instance")
	}
}
