package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// KubectlEventsInput targets a single cluster, optionally scoped to a
// namespace.
type KubectlEventsInput struct {
	Cluster   string `json:"cluster" jsonschema:"the inventory kubernetes cluster name"`
	Namespace string `json:"namespace,omitempty" jsonschema:"namespace to list events in (default: all namespaces)"`
}

// TargetServer implements Targeted.
func (in KubectlEventsInput) TargetServer() string { return in.Cluster }

// EventSummary describes one cluster event.
type EventSummary struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Object    string `json:"object"`
	Message   string `json:"message"`
	Count     int32  `json:"count"`
	FirstSeen string `json:"first_seen,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
}

// KubectlEventsOutput is the result of kubectl_events.
type KubectlEventsOutput struct {
	Cluster   string         `json:"cluster"`
	Namespace string         `json:"namespace,omitempty"`
	Events    []EventSummary `json:"events"`
	Timestamp string         `json:"timestamp"`
}

// RegisterKubectlEvents adds the kubectl_events tool to server.
func RegisterKubectlEvents(server *mcp.Server, logger *slog.Logger, diag KubernetesDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "kubectl_events",
		Description: "List recent Kubernetes events, most recent first, optionally scoped to one namespace.",
	}, withLogging(logger, "kubectl_events", func(ctx context.Context, req *mcp.CallToolRequest, in KubectlEventsInput) (*mcp.CallToolResult, KubectlEventsOutput, error) {
		events, err := diag.ListEvents(ctx, in.Cluster, in.Namespace)
		if err != nil {
			return nil, KubectlEventsOutput{}, wrapErr(err)
		}

		summaries := make([]EventSummary, 0, len(events))
		for _, e := range events {
			summaries = append(summaries, EventSummary{
				Type:      e.Type,
				Reason:    e.Reason,
				Object:    e.Object,
				Message:   e.Message,
				Count:     e.Count,
				FirstSeen: e.FirstSeen,
				LastSeen:  e.LastSeen,
			})
		}

		return nil, KubectlEventsOutput{
			Cluster:   in.Cluster,
			Namespace: in.Namespace,
			Events:    summaries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
