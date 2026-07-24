package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DockerImagesInput targets a single inventory server.
type DockerImagesInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
}

// TargetServer implements Targeted.
func (in DockerImagesInput) TargetServer() string { return in.Server }

// ImageEntry describes one local Docker image.
type ImageEntry struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	CreatedAt  string `json:"created_at"`
	Size       string `json:"size"`
}

// DockerImagesOutput is the result of docker_images.
type DockerImagesOutput struct {
	Server    string       `json:"server"`
	Images    []ImageEntry `json:"images"`
	Timestamp string       `json:"timestamp"`
}

// RegisterDockerImages adds the docker_images tool to server.
func RegisterDockerImages(server *mcp.Server, logger *slog.Logger, diag DockerDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_images",
		Description: "List local Docker images on a Docker host.",
	}, withLogging(logger, "docker_images", func(ctx context.Context, req *mcp.CallToolRequest, in DockerImagesInput) (*mcp.CallToolResult, DockerImagesOutput, error) {
		images, err := diag.Images(ctx, in.Server)
		if err != nil {
			return nil, DockerImagesOutput{}, wrapErr(err)
		}

		entries := make([]ImageEntry, 0, len(images))
		for _, img := range images {
			entries = append(entries, ImageEntry{
				ID:         img.ID,
				Repository: img.Repository,
				Tag:        img.Tag,
				CreatedAt:  img.CreatedAt,
				Size:       img.Size,
			})
		}

		return nil, DockerImagesOutput{
			Server:    in.Server,
			Images:    entries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
