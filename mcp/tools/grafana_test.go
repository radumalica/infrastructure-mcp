package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/grafana"
	"infrastructure-mcp/internal/inventory"
)

type fakeGrafanaDiagnostics struct {
	alerts      []grafana.AlertEntry
	dashboards  []grafana.DashboardEntry
	annotations []grafana.AnnotationEntry
	query       grafana.QueryResult
	err         error
	sawInstance string
	sawDsUID    string
	sawExpr     string
}

func (f *fakeGrafanaDiagnostics) ListAlerts(_ context.Context, instance string) ([]grafana.AlertEntry, error) {
	f.sawInstance = instance
	return f.alerts, f.err
}
func (f *fakeGrafanaDiagnostics) ListDashboards(_ context.Context, instance, _, _ string) ([]grafana.DashboardEntry, error) {
	f.sawInstance = instance
	return f.dashboards, f.err
}
func (f *fakeGrafanaDiagnostics) ListAnnotations(_ context.Context, instance string, _, _ int64, _ []string) ([]grafana.AnnotationEntry, error) {
	f.sawInstance = instance
	return f.annotations, f.err
}
func (f *fakeGrafanaDiagnostics) Query(_ context.Context, instance, dsUID, expr, _, _ string) (grafana.QueryResult, error) {
	f.sawInstance, f.sawDsUID, f.sawExpr = instance, dsUID, expr
	return f.query, f.err
}

func newGrafanaSession(t *testing.T, diag *fakeGrafanaDiagnostics) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterGrafanaAlerts(server, testLogger(), diag)
	RegisterGrafanaDashboards(server, testLogger(), diag)
	RegisterGrafanaAnnotations(server, testLogger(), diag)
	RegisterGrafanaQuery(server, testLogger(), diag)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go func() { _, _ = server.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestGrafanaAlerts_ViaMCPProtocol(t *testing.T) {
	diag := &fakeGrafanaDiagnostics{alerts: []grafana.AlertEntry{
		{Status: "active", Labels: map[string]string{"alertname": "HighCPU"}, Fingerprint: "abc"},
	}}
	session := newGrafanaSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "grafana_alerts",
		Arguments: map[string]any{"instance": "main"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawInstance != "main" {
		t.Errorf("sawInstance = %q, want main", diag.sawInstance)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out GrafanaAlertsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Alerts) != 1 || out.Alerts[0].Status != "active" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestGrafanaDashboards_ViaMCPProtocol(t *testing.T) {
	diag := &fakeGrafanaDiagnostics{dashboards: []grafana.DashboardEntry{
		{UID: "cIBgcSjkk", Title: "Production Overview", URL: "/d/cIBgcSjkk/production-overview"},
	}}
	session := newGrafanaSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "grafana_dashboards",
		Arguments: map[string]any{"instance": "main", "query": "Production"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out GrafanaDashboardsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Dashboards) != 1 || out.Dashboards[0].UID != "cIBgcSjkk" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestGrafanaAnnotations_ViaMCPProtocol(t *testing.T) {
	diag := &fakeGrafanaDiagnostics{annotations: []grafana.AnnotationEntry{
		{ID: 1124, Text: "test", Tags: []string{"tag1"}},
	}}
	session := newGrafanaSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "grafana_annotations",
		Arguments: map[string]any{"instance": "main"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out GrafanaAnnotationsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Annotations) != 1 || out.Annotations[0].ID != 1124 {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestGrafanaQuery_ViaMCPProtocol(t *testing.T) {
	diag := &fakeGrafanaDiagnostics{query: grafana.QueryResult{Raw: map[string]any{"results": map[string]any{"A": map[string]any{}}}}}
	session := newGrafanaSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "grafana_query",
		Arguments: map[string]any{"instance": "main", "datasource_uid": "prom-uid", "query": "up"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawDsUID != "prom-uid" || diag.sawExpr != "up" {
		t.Errorf("sawDsUID=%q sawExpr=%q", diag.sawDsUID, diag.sawExpr)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out GrafanaQueryOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Result["results"] == nil {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestGrafanaTools_ErrorPath(t *testing.T) {
	diag := &fakeGrafanaDiagnostics{err: errors.New("boom")}
	session := newGrafanaSession(t, diag)
	ctx := context.Background()

	cases := []struct {
		tool string
		args map[string]any
	}{
		{"grafana_alerts", map[string]any{"instance": "main"}},
		{"grafana_dashboards", map[string]any{"instance": "main"}},
		{"grafana_annotations", map[string]any{"instance": "main"}},
		{"grafana_query", map[string]any{"instance": "main", "datasource_uid": "prom-uid", "query": "up"}},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool transport error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result for %s", tc.tool)
			}
		})
	}
}

func TestGrafanaAlerts_InstanceNotFound(t *testing.T) {
	diag := &fakeGrafanaDiagnostics{err: inventory.ErrNotFound}
	session := newGrafanaSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "grafana_alerts",
		Arguments: map[string]any{"instance": "does-not-exist"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for a missing instance")
	}
}
