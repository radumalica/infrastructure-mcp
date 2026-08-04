package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProxmoxSnapshotInput targets a single guest on a single Proxmox node.
// Confirm must be explicitly set to true for the snapshot to actually be
// taken, per README's "dangerous actions require confirmation" rule.
type ProxmoxSnapshotInput struct {
	Instance string `json:"instance" jsonschema:"the inventory proxmox instance name"`
	Node     string `json:"node" jsonschema:"the cluster node the guest lives on"`
	VMID     int    `json:"vmid" jsonschema:"the guest's numeric VM/container ID"`
	Type     string `json:"type,omitempty" jsonschema:"guest type: 'qemu' (default) or 'lxc'"`
	Name     string `json:"name" jsonschema:"name for the new snapshot"`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"must be true to actually take the snapshot; otherwise the tool reports what it would do without acting"`
	DryRun   bool   `json:"dry_run,omitempty" jsonschema:"if true, report the exact API call that would be made and return without acting or requiring confirm"`
}

// TargetServer implements Targeted.
func (in ProxmoxSnapshotInput) TargetServer() string { return in.Instance }

// guestType returns in.Type, defaulting to "qemu" when unset.
func (in ProxmoxSnapshotInput) guestType() string {
	if in.Type == "" {
		return "qemu"
	}
	return in.Type
}

// ProxmoxSnapshotOutput is the result of proxmox_snapshot. Proxmox guest
// actions are asynchronous: a "snapshot_requested" status means the task
// was queued, not that the snapshot has finished — pass UPID to
// proxmox_tasks to check the outcome.
type ProxmoxSnapshotOutput struct {
	Instance  string `json:"instance"`
	Node      string `json:"node"`
	VMID      int    `json:"vmid"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	UPID      string `json:"upid,omitempty"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// RegisterProxmoxSnapshot adds the proxmox_snapshot tool to server.
func RegisterProxmoxSnapshot(server *mcp.Server, logger *slog.Logger, diag ProxmoxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "proxmox_snapshot",
		Description: "Take a snapshot of a QEMU VM or LXC container on a Proxmox node. This is a state-changing action: it requires `confirm: true`, and returns without acting if confirmation is missing. Snapshotting is asynchronous — the response carries a task UPID, not confirmation the snapshot has finished; check it with proxmox_tasks. Set `dry_run: true` to see the exact API call that would be made without needing confirm at all.",
	}, withLogging(logger, "proxmox_snapshot", func(ctx context.Context, req *mcp.CallToolRequest, in ProxmoxSnapshotInput) (*mcp.CallToolResult, ProxmoxSnapshotOutput, error) {
		guestType := in.guestType()

		if in.DryRun {
			return nil, ProxmoxSnapshotOutput{
				Instance:  in.Instance,
				Node:      in.Node,
				VMID:      in.VMID,
				Type:      guestType,
				Name:      in.Name,
				Status:    "dry_run",
				Message:   fmt.Sprintf("Would call: POST /nodes/%s/%s/%d/snapshot with snapname=%s (instance %q). No action was taken.", in.Node, guestType, in.VMID, in.Name, in.Instance),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}, nil
		}

		if !in.Confirm {
			return nil, ProxmoxSnapshotOutput{
				Instance:  in.Instance,
				Node:      in.Node,
				VMID:      in.VMID,
				Type:      guestType,
				Name:      in.Name,
				Status:    "confirmation_required",
				Message:   "Set confirm: true to take this snapshot. No action was taken.",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}, nil
		}

		upid, err := diag.Snapshot(ctx, in.Instance, in.Node, guestType, in.VMID, in.Name)
		if err != nil {
			return nil, ProxmoxSnapshotOutput{}, wrapErr(err)
		}

		return nil, ProxmoxSnapshotOutput{
			Instance:  in.Instance,
			Node:      in.Node,
			VMID:      in.VMID,
			Type:      guestType,
			Name:      in.Name,
			Status:    "snapshot_requested",
			UPID:      upid,
			Message:   "Snapshot requested; check proxmox_tasks with this UPID for the outcome.",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
