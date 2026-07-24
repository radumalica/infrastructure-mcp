package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DockerStatsInput targets a single inventory server, optionally scoped
// to one container.
type DockerStatsInput struct {
	Server    string `json:"server" jsonschema:"the inventory server name to check"`
	Container string `json:"container,omitempty" jsonschema:"restrict to a single container name or ID (default: all containers)"`
}

// TargetServer implements Targeted.
func (in DockerStatsInput) TargetServer() string { return in.Server }

// ContainerStatsEntry describes one point-in-time resource usage sample.
type ContainerStatsEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CPUPercent string `json:"cpu_percent"`
	MemUsage   string `json:"mem_usage"`
	MemPercent string `json:"mem_percent"`
	NetIO      string `json:"net_io"`
	BlockIO    string `json:"block_io"`
	PIDs       string `json:"pids"`
}

// DockerStatsOutput is the result of docker_stats.
type DockerStatsOutput struct {
	Server    string                `json:"server"`
	Stats     []ContainerStatsEntry `json:"stats"`
	Timestamp string                `json:"timestamp"`
}

// RegisterDockerStats adds the docker_stats tool to server.
func RegisterDockerStats(server *mcp.Server, logger *slog.Logger, diag DockerDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_stats",
		Description: "Report a point-in-time CPU/memory/network/disk usage snapshot for containers on a Docker host.",
	}, withLogging(logger, "docker_stats", func(ctx context.Context, req *mcp.CallToolRequest, in DockerStatsInput) (*mcp.CallToolResult, DockerStatsOutput, error) {
		stats, err := diag.Stats(ctx, in.Server, in.Container)
		if err != nil {
			return nil, DockerStatsOutput{}, wrapErr(err)
		}

		entries := make([]ContainerStatsEntry, 0, len(stats))
		for _, s := range stats {
			entries = append(entries, ContainerStatsEntry{
				ID:         s.ID,
				Name:       s.Name,
				CPUPercent: s.CPUPercent,
				MemUsage:   s.MemUsage,
				MemPercent: s.MemPercent,
				NetIO:      s.NetIO,
				BlockIO:    s.BlockIO,
				PIDs:       s.PIDs,
			})
		}

		return nil, DockerStatsOutput{
			Server:    in.Server,
			Stats:     entries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
