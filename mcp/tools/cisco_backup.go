package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CiscoBackupInput targets a single inventory router/switch.
type CiscoBackupInput struct {
	Device string `json:"device" jsonschema:"the inventory router/switch name"`
}

// TargetServer implements Targeted.
func (in CiscoBackupInput) TargetServer() string { return in.Device }

// CiscoBackupOutput is the result of cisco_backup.
type CiscoBackupOutput struct {
	Device    string `json:"device"`
	Config    string `json:"config"`
	Timestamp string `json:"timestamp"`
}

// RegisterCiscoBackup adds the cisco_backup tool to server.
func RegisterCiscoBackup(server *mcp.Server, logger *slog.Logger, diag CiscoDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cisco_backup",
		Description: "Fetch a Cisco device's running configuration (\"show running-config\"), as-is. Sent as a single command with no session setup: if the device paginates output (has not been configured with \"terminal length 0\"), only the first page is returned.",
	}, withLogging(logger, "cisco_backup", func(ctx context.Context, req *mcp.CallToolRequest, in CiscoBackupInput) (*mcp.CallToolResult, CiscoBackupOutput, error) {
		config, err := diag.Backup(ctx, in.Device)
		if err != nil {
			return nil, CiscoBackupOutput{}, wrapErr(err)
		}

		return nil, CiscoBackupOutput{
			Device:    in.Device,
			Config:    config,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
