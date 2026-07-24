package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// KubectlDescribeInput targets a single pod in a namespace on a cluster.
type KubectlDescribeInput struct {
	Cluster   string `json:"cluster" jsonschema:"the inventory kubernetes cluster name"`
	Namespace string `json:"namespace" jsonschema:"namespace the pod is in"`
	Pod       string `json:"pod" jsonschema:"pod name to describe"`
}

// TargetServer implements Targeted.
func (in KubectlDescribeInput) TargetServer() string { return in.Cluster }

// ContainerStatusSummary describes one container's status within a
// described pod.
type ContainerStatusSummary struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	Image        string `json:"image"`
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
}

// KubectlDescribeOutput is the result of kubectl_describe — a structured
// stand-in for `kubectl describe pod`, not a byte-for-byte port of its
// free-text output.
type KubectlDescribeOutput struct {
	Cluster    string                   `json:"cluster"`
	Namespace  string                   `json:"namespace"`
	Pod        string                   `json:"pod"`
	Phase      string                   `json:"phase"`
	Node       string                   `json:"node,omitempty"`
	PodIP      string                   `json:"pod_ip,omitempty"`
	StartTime  string                   `json:"start_time,omitempty"`
	Containers []ContainerStatusSummary `json:"containers"`
	Events     []EventSummary           `json:"events"`
	Timestamp  string                   `json:"timestamp"`
}

// RegisterKubectlDescribe adds the kubectl_describe tool to server.
func RegisterKubectlDescribe(server *mcp.Server, logger *slog.Logger, diag KubernetesDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "kubectl_describe",
		Description: "Describe a pod: its spec/status highlights plus recent events for it.",
	}, withLogging(logger, "kubectl_describe", func(ctx context.Context, req *mcp.CallToolRequest, in KubectlDescribeInput) (*mcp.CallToolResult, KubectlDescribeOutput, error) {
		desc, err := diag.DescribePod(ctx, in.Cluster, in.Namespace, in.Pod)
		if err != nil {
			return nil, KubectlDescribeOutput{}, wrapErr(err)
		}

		containers := make([]ContainerStatusSummary, 0, len(desc.Containers))
		for _, c := range desc.Containers {
			containers = append(containers, ContainerStatusSummary{
				Name:         c.Name,
				Ready:        c.Ready,
				RestartCount: c.RestartCount,
				Image:        c.Image,
				State:        c.State,
				Reason:       c.Reason,
			})
		}

		events := make([]EventSummary, 0, len(desc.Events))
		for _, e := range desc.Events {
			events = append(events, EventSummary{
				Type:      e.Type,
				Reason:    e.Reason,
				Object:    e.Object,
				Message:   e.Message,
				Count:     e.Count,
				FirstSeen: e.FirstSeen,
				LastSeen:  e.LastSeen,
			})
		}

		return nil, KubectlDescribeOutput{
			Cluster:    in.Cluster,
			Namespace:  desc.Namespace,
			Pod:        desc.Name,
			Phase:      desc.Phase,
			Node:       desc.Node,
			PodIP:      desc.PodIP,
			StartTime:  desc.StartTime,
			Containers: containers,
			Events:     events,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
