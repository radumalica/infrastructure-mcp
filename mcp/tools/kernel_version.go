package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// KernelVersionInput targets a single inventory server.
type KernelVersionInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
}

// TargetServer implements Targeted.
func (in KernelVersionInput) TargetServer() string { return in.Server }

// KernelVersionOutput is the result of kernel_version.
type KernelVersionOutput struct {
	Server        string `json:"server"`
	KernelRelease string `json:"kernel_release"`
	Timestamp     string `json:"timestamp"`
}

// RegisterKernelVersion adds the kernel_version tool to server.
func RegisterKernelVersion(server *mcp.Server, logger *slog.Logger, diag LinuxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "kernel_version",
		Description: "Report the running kernel release (`uname -r`) of a Linux server.",
	}, withLogging(logger, "kernel_version", func(ctx context.Context, req *mcp.CallToolRequest, in KernelVersionInput) (*mcp.CallToolResult, KernelVersionOutput, error) {
		release, err := diag.KernelVersion(ctx, in.Server)
		if err != nil {
			return nil, KernelVersionOutput{}, wrapErr(err)
		}
		return nil, KernelVersionOutput{
			Server:        in.Server,
			KernelRelease: release,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
