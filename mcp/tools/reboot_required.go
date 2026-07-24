package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RebootRequiredInput targets a single inventory server.
type RebootRequiredInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
}

// TargetServer implements Targeted.
func (in RebootRequiredInput) TargetServer() string { return in.Server }

// RebootRequiredOutput is the result of reboot_required.
type RebootRequiredOutput struct {
	Server         string `json:"server"`
	Required       bool   `json:"required"`
	Reason         string `json:"reason,omitempty"`
	RunningKernel  string `json:"running_kernel"`
	NewestKernel   string `json:"newest_kernel,omitempty"`
	Status         string `json:"status"`
	Recommendation string `json:"recommendation,omitempty"`
	Timestamp      string `json:"timestamp"`
}

// RegisterRebootRequired adds the reboot_required tool to server.
func RegisterRebootRequired(server *mcp.Server, logger *slog.Logger, diag LinuxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "reboot_required",
		Description: "Report whether a Linux server is pending a reboot (Debian/Ubuntu reboot-required marker, or a running kernel older than the newest installed one).",
	}, withLogging(logger, "reboot_required", func(ctx context.Context, req *mcp.CallToolRequest, in RebootRequiredInput) (*mcp.CallToolResult, RebootRequiredOutput, error) {
		info, err := diag.RebootRequired(ctx, in.Server)
		if err != nil {
			return nil, RebootRequiredOutput{}, wrapErr(err)
		}

		status := "ok"
		recommendation := ""
		if info.Required {
			status = "warning"
			recommendation = "Schedule a reboot during a maintenance window."
		}

		return nil, RebootRequiredOutput{
			Server:         in.Server,
			Required:       info.Required,
			Reason:         info.Reason,
			RunningKernel:  info.RunningKernel,
			NewestKernel:   info.NewestKernel,
			Status:         status,
			Recommendation: recommendation,
			Timestamp:      time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
