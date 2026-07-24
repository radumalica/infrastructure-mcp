package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GrafanaQueryInput runs a single raw query, in the target datasource's
// own query language (PromQL, LogQL, SQL, ...), against one datasource
// on one Grafana instance.
type GrafanaQueryInput struct {
	Instance      string `json:"instance" jsonschema:"the inventory grafana instance name"`
	DatasourceUID string `json:"datasource_uid" jsonschema:"UID of the datasource to query (see its dashboard/settings URL)"`
	Query         string `json:"query" jsonschema:"raw query in the datasource's own language, e.g. a PromQL or LogQL expression"`
	From          string `json:"from,omitempty" jsonschema:"start of the time range: a Grafana relative unit like now-1h, or epoch milliseconds (default: now-1h)"`
	To            string `json:"to,omitempty" jsonschema:"end of the time range: a Grafana relative unit like now, or epoch milliseconds (default: now)"`
}

// TargetServer implements Targeted.
func (in GrafanaQueryInput) TargetServer() string { return in.Instance }

// GrafanaQueryOutput is the result of grafana_query. Result is a
// passthrough of Grafana's decoded JSON response — its shape is
// datasource-specific (a Prometheus matrix, a Loki stream, a SQL table
// all serialize differently), so it is deliberately not normalized here.
type GrafanaQueryOutput struct {
	Instance      string         `json:"instance"`
	DatasourceUID string         `json:"datasource_uid"`
	Result        map[string]any `json:"result"`
	Timestamp     string         `json:"timestamp"`
}

// RegisterGrafanaQuery adds the grafana_query tool to server.
func RegisterGrafanaQuery(server *mcp.Server, logger *slog.Logger, diag GrafanaDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "grafana_query",
		Description: "Run a raw query against one datasource on a Grafana instance. The response shape is datasource-specific (Prometheus, Loki, SQL, ...); this is a passthrough, not a normalized result.",
	}, withLogging(logger, "grafana_query", func(ctx context.Context, req *mcp.CallToolRequest, in GrafanaQueryInput) (*mcp.CallToolResult, GrafanaQueryOutput, error) {
		result, err := diag.Query(ctx, in.Instance, in.DatasourceUID, in.Query, in.From, in.To)
		if err != nil {
			return nil, GrafanaQueryOutput{}, wrapErr(err)
		}

		return nil, GrafanaQueryOutput{
			Instance:      in.Instance,
			DatasourceUID: in.DatasourceUID,
			Result:        result.Raw,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
