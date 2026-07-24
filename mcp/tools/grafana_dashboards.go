package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GrafanaDashboardsInput targets a single Grafana instance, optionally
// filtered by a free-text query and/or tag.
type GrafanaDashboardsInput struct {
	Instance string `json:"instance" jsonschema:"the inventory grafana instance name"`
	Query    string `json:"query,omitempty" jsonschema:"free-text search filter on dashboard title"`
	Tag      string `json:"tag,omitempty" jsonschema:"filter to dashboards carrying this tag"`
}

// TargetServer implements Targeted.
func (in GrafanaDashboardsInput) TargetServer() string { return in.Instance }

// DashboardSummary describes one dashboard.
type DashboardSummary struct {
	UID         string   `json:"uid"`
	Title       string   `json:"title"`
	Tags        []string `json:"tags,omitempty"`
	URL         string   `json:"url"`
	FolderTitle string   `json:"folder_title,omitempty"`
}

// GrafanaDashboardsOutput is the result of grafana_dashboards.
type GrafanaDashboardsOutput struct {
	Instance   string             `json:"instance"`
	Dashboards []DashboardSummary `json:"dashboards"`
	Timestamp  string             `json:"timestamp"`
}

// RegisterGrafanaDashboards adds the grafana_dashboards tool to server.
func RegisterGrafanaDashboards(server *mcp.Server, logger *slog.Logger, diag GrafanaDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "grafana_dashboards",
		Description: "Search dashboards on a Grafana instance, optionally filtered by title text or tag.",
	}, withLogging(logger, "grafana_dashboards", func(ctx context.Context, req *mcp.CallToolRequest, in GrafanaDashboardsInput) (*mcp.CallToolResult, GrafanaDashboardsOutput, error) {
		dashboards, err := diag.ListDashboards(ctx, in.Instance, in.Query, in.Tag)
		if err != nil {
			return nil, GrafanaDashboardsOutput{}, wrapErr(err)
		}

		summaries := make([]DashboardSummary, 0, len(dashboards))
		for _, d := range dashboards {
			summaries = append(summaries, DashboardSummary{
				UID:         d.UID,
				Title:       d.Title,
				Tags:        d.Tags,
				URL:         d.URL,
				FolderTitle: d.FolderTitle,
			})
		}

		return nil, GrafanaDashboardsOutput{
			Instance:   in.Instance,
			Dashboards: summaries,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
