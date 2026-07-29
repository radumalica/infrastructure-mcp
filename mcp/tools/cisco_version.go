package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CiscoVersionInput targets a single inventory router/switch.
type CiscoVersionInput struct {
	Device string `json:"device" jsonschema:"the inventory router/switch name"`
}

// TargetServer implements Targeted.
func (in CiscoVersionInput) TargetServer() string { return in.Device }

// CiscoVersionOutput is the result of cisco_version.
type CiscoVersionOutput struct {
	Device      string `json:"device"`
	Hostname    string `json:"hostname,omitempty"`
	VersionLine string `json:"version_line"`
	Uptime      string `json:"uptime,omitempty"`
	Timestamp   string `json:"timestamp"`
}

// RegisterCiscoVersion adds the cisco_version tool to server.
func RegisterCiscoVersion(server *mcp.Server, logger *slog.Logger, diag CiscoDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cisco_version",
		Description: "Get a Cisco device's software version, hostname, and uptime (\"show version\").",
	}, withLogging(logger, "cisco_version", func(ctx context.Context, req *mcp.CallToolRequest, in CiscoVersionInput) (*mcp.CallToolResult, CiscoVersionOutput, error) {
		info, err := diag.ShowVersion(ctx, in.Device)
		if err != nil {
			return nil, CiscoVersionOutput{}, wrapErr(err)
		}

		return nil, CiscoVersionOutput{
			Device:      in.Device,
			Hostname:    info.Hostname,
			VersionLine: info.VersionLine,
			Uptime:      info.Uptime,
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
