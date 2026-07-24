package tools

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// KubectlLogsInput targets a single pod (and optionally one of its
// containers) in a namespace on a cluster.
type KubectlLogsInput struct {
	Cluster   string `json:"cluster" jsonschema:"the inventory kubernetes cluster name"`
	Namespace string `json:"namespace" jsonschema:"namespace the pod is in"`
	Pod       string `json:"pod" jsonschema:"pod name to fetch logs for"`
	Container string `json:"container,omitempty" jsonschema:"container name (default: the pod's only/first container)"`
	Tail      int64  `json:"tail,omitempty" jsonschema:"number of most recent log lines to return (default: all available)"`
}

// TargetServer implements Targeted.
func (in KubectlLogsInput) TargetServer() string { return in.Cluster }

// KubectlLogsOutput is the result of kubectl_logs.
type KubectlLogsOutput struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container,omitempty"`
	Logs      string `json:"logs"`
	LineCount int    `json:"line_count"`
	Timestamp string `json:"timestamp"`
}

// RegisterKubectlLogs adds the kubectl_logs tool to server.
func RegisterKubectlLogs(server *mcp.Server, logger *slog.Logger, diag KubernetesDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "kubectl_logs",
		Description: "Fetch log lines from a pod's container in a Kubernetes cluster.",
	}, withLogging(logger, "kubectl_logs", func(ctx context.Context, req *mcp.CallToolRequest, in KubectlLogsInput) (*mcp.CallToolResult, KubectlLogsOutput, error) {
		logs, err := diag.Logs(ctx, in.Cluster, in.Namespace, in.Pod, in.Container, in.Tail)
		if err != nil {
			return nil, KubectlLogsOutput{}, wrapErr(err)
		}

		lineCount := 0
		if trimmed := strings.TrimRight(logs, "\n"); trimmed != "" {
			lineCount = strings.Count(trimmed, "\n") + 1
		}

		return nil, KubectlLogsOutput{
			Cluster:   in.Cluster,
			Namespace: in.Namespace,
			Pod:       in.Pod,
			Container: in.Container,
			Logs:      logs,
			LineCount: lineCount,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
