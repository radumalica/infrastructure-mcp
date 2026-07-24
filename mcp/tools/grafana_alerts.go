package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GrafanaAlertsInput targets a single Grafana instance.
type GrafanaAlertsInput struct {
	Instance string `json:"instance" jsonschema:"the inventory grafana instance name"`
}

// TargetServer implements Targeted.
func (in GrafanaAlertsInput) TargetServer() string { return in.Instance }

// AlertSummary describes one firing/resolved alert instance.
type AlertSummary struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	StartsAt     string            `json:"starts_at"`
	EndsAt       string            `json:"ends_at,omitempty"`
	Fingerprint  string            `json:"fingerprint"`
	GeneratorURL string            `json:"generator_url,omitempty"`
}

// GrafanaAlertsOutput is the result of grafana_alerts.
type GrafanaAlertsOutput struct {
	Instance  string         `json:"instance"`
	Alerts    []AlertSummary `json:"alerts"`
	Timestamp string         `json:"timestamp"`
}

// RegisterGrafanaAlerts adds the grafana_alerts tool to server.
func RegisterGrafanaAlerts(server *mcp.Server, logger *slog.Logger, diag GrafanaDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "grafana_alerts",
		Description: "List currently firing or resolved alert instances from a Grafana instance's alerting API.",
	}, withLogging(logger, "grafana_alerts", func(ctx context.Context, req *mcp.CallToolRequest, in GrafanaAlertsInput) (*mcp.CallToolResult, GrafanaAlertsOutput, error) {
		alerts, err := diag.ListAlerts(ctx, in.Instance)
		if err != nil {
			return nil, GrafanaAlertsOutput{}, wrapErr(err)
		}

		summaries := make([]AlertSummary, 0, len(alerts))
		for _, a := range alerts {
			summaries = append(summaries, AlertSummary{
				Status:       a.Status,
				Labels:       a.Labels,
				Annotations:  a.Annotations,
				StartsAt:     a.StartsAt,
				EndsAt:       a.EndsAt,
				Fingerprint:  a.Fingerprint,
				GeneratorURL: a.GeneratorURL,
			})
		}

		return nil, GrafanaAlertsOutput{
			Instance:  in.Instance,
			Alerts:    summaries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
