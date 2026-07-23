package tools

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/ssh"
)

// RunCommandInput targets a single inventory server and command.
type RunCommandInput struct {
	Server  string `json:"server" jsonschema:"the inventory server name to run the command on"`
	Command string `json:"command" jsonschema:"the shell command to execute on the target"`
}

// RunCommandOutput is the result of executing a command on a server.
type RunCommandOutput struct {
	Server     string `json:"server"`
	Command    string `json:"command"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Timestamp  string `json:"timestamp"`
}

// CommandRunner executes a command on a named inventory server. *ssh.Pool
// satisfies this interface.
type CommandRunner interface {
	Run(ctx context.Context, server, command string) (ssh.Result, error)
}

// RegisterRunCommand adds the run_command tool to server. runner executes
// commands against inventory servers (normally an *ssh.Pool).
func RegisterRunCommand(server *mcp.Server, runner CommandRunner) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_command",
		Description: "Run a shell command on an inventory server over SSH and return its output, exit code, and duration.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in RunCommandInput) (*mcp.CallToolResult, RunCommandOutput, error) {
		res, err := runner.Run(ctx, in.Server, in.Command)
		if err != nil {
			return nil, RunCommandOutput{}, err
		}
		return nil, RunCommandOutput{
			Server:     in.Server,
			Command:    in.Command,
			Stdout:     res.Stdout,
			Stderr:     res.Stderr,
			ExitCode:   res.ExitCode,
			DurationMs: res.Duration.Milliseconds(),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}
