package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CiscoLogsInput targets a single inventory router/switch.
type CiscoLogsInput struct {
	Device string `json:"device" jsonschema:"the inventory router/switch name"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of log lines to return — the most recent N messages, kept in their original oldest-to-newest order (default: all buffered lines)"`
}

// TargetServer implements Targeted.
func (in CiscoLogsInput) TargetServer() string { return in.Device }

// CiscoLogsOutput is the result of cisco_logs.
type CiscoLogsOutput struct {
	Device    string   `json:"device"`
	Lines     []string `json:"lines"`
	Timestamp string   `json:"timestamp"`
}

// RegisterCiscoLogs adds the cisco_logs tool to server.
func RegisterCiscoLogs(server *mcp.Server, logger *slog.Logger, diag CiscoDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cisco_logs",
		Description: "Fetch a Cisco device's buffered syslog messages (\"show logging\"), optionally capped to the most recent `limit` lines.",
	}, withLogging(logger, "cisco_logs", func(ctx context.Context, req *mcp.CallToolRequest, in CiscoLogsInput) (*mcp.CallToolResult, CiscoLogsOutput, error) {
		lines, err := diag.Logs(ctx, in.Device, in.Limit)
		if err != nil {
			return nil, CiscoLogsOutput{}, wrapErr(err)
		}

		return nil, CiscoLogsOutput{
			Device:    in.Device,
			Lines:     lines,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
