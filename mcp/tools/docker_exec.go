package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DockerExecInput targets a single container on a single server. Unlike
// docker_restart, this is not confirm-gated — it follows run_command's
// convention (arbitrary command, no destructive-action gate) rather than
// docker_restart's (a single named destructive action).
type DockerExecInput struct {
	Server    string `json:"server" jsonschema:"the inventory server name the container runs on"`
	Container string `json:"container" jsonschema:"container name or ID to run the command in"`
	Command   string `json:"command" jsonschema:"the shell command to execute inside the container"`
}

// TargetServer implements Targeted.
func (in DockerExecInput) TargetServer() string { return in.Server }

// DockerExecOutput is the result of docker_exec.
type DockerExecOutput struct {
	Server    string `json:"server"`
	Container string `json:"container"`
	Command   string `json:"command"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Timestamp string `json:"timestamp"`
}

// RegisterDockerExec adds the docker_exec tool to server.
func RegisterDockerExec(server *mcp.Server, logger *slog.Logger, diag DockerDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_exec",
		Description: "Run a shell command inside a running container and return its output and exit code — the container-scoped equivalent of run_command.",
	}, withLogging(logger, "docker_exec", func(ctx context.Context, req *mcp.CallToolRequest, in DockerExecInput) (*mcp.CallToolResult, DockerExecOutput, error) {
		res, err := diag.Exec(ctx, in.Server, in.Container, in.Command)
		if err != nil {
			return nil, DockerExecOutput{}, wrapErr(err)
		}
		return nil, DockerExecOutput{
			Server:    in.Server,
			Container: in.Container,
			Command:   in.Command,
			Stdout:    res.Stdout,
			Stderr:    res.Stderr,
			ExitCode:  res.ExitCode,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
