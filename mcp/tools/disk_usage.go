package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	diskWarnPercent = 75.0
	diskCritPercent = 90.0
)

// DiskUsageInput targets a single inventory server.
type DiskUsageInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
}

// TargetServer implements Targeted.
func (in DiskUsageInput) TargetServer() string { return in.Server }

// DiskUsageEntry reports usage and severity for one mounted filesystem.
type DiskUsageEntry struct {
	Filesystem     string `json:"filesystem"`
	MountPoint     string `json:"mount_point"`
	TotalKB        int64  `json:"total_kb"`
	UsedKB         int64  `json:"used_kb"`
	AvailableKB    int64  `json:"available_kb"`
	UsedPercent    int    `json:"used_percent"`
	Status         string `json:"status"`
	Recommendation string `json:"recommendation,omitempty"`
}

// DiskUsageOutput is the result of disk_usage.
type DiskUsageOutput struct {
	Server      string           `json:"server"`
	Filesystems []DiskUsageEntry `json:"filesystems"`
	Timestamp   string           `json:"timestamp"`
}

// RegisterDiskUsage adds the disk_usage tool to server.
func RegisterDiskUsage(server *mcp.Server, logger *slog.Logger, diag LinuxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "disk_usage",
		Description: "Report per-filesystem disk usage on a Linux server, with a warning/critical severity and recommendation for filesystems running low on space.",
	}, withLogging(logger, "disk_usage", func(ctx context.Context, req *mcp.CallToolRequest, in DiskUsageInput) (*mcp.CallToolResult, DiskUsageOutput, error) {
		disks, err := diag.DiskUsage(ctx, in.Server)
		if err != nil {
			return nil, DiskUsageOutput{}, wrapErr(err)
		}

		entries := make([]DiskUsageEntry, 0, len(disks))
		for _, d := range disks {
			status, recommendation := severityForPercent(float64(d.UsedPercent), diskWarnPercent, diskCritPercent, "disk space")
			entries = append(entries, DiskUsageEntry{
				Filesystem:     d.Filesystem,
				MountPoint:     d.MountPoint,
				TotalKB:        d.TotalKB,
				UsedKB:         d.UsedKB,
				AvailableKB:    d.AvailableKB,
				UsedPercent:    d.UsedPercent,
				Status:         status,
				Recommendation: recommendation,
			})
		}

		return nil, DiskUsageOutput{
			Server:      in.Server,
			Filesystems: entries,
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
