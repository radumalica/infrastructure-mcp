package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DockerPsInput targets a single inventory server.
type DockerPsInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
	All    bool   `json:"all,omitempty" jsonschema:"include stopped containers too (default: running containers only)"`
}

// TargetServer implements Targeted.
func (in DockerPsInput) TargetServer() string { return in.Server }

// ContainerEntry describes one container.
type ContainerEntry struct {
	ID         string `json:"id"`
	Image      string `json:"image"`
	Command    string `json:"command"`
	CreatedAt  string `json:"created_at"`
	Status     string `json:"status"`
	State      string `json:"state"`
	Ports      string `json:"ports,omitempty"`
	Names      string `json:"names"`
	RunningFor string `json:"running_for,omitempty"`
}

// DockerPsOutput is the result of docker_ps.
type DockerPsOutput struct {
	Server     string           `json:"server"`
	Containers []ContainerEntry `json:"containers"`
	Timestamp  string           `json:"timestamp"`
}

// RegisterDockerPs adds the docker_ps tool to server.
func RegisterDockerPs(server *mcp.Server, logger *slog.Logger, diag DockerDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_ps",
		Description: "List containers on a Docker host (running only, unless `all` is set).",
	}, withLogging(logger, "docker_ps", func(ctx context.Context, req *mcp.CallToolRequest, in DockerPsInput) (*mcp.CallToolResult, DockerPsOutput, error) {
		containers, err := diag.Ps(ctx, in.Server, in.All)
		if err != nil {
			return nil, DockerPsOutput{}, wrapErr(err)
		}

		entries := make([]ContainerEntry, 0, len(containers))
		for _, cnt := range containers {
			entries = append(entries, ContainerEntry{
				ID:         cnt.ID,
				Image:      cnt.Image,
				Command:    cnt.Command,
				CreatedAt:  cnt.CreatedAt,
				Status:     cnt.Status,
				State:      cnt.State,
				Ports:      cnt.Ports,
				Names:      cnt.Names,
				RunningFor: cnt.RunningFor,
			})
		}

		return nil, DockerPsOutput{
			Server:     in.Server,
			Containers: entries,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
