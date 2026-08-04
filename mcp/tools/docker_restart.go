package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/audit"
)

// DockerRestartInput targets a single container on a single server.
// Confirm must be explicitly set to true for the restart to actually
// execute, per README's "dangerous actions require confirmation" rule —
// this is the tool surface's first destructive, named action.
type DockerRestartInput struct {
	Server    string `json:"server" jsonschema:"the inventory server name to check"`
	Container string `json:"container" jsonschema:"container name or ID to restart"`
	Confirm   bool   `json:"confirm,omitempty" jsonschema:"must be true to actually restart the container; otherwise the tool reports what it would do without acting"`
	DryRun    bool   `json:"dry_run,omitempty" jsonschema:"if true, report the exact command that would be run and return without acting or requiring confirm"`
}

// TargetServer implements Targeted.
func (in DockerRestartInput) TargetServer() string { return in.Server }

// DockerRestartOutput is the result of docker_restart.
type DockerRestartOutput struct {
	Server    string `json:"server"`
	Container string `json:"container"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// auditStatus implements auditable.
func (o DockerRestartOutput) auditStatus() string { return o.Status }

// RegisterDockerRestart adds the docker_restart tool to server.
func RegisterDockerRestart(server *mcp.Server, logger *slog.Logger, diag DockerDiagnostics, auditLog *audit.Log) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_restart",
		Description: "Restart a container on a Docker host. This is a dangerous, service-impacting action: it requires `confirm: true`, and returns without acting if confirmation is missing. Set `dry_run: true` to see the exact command that would run without needing confirm at all.",
	}, withAudit(auditLog, "docker_restart", withLogging(logger, "docker_restart", func(ctx context.Context, req *mcp.CallToolRequest, in DockerRestartInput) (*mcp.CallToolResult, DockerRestartOutput, error) {
		if in.DryRun {
			return nil, DockerRestartOutput{
				Server:    in.Server,
				Container: in.Container,
				Status:    "dry_run",
				Message:   fmt.Sprintf("Would run: docker restart %s (on server %q). No action was taken.", in.Container, in.Server),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}, nil
		}

		if !in.Confirm {
			return nil, DockerRestartOutput{
				Server:    in.Server,
				Container: in.Container,
				Status:    "confirmation_required",
				Message:   "Set confirm: true to restart this container. No action was taken.",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}, nil
		}

		if err := diag.Restart(ctx, in.Server, in.Container); err != nil {
			return nil, DockerRestartOutput{}, wrapErr(err)
		}

		return nil, DockerRestartOutput{
			Server:    in.Server,
			Container: in.Container,
			Status:    "restarted",
			Message:   "Container restarted.",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	})))
}
