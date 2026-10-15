package tools

import (
	"context"

	"infrastructure-mcp/internal/kubernetes"
)

// KubernetesDiagnostics is satisfied by *kubernetes.Client.
type KubernetesDiagnostics interface {
	ListPods(ctx context.Context, cluster, namespace string) ([]kubernetes.PodEntry, error)
	Logs(ctx context.Context, cluster, namespace, pod, container string, tailLines int64) (string, error)
	ListEvents(ctx context.Context, cluster, namespace string) ([]kubernetes.EventEntry, error)
	DescribePod(ctx context.Context, cluster, namespace, pod string) (kubernetes.PodDescription, error)
	ListNodes(ctx context.Context, cluster string) ([]kubernetes.NodeEntry, error)
	Exec(ctx context.Context, cluster, namespace, pod, container string, command []string) (kubernetes.ExecResult, error)
}
