// Package grafana talks to a Grafana instance's HTTP API — alerting,
// dashboard search, ad-hoc datasource queries, and annotations. All
// authentication (service account token, or basic auth) lives inside the
// referenced inventory.ServiceEndpoint and is never exposed to callers.
package grafana

// AlertEntry describes one alert instance from the Alertmanager-compatible
// alerting API (firing or resolved), not an alert *rule* definition.
type AlertEntry struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"starts_at"`
	EndsAt       string            `json:"ends_at,omitempty"`
	Fingerprint  string            `json:"fingerprint"`
	GeneratorURL string            `json:"generator_url,omitempty"`
}

// DashboardEntry describes one dashboard or folder returned by a search.
type DashboardEntry struct {
	UID         string   `json:"uid"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags,omitempty"`
	URL         string   `json:"url"`
	FolderTitle string   `json:"folder_title,omitempty"`
}

// AnnotationEntry describes one annotation returned by /api/annotations.
type AnnotationEntry struct {
	ID           int64    `json:"id"`
	DashboardUID string   `json:"dashboard_uid,omitempty"`
	PanelID      int64    `json:"panel_id,omitempty"`
	Time         int64    `json:"time"`
	TimeEnd      int64    `json:"time_end,omitempty"`
	Text         string   `json:"text"`
	Tags         []string `json:"tags,omitempty"`
}

// QueryResult is the raw response of a POST /api/ds/query call. Grafana's
// response shape (data frames) is inherently datasource-specific — a
// Prometheus matrix, a Loki stream, and a SQL table all serialize
// differently — so this is deliberately a passthrough of the decoded JSON
// rather than a normalized struct. See PROGRESS.md v0.5 entry.
type QueryResult struct {
	Raw map[string]any `json:"raw"`
}
