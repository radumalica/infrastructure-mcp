package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProxmoxTasksInput targets a single node within a Proxmox cluster.
type ProxmoxTasksInput struct {
	Instance string `json:"instance" jsonschema:"the inventory proxmox instance name"`
	Node     string `json:"node" jsonschema:"the cluster node to list tasks on, e.g. 'pve01'"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum number of tasks to return, most recent first (default: Proxmox's server-side default)"`
}

// TargetServer implements Targeted.
func (in ProxmoxTasksInput) TargetServer() string { return in.Instance }

// TaskSummary describes one recorded Proxmox task.
type TaskSummary struct {
	UPID      string `json:"upid"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	User      string `json:"user"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time,omitempty"`
}

// ProxmoxTasksOutput is the result of proxmox_tasks.
type ProxmoxTasksOutput struct {
	Instance  string        `json:"instance"`
	Node      string        `json:"node"`
	Tasks     []TaskSummary `json:"tasks"`
	Timestamp string        `json:"timestamp"`
}

// RegisterProxmoxTasks adds the proxmox_tasks tool to server.
func RegisterProxmoxTasks(server *mcp.Server, logger *slog.Logger, diag ProxmoxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "proxmox_tasks",
		Description: "List recent tasks (backups, migrations, VM actions, ...) recorded on a Proxmox node, most recent first.",
	}, withLogging(logger, "proxmox_tasks", func(ctx context.Context, req *mcp.CallToolRequest, in ProxmoxTasksInput) (*mcp.CallToolResult, ProxmoxTasksOutput, error) {
		tasks, err := diag.ListTasks(ctx, in.Instance, in.Node, in.Limit)
		if err != nil {
			return nil, ProxmoxTasksOutput{}, wrapErr(err)
		}

		summaries := make([]TaskSummary, 0, len(tasks))
		for _, t := range tasks {
			summaries = append(summaries, TaskSummary{
				UPID:      t.UPID,
				Type:      t.Type,
				Status:    t.Status,
				User:      t.User,
				StartTime: t.StartTime,
				EndTime:   t.EndTime,
			})
		}

		return nil, ProxmoxTasksOutput{
			Instance:  in.Instance,
			Node:      in.Node,
			Tasks:     summaries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
