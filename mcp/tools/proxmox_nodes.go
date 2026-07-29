package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProxmoxNodesInput targets a single Proxmox cluster.
type ProxmoxNodesInput struct {
	Instance string `json:"instance" jsonschema:"the inventory proxmox instance name"`
}

// TargetServer implements Targeted.
func (in ProxmoxNodesInput) TargetServer() string { return in.Instance }

// ProxmoxNodeSummary describes one cluster node.
type ProxmoxNodeSummary struct {
	Node   string  `json:"node"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	MaxCPU int     `json:"max_cpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"max_mem"`
	Uptime int64   `json:"uptime"`
}

// ProxmoxNodesOutput is the result of proxmox_nodes.
type ProxmoxNodesOutput struct {
	Instance  string               `json:"instance"`
	Nodes     []ProxmoxNodeSummary `json:"nodes"`
	Timestamp string               `json:"timestamp"`
}

// RegisterProxmoxNodes adds the proxmox_nodes tool to server.
func RegisterProxmoxNodes(server *mcp.Server, logger *slog.Logger, diag ProxmoxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "proxmox_nodes",
		Description: "List every node in a Proxmox VE cluster along with its resource usage summary.",
	}, withLogging(logger, "proxmox_nodes", func(ctx context.Context, req *mcp.CallToolRequest, in ProxmoxNodesInput) (*mcp.CallToolResult, ProxmoxNodesOutput, error) {
		nodes, err := diag.ListNodes(ctx, in.Instance)
		if err != nil {
			return nil, ProxmoxNodesOutput{}, wrapErr(err)
		}

		summaries := make([]ProxmoxNodeSummary, 0, len(nodes))
		for _, n := range nodes {
			summaries = append(summaries, ProxmoxNodeSummary{
				Node:   n.Node,
				Status: n.Status,
				CPU:    n.CPU,
				MaxCPU: n.MaxCPU,
				Mem:    n.Mem,
				MaxMem: n.MaxMem,
				Uptime: n.Uptime,
			})
		}

		return nil, ProxmoxNodesOutput{
			Instance:  in.Instance,
			Nodes:     summaries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
