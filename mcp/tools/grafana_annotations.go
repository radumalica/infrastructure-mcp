package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GrafanaAnnotationsInput targets a single Grafana instance, optionally
// scoped to a time range (epoch milliseconds) and/or tags.
type GrafanaAnnotationsInput struct {
	Instance string   `json:"instance" jsonschema:"the inventory grafana instance name"`
	FromMS   int64    `json:"from_ms,omitempty" jsonschema:"start of the time range, epoch milliseconds (default: unbounded)"`
	ToMS     int64    `json:"to_ms,omitempty" jsonschema:"end of the time range, epoch milliseconds (default: unbounded)"`
	Tags     []string `json:"tags,omitempty" jsonschema:"only annotations carrying all of these tags"`
}

// TargetServer implements Targeted.
func (in GrafanaAnnotationsInput) TargetServer() string { return in.Instance }

// AnnotationSummary describes one annotation.
type AnnotationSummary struct {
	ID           int64    `json:"id"`
	DashboardUID string   `json:"dashboard_uid,omitempty"`
	PanelID      int64    `json:"panel_id,omitempty"`
	Time         int64    `json:"time"`
	TimeEnd      int64    `json:"time_end,omitempty"`
	Text         string   `json:"text"`
	Tags         []string `json:"tags,omitempty"`
}

// GrafanaAnnotationsOutput is the result of grafana_annotations.
type GrafanaAnnotationsOutput struct {
	Instance    string              `json:"instance"`
	Annotations []AnnotationSummary `json:"annotations"`
	Timestamp   string              `json:"timestamp"`
}

// RegisterGrafanaAnnotations adds the grafana_annotations tool to server.
func RegisterGrafanaAnnotations(server *mcp.Server, logger *slog.Logger, diag GrafanaDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "grafana_annotations",
		Description: "List annotations on a Grafana instance, optionally scoped to a time range and/or tags.",
	}, withLogging(logger, "grafana_annotations", func(ctx context.Context, req *mcp.CallToolRequest, in GrafanaAnnotationsInput) (*mcp.CallToolResult, GrafanaAnnotationsOutput, error) {
		annotations, err := diag.ListAnnotations(ctx, in.Instance, in.FromMS, in.ToMS, in.Tags)
		if err != nil {
			return nil, GrafanaAnnotationsOutput{}, wrapErr(err)
		}

		summaries := make([]AnnotationSummary, 0, len(annotations))
		for _, a := range annotations {
			summaries = append(summaries, AnnotationSummary{
				ID:           a.ID,
				DashboardUID: a.DashboardUID,
				PanelID:      a.PanelID,
				Time:         a.Time,
				TimeEnd:      a.TimeEnd,
				Text:         a.Text,
				Tags:         a.Tags,
			})
		}

		return nil, GrafanaAnnotationsOutput{
			Instance:    in.Instance,
			Annotations: summaries,
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
