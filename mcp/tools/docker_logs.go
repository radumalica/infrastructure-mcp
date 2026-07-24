package tools

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DockerLogsInput targets a single container on a single server.
type DockerLogsInput struct {
	Server    string `json:"server" jsonschema:"the inventory server name to check"`
	Container string `json:"container" jsonschema:"container name or ID to fetch logs for"`
	Tail      int    `json:"tail,omitempty" jsonschema:"number of most recent log lines to return (default 100)"`
}

// TargetServer implements Targeted.
func (in DockerLogsInput) TargetServer() string { return in.Server }

// DockerLogsOutput is the result of docker_logs.
type DockerLogsOutput struct {
	Server    string `json:"server"`
	Container string `json:"container"`
	Logs      string `json:"logs"`
	LineCount int    `json:"line_count"`
	Timestamp string `json:"timestamp"`
}

// RegisterDockerLogs adds the docker_logs tool to server.
func RegisterDockerLogs(server *mcp.Server, logger *slog.Logger, diag DockerDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_logs",
		Description: "Fetch the most recent combined stdout/stderr log lines from a container on a Docker host.",
	}, withLogging(logger, "docker_logs", func(ctx context.Context, req *mcp.CallToolRequest, in DockerLogsInput) (*mcp.CallToolResult, DockerLogsOutput, error) {
		logs, err := diag.Logs(ctx, in.Server, in.Container, in.Tail)
		if err != nil {
			return nil, DockerLogsOutput{}, wrapErr(err)
		}

		lineCount := 0
		if trimmed := strings.TrimRight(logs, "\n"); trimmed != "" {
			lineCount = strings.Count(trimmed, "\n") + 1
		}

		return nil, DockerLogsOutput{
			Server:    in.Server,
			Container: in.Container,
			Logs:      logs,
			LineCount: lineCount,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
