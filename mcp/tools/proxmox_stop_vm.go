package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProxmoxStopVMInput targets a single guest on a single Proxmox node.
// Confirm must be explicitly set to true for the stop to actually
// execute, per README's "dangerous actions require confirmation" rule.
type ProxmoxStopVMInput struct {
	Instance string `json:"instance" jsonschema:"the inventory proxmox instance name"`
	Node     string `json:"node" jsonschema:"the cluster node the guest lives on"`
	VMID     int    `json:"vmid" jsonschema:"the guest's numeric VM/container ID"`
	Type     string `json:"type,omitempty" jsonschema:"guest type: 'qemu' (default) or 'lxc'"`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"must be true to actually stop the guest; otherwise the tool reports what it would do without acting"`
}

// TargetServer implements Targeted.
func (in ProxmoxStopVMInput) TargetServer() string { return in.Instance }

// guestType returns in.Type, defaulting to "qemu" when unset.
func (in ProxmoxStopVMInput) guestType() string {
	if in.Type == "" {
		return "qemu"
	}
	return in.Type
}

// ProxmoxStopVMOutput is the result of proxmox_stop_vm. Proxmox guest
// actions are asynchronous: a "stop_requested" status means the task was
// queued, not that the guest has finished stopping — pass UPID to
// proxmox_tasks to check the outcome.
type ProxmoxStopVMOutput struct {
	Instance  string `json:"instance"`
	Node      string `json:"node"`
	VMID      int    `json:"vmid"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	UPID      string `json:"upid,omitempty"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// RegisterProxmoxStopVM adds the proxmox_stop_vm tool to server.
func RegisterProxmoxStopVM(server *mcp.Server, logger *slog.Logger, diag ProxmoxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "proxmox_stop_vm",
		Description: "Force-stop (hard power-off, not a graceful shutdown) a QEMU VM or LXC container on a Proxmox node. This is a dangerous, service-impacting action: it requires `confirm: true`, and returns without acting if confirmation is missing. Stopping is asynchronous — the response carries a task UPID, not confirmation the guest has finished stopping; check it with proxmox_tasks.",
	}, withLogging(logger, "proxmox_stop_vm", func(ctx context.Context, req *mcp.CallToolRequest, in ProxmoxStopVMInput) (*mcp.CallToolResult, ProxmoxStopVMOutput, error) {
		guestType := in.guestType()

		if !in.Confirm {
			return nil, ProxmoxStopVMOutput{
				Instance:  in.Instance,
				Node:      in.Node,
				VMID:      in.VMID,
				Type:      guestType,
				Status:    "confirmation_required",
				Message:   "Set confirm: true to stop this guest. No action was taken.",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}, nil
		}

		upid, err := diag.StopVM(ctx, in.Instance, in.Node, guestType, in.VMID)
		if err != nil {
			return nil, ProxmoxStopVMOutput{}, wrapErr(err)
		}

		return nil, ProxmoxStopVMOutput{
			Instance:  in.Instance,
			Node:      in.Node,
			VMID:      in.VMID,
			Type:      guestType,
			Status:    "stop_requested",
			UPID:      upid,
			Message:   "Stop requested; check proxmox_tasks with this UPID for the outcome.",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
