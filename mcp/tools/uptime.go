package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/linux"
)

// UptimeInput targets a single inventory server.
type UptimeInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
}

// TargetServer implements Targeted.
func (in UptimeInput) TargetServer() string { return in.Server }

// UptimeOutput reports how long a server has been up and its load.
type UptimeOutput struct {
	Server        string  `json:"server"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	Load1         float64 `json:"load1"`
	Load5         float64 `json:"load5"`
	Load15        float64 `json:"load15"`
	Timestamp     string  `json:"timestamp"`
}

// LinuxDiagnostics is satisfied by *linux.Client.
type LinuxDiagnostics interface {
	Uptime(ctx context.Context, server string) (linux.UptimeInfo, error)
	DiskUsage(ctx context.Context, server string) ([]linux.DiskUsage, error)
	MemoryUsage(ctx context.Context, server string) (linux.MemoryUsage, error)
	FailedServices(ctx context.Context, server string) ([]linux.FailedService, error)
	CPUUsage(ctx context.Context, server string) (linux.CPUUsage, error)
	RebootRequired(ctx context.Context, server string) (linux.RebootRequired, error)
	RunningProcesses(ctx context.Context, server string, limit int) ([]linux.ProcessInfo, error)
	JournalErrors(ctx context.Context, server string, limit int) ([]linux.JournalEntry, error)
	KernelVersion(ctx context.Context, server string) (string, error)
}

// RegisterUptime adds the uptime tool to server.
func RegisterUptime(server *mcp.Server, logger *slog.Logger, diag LinuxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "uptime",
		Description: "Report how long a Linux server has been running and its 1/5/15-minute load averages.",
	}, withLogging(logger, "uptime", func(ctx context.Context, req *mcp.CallToolRequest, in UptimeInput) (*mcp.CallToolResult, UptimeOutput, error) {
		info, err := diag.Uptime(ctx, in.Server)
		if err != nil {
			return nil, UptimeOutput{}, wrapErr(err)
		}
		return nil, UptimeOutput{
			Server:        in.Server,
			UptimeSeconds: info.Uptime.Seconds(),
			Load1:         info.Load1,
			Load5:         info.Load5,
			Load15:        info.Load15,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
