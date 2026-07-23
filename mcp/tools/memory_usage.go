package tools

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	memoryWarnPercent = 85.0
	memoryCritPercent = 95.0
)

// MemoryUsageInput targets a single inventory server.
type MemoryUsageInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
}

// MemoryUsageOutput is the result of memory_usage.
type MemoryUsageOutput struct {
	Server         string  `json:"server"`
	TotalKB        int64   `json:"total_kb"`
	UsedKB         int64   `json:"used_kb"`
	AvailableKB    int64   `json:"available_kb"`
	UsedPercent    float64 `json:"used_percent"`
	SwapTotalKB    int64   `json:"swap_total_kb"`
	SwapFreeKB     int64   `json:"swap_free_kb"`
	Status         string  `json:"status"`
	Recommendation string  `json:"recommendation,omitempty"`
	Timestamp      string  `json:"timestamp"`
}

// RegisterMemoryUsage adds the memory_usage tool to server.
func RegisterMemoryUsage(server *mcp.Server, diag LinuxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_usage",
		Description: "Report memory usage on a Linux server, with a warning/critical severity and recommendation when memory is running low.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in MemoryUsageInput) (*mcp.CallToolResult, MemoryUsageOutput, error) {
		mem, err := diag.MemoryUsage(ctx, in.Server)
		if err != nil {
			return nil, MemoryUsageOutput{}, err
		}

		status, recommendation := severityForPercent(mem.UsedPercent, memoryWarnPercent, memoryCritPercent, "memory")

		return nil, MemoryUsageOutput{
			Server:         in.Server,
			TotalKB:        mem.TotalKB,
			UsedKB:         mem.UsedKB,
			AvailableKB:    mem.AvailableKB,
			UsedPercent:    mem.UsedPercent,
			SwapTotalKB:    mem.SwapTotalKB,
			SwapFreeKB:     mem.SwapFreeKB,
			Status:         status,
			Recommendation: recommendation,
			Timestamp:      time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}
