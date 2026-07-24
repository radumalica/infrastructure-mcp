package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	cpuWarnPercent = 80.0
	cpuCritPercent = 95.0
)

// CPUUsageInput targets a single inventory server.
type CPUUsageInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
}

// TargetServer implements Targeted.
func (in CPUUsageInput) TargetServer() string { return in.Server }

// CPUUsageOutput is the result of cpu_usage.
type CPUUsageOutput struct {
	Server         string  `json:"server"`
	UsedPercent    float64 `json:"used_percent"`
	Status         string  `json:"status"`
	Recommendation string  `json:"recommendation,omitempty"`
	Timestamp      string  `json:"timestamp"`
}

// RegisterCPUUsage adds the cpu_usage tool to server.
func RegisterCPUUsage(server *mcp.Server, logger *slog.Logger, diag LinuxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cpu_usage",
		Description: "Report aggregate CPU utilization on a Linux server, sampled over a 1-second window, with a warning/critical severity and recommendation.",
	}, withLogging(logger, "cpu_usage", func(ctx context.Context, req *mcp.CallToolRequest, in CPUUsageInput) (*mcp.CallToolResult, CPUUsageOutput, error) {
		cpu, err := diag.CPUUsage(ctx, in.Server)
		if err != nil {
			return nil, CPUUsageOutput{}, wrapErr(err)
		}

		status, recommendation := severityForPercent(cpu.UsedPercent, cpuWarnPercent, cpuCritPercent, "CPU")

		return nil, CPUUsageOutput{
			Server:         in.Server,
			UsedPercent:    cpu.UsedPercent,
			Status:         status,
			Recommendation: recommendation,
			Timestamp:      time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
