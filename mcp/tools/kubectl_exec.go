package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// KubectlExecInput targets a single pod (and optionally one of its
// containers) in a namespace on a cluster. Like docker_exec, this is not
// confirm-gated — it follows run_command's convention (arbitrary command,
// no destructive-action gate), not docker_restart's.
type KubectlExecInput struct {
	Cluster   string   `json:"cluster" jsonschema:"the inventory kubernetes cluster name"`
	Namespace string   `json:"namespace" jsonschema:"namespace the pod is in"`
	Pod       string   `json:"pod" jsonschema:"pod name to run the command in"`
	Container string   `json:"container,omitempty" jsonschema:"container name (default: the pod's only/first container)"`
	Command   []string `json:"command" jsonschema:"the command and its arguments to execute inside the container, e.g. [\"cat\", \"/etc/resolv.conf\"]"`
}

// TargetServer implements Targeted.
func (in KubectlExecInput) TargetServer() string { return in.Cluster }

// KubectlExecOutput is the result of kubectl_exec.
type KubectlExecOutput struct {
	Cluster   string   `json:"cluster"`
	Namespace string   `json:"namespace"`
	Pod       string   `json:"pod"`
	Container string   `json:"container,omitempty"`
	Command   []string `json:"command"`
	Stdout    string   `json:"stdout"`
	Stderr    string   `json:"stderr"`
	ExitCode  int      `json:"exit_code"`
	Timestamp string   `json:"timestamp"`
}

// RegisterKubectlExec adds the kubectl_exec tool to server.
func RegisterKubectlExec(server *mcp.Server, logger *slog.Logger, diag KubernetesDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "kubectl_exec",
		Description: "Run a command inside a pod's container and return its output and exit code — the pod-scoped equivalent of run_command/docker_exec.",
	}, withLogging(logger, "kubectl_exec", func(ctx context.Context, req *mcp.CallToolRequest, in KubectlExecInput) (*mcp.CallToolResult, KubectlExecOutput, error) {
		res, err := diag.Exec(ctx, in.Cluster, in.Namespace, in.Pod, in.Container, in.Command)
		if err != nil {
			return nil, KubectlExecOutput{}, wrapErr(err)
		}
		return nil, KubectlExecOutput{
			Cluster:   in.Cluster,
			Namespace: in.Namespace,
			Pod:       in.Pod,
			Container: in.Container,
			Command:   in.Command,
			Stdout:    res.Stdout,
			Stderr:    res.Stderr,
			ExitCode:  res.ExitCode,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
