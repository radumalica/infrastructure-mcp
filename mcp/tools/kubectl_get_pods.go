package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// KubectlGetPodsInput targets a single cluster, optionally scoped to a
// namespace.
type KubectlGetPodsInput struct {
	Cluster   string `json:"cluster" jsonschema:"the inventory kubernetes cluster name"`
	Namespace string `json:"namespace,omitempty" jsonschema:"namespace to list pods in (default: all namespaces)"`
}

// TargetServer implements Targeted.
func (in KubectlGetPodsInput) TargetServer() string { return in.Cluster }

// PodSummary describes one pod.
type PodSummary struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Phase     string `json:"phase"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Node      string `json:"node,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// KubectlGetPodsOutput is the result of kubectl_get_pods.
type KubectlGetPodsOutput struct {
	Cluster   string       `json:"cluster"`
	Namespace string       `json:"namespace,omitempty"`
	Pods      []PodSummary `json:"pods"`
	Timestamp string       `json:"timestamp"`
}

// RegisterKubectlGetPods adds the kubectl_get_pods tool to server.
func RegisterKubectlGetPods(server *mcp.Server, logger *slog.Logger, diag KubernetesDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "kubectl_get_pods",
		Description: "List pods in a Kubernetes cluster, optionally scoped to one namespace.",
	}, withLogging(logger, "kubectl_get_pods", func(ctx context.Context, req *mcp.CallToolRequest, in KubectlGetPodsInput) (*mcp.CallToolResult, KubectlGetPodsOutput, error) {
		pods, err := diag.ListPods(ctx, in.Cluster, in.Namespace)
		if err != nil {
			return nil, KubectlGetPodsOutput{}, wrapErr(err)
		}

		summaries := make([]PodSummary, 0, len(pods))
		for _, p := range pods {
			summaries = append(summaries, PodSummary{
				Name:      p.Name,
				Namespace: p.Namespace,
				Phase:     p.Phase,
				Ready:     p.Ready,
				Restarts:  p.Restarts,
				Node:      p.Node,
				StartTime: p.StartTime,
				Reason:    p.Reason,
			})
		}

		return nil, KubectlGetPodsOutput{
			Cluster:   in.Cluster,
			Namespace: in.Namespace,
			Pods:      summaries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
