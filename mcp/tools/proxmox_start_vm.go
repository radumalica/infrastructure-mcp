package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/audit"
)

// ProxmoxStartVMInput targets a single guest on a single Proxmox node.
// Confirm must be explicitly set to true for the start to actually
// execute, per README's "dangerous actions require confirmation" rule.
type ProxmoxStartVMInput struct {
	Instance string `json:"instance" jsonschema:"the inventory proxmox instance name"`
	Node     string `json:"node" jsonschema:"the cluster node the guest lives on"`
	VMID     int    `json:"vmid" jsonschema:"the guest's numeric VM/container ID"`
	Type     string `json:"type,omitempty" jsonschema:"guest type: 'qemu' (default) or 'lxc'"`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"must be true to actually start the guest; otherwise the tool reports what it would do without acting"`
	DryRun   bool   `json:"dry_run,omitempty" jsonschema:"if true, report the exact API call that would be made and return without acting or requiring confirm"`
}

// TargetServer implements Targeted.
func (in ProxmoxStartVMInput) TargetServer() string { return in.Instance }

// guestType returns in.Type, defaulting to "qemu" when unset.
func (in ProxmoxStartVMInput) guestType() string {
	if in.Type == "" {
		return "qemu"
	}
	return in.Type
}

// ProxmoxStartVMOutput is the result of proxmox_start_vm. Proxmox guest
// actions are asynchronous: a "start_requested" status means the task was
// queued, not that the guest has finished starting — pass UPID to
// proxmox_tasks to check the outcome.
type ProxmoxStartVMOutput struct {
	Instance  string `json:"instance"`
	Node      string `json:"node"`
	VMID      int    `json:"vmid"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	UPID      string `json:"upid,omitempty"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// auditStatus implements auditable.
func (o ProxmoxStartVMOutput) auditStatus() string { return o.Status }

// RegisterProxmoxStartVM adds the proxmox_start_vm tool to server.
func RegisterProxmoxStartVM(server *mcp.Server, logger *slog.Logger, diag ProxmoxDiagnostics, auditLog *audit.Log) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "proxmox_start_vm",
		Description: "Start a QEMU VM or LXC container on a Proxmox node. This is a state-changing action: it requires `confirm: true`, and returns without acting if confirmation is missing. Starting is asynchronous — the response carries a task UPID, not confirmation the guest has finished starting; check it with proxmox_tasks. Set `dry_run: true` to see the exact API call that would be made without needing confirm at all.",
	}, withAudit(auditLog, "proxmox_start_vm", withLogging(logger, "proxmox_start_vm", func(ctx context.Context, req *mcp.CallToolRequest, in ProxmoxStartVMInput) (*mcp.CallToolResult, ProxmoxStartVMOutput, error) {
		guestType := in.guestType()

		if in.DryRun {
			return nil, ProxmoxStartVMOutput{
				Instance:  in.Instance,
				Node:      in.Node,
				VMID:      in.VMID,
				Type:      guestType,
				Status:    "dry_run",
				Message:   fmt.Sprintf("Would call: POST /nodes/%s/%s/%d/status/start (instance %q). No action was taken.", in.Node, guestType, in.VMID, in.Instance),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}, nil
		}

		if !in.Confirm {
			return nil, ProxmoxStartVMOutput{
				Instance:  in.Instance,
				Node:      in.Node,
				VMID:      in.VMID,
				Type:      guestType,
				Status:    "confirmation_required",
				Message:   "Set confirm: true to start this guest. No action was taken.",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}, nil
		}

		upid, err := diag.StartVM(ctx, in.Instance, in.Node, guestType, in.VMID)
		if err != nil {
			return nil, ProxmoxStartVMOutput{}, wrapErr(err)
		}

		return nil, ProxmoxStartVMOutput{
			Instance:  in.Instance,
			Node:      in.Node,
			VMID:      in.VMID,
			Type:      guestType,
			Status:    "start_requested",
			UPID:      upid,
			Message:   "Start requested; check proxmox_tasks with this UPID for the outcome.",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	})))
}
