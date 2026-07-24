package tools

import (
	"context"

	"infrastructure-mcp/internal/docker"
)

// DockerDiagnostics is satisfied by *docker.Client.
type DockerDiagnostics interface {
	Ps(ctx context.Context, server string, all bool) ([]docker.Container, error)
	Images(ctx context.Context, server string) ([]docker.Image, error)
	Stats(ctx context.Context, server, container string) ([]docker.ContainerStats, error)
	Logs(ctx context.Context, server, container string, tail int) (string, error)
	Restart(ctx context.Context, server, container string) error
}
