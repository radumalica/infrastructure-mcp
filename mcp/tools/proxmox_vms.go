package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProxmoxVMsInput targets a single node within a Proxmox cluster.
type ProxmoxVMsInput struct {
	Instance string `json:"instance" jsonschema:"the inventory proxmox instance name"`
	Node     string `json:"node" jsonschema:"the cluster node to list guests on, e.g. 'pve01'"`
}

// TargetServer implements Targeted.
func (in ProxmoxVMsInput) TargetServer() string { return in.Instance }

// VMSummary describes one QEMU VM or LXC container.
type VMSummary struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	MaxMem int64   `json:"max_mem"`
	Mem    int64   `json:"mem"`
	Uptime int64   `json:"uptime"`
}

// ProxmoxVMsOutput is the result of proxmox_vms.
type ProxmoxVMsOutput struct {
	Instance  string      `json:"instance"`
	Node      string      `json:"node"`
	VMs       []VMSummary `json:"vms"`
	Timestamp string      `json:"timestamp"`
}

// RegisterProxmoxVMs adds the proxmox_vms tool to server.
func RegisterProxmoxVMs(server *mcp.Server, logger *slog.Logger, diag ProxmoxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "proxmox_vms",
		Description: "List every QEMU VM and LXC container on a Proxmox node.",
	}, withLogging(logger, "proxmox_vms", func(ctx context.Context, req *mcp.CallToolRequest, in ProxmoxVMsInput) (*mcp.CallToolResult, ProxmoxVMsOutput, error) {
		vms, err := diag.ListVMs(ctx, in.Instance, in.Node)
		if err != nil {
			return nil, ProxmoxVMsOutput{}, wrapErr(err)
		}

		summaries := make([]VMSummary, 0, len(vms))
		for _, v := range vms {
			summaries = append(summaries, VMSummary{
				VMID:   v.VMID,
				Name:   v.Name,
				Type:   v.Type,
				Status: v.Status,
				CPU:    v.CPU,
				MaxMem: v.MaxMem,
				Mem:    v.Mem,
				Uptime: v.Uptime,
			})
		}

		return nil, ProxmoxVMsOutput{
			Instance:  in.Instance,
			Node:      in.Node,
			VMs:       summaries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
