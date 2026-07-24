package tools

import (
	"context"

	"infrastructure-mcp/internal/grafana"
)

// GrafanaDiagnostics is satisfied by *grafana.Client.
type GrafanaDiagnostics interface {
	ListAlerts(ctx context.Context, instance string) ([]grafana.AlertEntry, error)
	ListDashboards(ctx context.Context, instance, query, tag string) ([]grafana.DashboardEntry, error)
	ListAnnotations(ctx context.Context, instance string, fromMS, toMS int64, tags []string) ([]grafana.AnnotationEntry, error)
	Query(ctx context.Context, instance, datasourceUID, expr, from, to string) (grafana.QueryResult, error)
}
