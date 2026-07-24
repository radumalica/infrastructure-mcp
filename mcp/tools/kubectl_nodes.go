package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// KubectlNodesInput targets a single cluster.
type KubectlNodesInput struct {
	Cluster string `json:"cluster" jsonschema:"the inventory kubernetes cluster name"`
}

// TargetServer implements Targeted.
func (in KubectlNodesInput) TargetServer() string { return in.Cluster }

// NodeSummary describes one cluster node.
type NodeSummary struct {
	Name             string   `json:"name"`
	Ready            bool     `json:"ready"`
	Roles            []string `json:"roles,omitempty"`
	Unschedulable    bool     `json:"unschedulable"`
	KubeletVersion   string   `json:"kubelet_version,omitempty"`
	OSImage          string   `json:"os_image,omitempty"`
	KernelVersion    string   `json:"kernel_version,omitempty"`
	ContainerRuntime string   `json:"container_runtime,omitempty"`
	CPUCapacity      string   `json:"cpu_capacity,omitempty"`
	MemoryCapacity   string   `json:"memory_capacity,omitempty"`
	InternalIP       string   `json:"internal_ip,omitempty"`
}

// KubectlNodesOutput is the result of kubectl_nodes.
type KubectlNodesOutput struct {
	Cluster   string        `json:"cluster"`
	Nodes     []NodeSummary `json:"nodes"`
	Timestamp string        `json:"timestamp"`
}

// RegisterKubectlNodes adds the kubectl_nodes tool to server.
func RegisterKubectlNodes(server *mcp.Server, logger *slog.Logger, diag KubernetesDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "kubectl_nodes",
		Description: "List nodes in a Kubernetes cluster with readiness, roles, and capacity.",
	}, withLogging(logger, "kubectl_nodes", func(ctx context.Context, req *mcp.CallToolRequest, in KubectlNodesInput) (*mcp.CallToolResult, KubectlNodesOutput, error) {
		nodes, err := diag.ListNodes(ctx, in.Cluster)
		if err != nil {
			return nil, KubectlNodesOutput{}, wrapErr(err)
		}

		summaries := make([]NodeSummary, 0, len(nodes))
		for _, n := range nodes {
			summaries = append(summaries, NodeSummary{
				Name:             n.Name,
				Ready:            n.Ready,
				Roles:            n.Roles,
				Unschedulable:    n.Unschedulable,
				KubeletVersion:   n.KubeletVersion,
				OSImage:          n.OSImage,
				KernelVersion:    n.KernelVersion,
				ContainerRuntime: n.ContainerRuntime,
				CPUCapacity:      n.CPUCapacity,
				MemoryCapacity:   n.MemoryCapacity,
				InternalIP:       n.InternalIP,
			})
		}

		return nil, KubectlNodesOutput{
			Cluster:   in.Cluster,
			Nodes:     summaries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
