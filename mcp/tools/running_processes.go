package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunningProcessesInput targets a single inventory server.
type RunningProcessesInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of processes to return, sorted by CPU usage descending (default 20)"`
}

// TargetServer implements Targeted.
func (in RunningProcessesInput) TargetServer() string { return in.Server }

// ProcessEntry describes one running process.
type ProcessEntry struct {
	PID        int     `json:"pid"`
	PPID       int     `json:"ppid"`
	User       string  `json:"user"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	Command    string  `json:"command"`
}

// RunningProcessesOutput is the result of running_processes.
type RunningProcessesOutput struct {
	Server    string         `json:"server"`
	Processes []ProcessEntry `json:"processes"`
	Timestamp string         `json:"timestamp"`
}

// RegisterRunningProcesses adds the running_processes tool to server.
func RegisterRunningProcesses(server *mcp.Server, logger *slog.Logger, diag LinuxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "running_processes",
		Description: "List the top processes on a Linux server, sorted by CPU usage descending.",
	}, withLogging(logger, "running_processes", func(ctx context.Context, req *mcp.CallToolRequest, in RunningProcessesInput) (*mcp.CallToolResult, RunningProcessesOutput, error) {
		procs, err := diag.RunningProcesses(ctx, in.Server, in.Limit)
		if err != nil {
			return nil, RunningProcessesOutput{}, wrapErr(err)
		}

		entries := make([]ProcessEntry, 0, len(procs))
		for _, p := range procs {
			entries = append(entries, ProcessEntry{
				PID:        p.PID,
				PPID:       p.PPID,
				User:       p.User,
				CPUPercent: p.CPUPercent,
				MemPercent: p.MemPercent,
				Command:    p.Command,
			})
		}

		return nil, RunningProcessesOutput{
			Server:    in.Server,
			Processes: entries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
