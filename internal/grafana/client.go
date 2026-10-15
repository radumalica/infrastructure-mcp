package grafana

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"infrastructure-mcp/internal/inventory"
)

// ErrUnauthorized indicates the instance rejected our credentials (HTTP
// 401/403).
var ErrUnauthorized = errors.New("grafana: unauthorized")

// EndpointLookup resolves an inventory Grafana instance name to its
// connection details. *inventory.Inventory satisfies this; tests
// substitute a fake.
type EndpointLookup interface {
	GrafanaEndpoint(name string) (inventory.ServiceEndpoint, error)
}

// Client calls the HTTP API of any Grafana instance in the inventory.
type Client struct {
	inv        EndpointLookup
	httpClient *http.Client

	mu              sync.Mutex
	insecureClients map[string]*http.Client
}

// New creates a Client that resolves instance names against inv, using
// httpClient for requests. Pass nil to use http.DefaultClient with a
// 15s timeout.
func New(inv EndpointLookup, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{inv: inv, httpClient: httpClient, insecureClients: make(map[string]*http.Client)}
}

// httpClientFor returns the Client's normal shared http.Client, unless
// instance's inventory entry opts into InsecureSkipVerify, in which case
// it returns a dedicated client (built once, then cached) with TLS
// certificate verification disabled — never done globally, only for the
// one instance that explicitly asked for it.
func (c *Client) httpClientFor(instance string, ep inventory.ServiceEndpoint) *http.Client {
	if !ep.InsecureSkipVerify {
		return c.httpClient
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if cl, ok := c.insecureClients[instance]; ok {
		return cl
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	cl := &http.Client{Timeout: c.httpClient.Timeout, Transport: transport}
	c.insecureClients[instance] = cl
	return cl
}

func (c *Client) doRequest(ctx context.Context, instance, method, path string, query url.Values, body any) ([]byte, error) {
	ep, err := c.inv.GrafanaEndpoint(instance)
	if err != nil {
		return nil, err
	}

	fullURL := strings.TrimRight(ep.URL, "/") + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("grafana: encode request body: %w", err)
		}
		reqBody = strings.NewReader(string(b))
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("grafana: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch {
	case ep.Token != "":
		req.Header.Set("Authorization", "Bearer "+ep.Token)
	case ep.User != "":
		req.SetBasicAuth(ep.User, ep.Password)
	}

	resp, err := c.httpClientFor(instance, ep).Do(req)
	if err != nil {
		return nil, fmt.Errorf("grafana: request to instance %q: %w", instance, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("grafana: read response from instance %q: %w", instance, err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: instance %q returned %d", ErrUnauthorized, instance, resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%w: %s %s on instance %q", inventory.ErrNotFound, method, path, instance)
	case resp.StatusCode >= 300:
		return nil, fmt.Errorf("grafana: instance %q returned %d: %s", instance, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ListAlerts returns the currently firing/resolved alert instances from
// the Alertmanager-compatible alerting API (not alert rule definitions).
func (c *Client) ListAlerts(ctx context.Context, instance string) ([]AlertEntry, error) {
	body, err := c.doRequest(ctx, instance, http.MethodGet, "/api/alertmanager/grafana/api/v2/alerts", nil, nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Status struct {
			State string `json:"state"`
		} `json:"status"`
		Labels       map[string]string `json:"labels"`
		Annotations  map[string]string `json:"annotations"`
		StartsAt     string            `json:"startsAt"`
		EndsAt       string            `json:"endsAt"`
		Fingerprint  string            `json:"fingerprint"`
		GeneratorURL string            `json:"generatorURL"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("grafana: decode alerts response: %w", err)
	}

	alerts := make([]AlertEntry, 0, len(raw))
	for _, a := range raw {
		alerts = append(alerts, AlertEntry{
			Status:       a.Status.State,
			Labels:       a.Labels,
			Annotations:  a.Annotations,
			StartsAt:     a.StartsAt,
			EndsAt:       a.EndsAt,
			Fingerprint:  a.Fingerprint,
			GeneratorURL: a.GeneratorURL,
		})
	}
	return alerts, nil
}

// ListDashboards searches dashboards (and folders), optionally filtered
// by a free-text query and/or tag.
func (c *Client) ListDashboards(ctx context.Context, instance, query, tag string) ([]DashboardEntry, error) {
	q := url.Values{}
	q.Set("type", "dash-db")
	if query != "" {
		q.Set("query", query)
	}
	if tag != "" {
		q.Set("tag", tag)
	}

	body, err := c.doRequest(ctx, instance, http.MethodGet, "/api/search", q, nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		UID         string   `json:"uid"`
		Title       string   `json:"title"`
		Type        string   `json:"type"`
		Tags        []string `json:"tags"`
		URL         string   `json:"url"`
		FolderTitle string   `json:"folderTitle"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("grafana: decode search response: %w", err)
	}

	dashboards := make([]DashboardEntry, 0, len(raw))
	for _, d := range raw {
		dashboards = append(dashboards, DashboardEntry{
			UID:         d.UID,
			Title:       d.Title,
			Type:        d.Type,
			Tags:        d.Tags,
			URL:         d.URL,
			FolderTitle: d.FolderTitle,
		})
	}
	return dashboards, nil
}

// ListAnnotations returns annotations, optionally filtered by a time
// range (epoch milliseconds; zero means unbounded) and/or tags.
func (c *Client) ListAnnotations(ctx context.Context, instance string, fromMS, toMS int64, tags []string) ([]AnnotationEntry, error) {
	q := url.Values{}
	if fromMS > 0 {
		q.Set("from", strconv.FormatInt(fromMS, 10))
	}
	if toMS > 0 {
		q.Set("to", strconv.FormatInt(toMS, 10))
	}
	for _, t := range tags {
		q.Add("tags", t)
	}

	body, err := c.doRequest(ctx, instance, http.MethodGet, "/api/annotations", q, nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID           int64    `json:"id"`
		DashboardUID string   `json:"dashboardUID"`
		PanelID      int64    `json:"panelId"`
		Time         int64    `json:"time"`
		TimeEnd      int64    `json:"timeEnd"`
		Text         string   `json:"text"`
		Tags         []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("grafana: decode annotations response: %w", err)
	}

	annotations := make([]AnnotationEntry, 0, len(raw))
	for _, a := range raw {
		annotations = append(annotations, AnnotationEntry{
			ID:           a.ID,
			DashboardUID: a.DashboardUID,
			PanelID:      a.PanelID,
			Time:         a.Time,
			TimeEnd:      a.TimeEnd,
			Text:         a.Text,
			Tags:         a.Tags,
		})
	}
	return annotations, nil
}

// dsQueryRequest mirrors the POST /api/ds/query body Grafana expects: a
// list of queries, each targeting one datasource by UID, plus a shared
// time range (relative Grafana time units or epoch-ms strings).
type dsQueryRequest struct {
	Queries []dsQuery `json:"queries"`
	From    string    `json:"from"`
	To      string    `json:"to"`
}

type dsQuery struct {
	RefID      string           `json:"refId"`
	Datasource dsQueryReference `json:"datasource"`
	Expr       string           `json:"expr,omitempty"`
}

type dsQueryReference struct {
	UID string `json:"uid"`
}

// Query runs a single raw query (in the target datasource's own query
// language — PromQL, LogQL, SQL, ...) against one datasource via
// POST /api/ds/query. The response is datasource-shaped (data frames)
// and is deliberately returned as a passthrough rather than normalized —
// see QueryResult's doc comment. from/to accept Grafana relative time
// units ("now-1h") or epoch-ms strings; empty defaults to "now-1h"/"now".
func (c *Client) Query(ctx context.Context, instance, datasourceUID, expr, from, to string) (QueryResult, error) {
	if from == "" {
		from = "now-1h"
	}
	if to == "" {
		to = "now"
	}

	reqBody := dsQueryRequest{
		Queries: []dsQuery{{
			RefID:      "A",
			Datasource: dsQueryReference{UID: datasourceUID},
			Expr:       expr,
		}},
		From: from,
		To:   to,
	}

	body, err := c.doRequest(ctx, instance, http.MethodPost, "/api/ds/query", nil, reqBody)
	if err != nil {
		return QueryResult{}, err
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return QueryResult{}, fmt.Errorf("grafana: decode query response: %w", err)
	}
	return QueryResult{Raw: raw}, nil
}
