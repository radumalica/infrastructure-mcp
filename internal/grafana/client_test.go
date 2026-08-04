package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"infrastructure-mcp/internal/inventory"
)

type fakeLookup struct {
	ep  inventory.ServiceEndpoint
	err error
}

func (f fakeLookup) GrafanaEndpoint(name string) (inventory.ServiceEndpoint, error) {
	return f.ep, f.err
}

func testClient(t *testing.T, srv *httptest.Server, token string) *Client {
	t.Helper()
	return New(fakeLookup{ep: inventory.ServiceEndpoint{URL: srv.URL, Token: token}}, srv.Client())
}

func TestListAlerts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("expected bearer token header, got %q", got)
		}
		if r.URL.Path != "/api/alertmanager/grafana/api/v2/alerts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"status":       map[string]any{"state": "active"},
				"labels":       map[string]string{"alertname": "HighCPU"},
				"annotations":  map[string]string{"summary": "cpu hot"},
				"startsAt":     "2026-07-24T10:00:00Z",
				"endsAt":       "0001-01-01T00:00:00Z",
				"fingerprint":  "abc123",
				"generatorURL": "https://grafana.lab.local/alerting",
			},
		})
	}))
	defer srv.Close()

	alerts, err := testClient(t, srv, "secret").ListAlerts(context.Background(), "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 1 || alerts[0].Status != "active" || alerts[0].Labels["alertname"] != "HighCPU" {
		t.Errorf("unexpected alerts: %+v", alerts)
	}
}

func TestListDashboards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("type") != "dash-db" {
			t.Errorf("expected type=dash-db, got %q", r.URL.Query().Get("type"))
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"uid": "cIBgcSjkk", "title": "Production Overview", "type": "dash-db", "tags": []string{"prod"}, "url": "/d/cIBgcSjkk/production-overview"},
		})
	}))
	defer srv.Close()

	dashboards, err := testClient(t, srv, "secret").ListDashboards(context.Background(), "main", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dashboards) != 1 || dashboards[0].UID != "cIBgcSjkk" {
		t.Errorf("unexpected dashboards: %+v", dashboards)
	}
}

func TestListAnnotations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/annotations" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1124, "dashboardUID": "uGlb_lG7z", "panelId": 2, "time": 1507266395000, "text": "test", "tags": []string{"tag1"}},
		})
	}))
	defer srv.Close()

	annotations, err := testClient(t, srv, "secret").ListAnnotations(context.Background(), "main", 0, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(annotations) != 1 || annotations[0].ID != 1124 {
		t.Errorf("unexpected annotations: %+v", annotations)
	}
}

func TestQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/ds/query" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body dsQueryRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if len(body.Queries) != 1 || body.Queries[0].Datasource.UID != "prom-uid" || body.Queries[0].Expr != "up" {
			t.Errorf("unexpected request body: %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": map[string]any{"A": map[string]any{"status": 200}}})
	}))
	defer srv.Close()

	result, err := testClient(t, srv, "secret").Query(context.Background(), "main", "prom-uid", "up", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Raw["results"] == nil {
		t.Errorf("unexpected query result: %+v", result)
	}
}

func TestDoRequest_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := testClient(t, srv, "bad-token").ListAlerts(context.Background(), "main")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDoRequest_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := testClient(t, srv, "secret").ListAlerts(context.Background(), "main")
	if !errors.Is(err, inventory.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDoRequest_InstanceNotFound(t *testing.T) {
	c := New(fakeLookup{err: inventory.ErrNotFound}, http.DefaultClient)
	_, err := c.ListAlerts(context.Background(), "does-not-exist")
	if !errors.Is(err, inventory.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDoRequest_BasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "hunter2" {
			t.Errorf("expected basic auth admin/hunter2, got %q/%q (ok=%v)", user, pass, ok)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer srv.Close()

	c := New(fakeLookup{ep: inventory.ServiceEndpoint{URL: srv.URL, User: "admin", Password: "hunter2"}}, srv.Client())
	if _, err := c.ListAlerts(context.Background(), "main"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDoRequest_SelfSignedCert_RejectedByDefault proves the safe default:
// a self-signed cert (the same shape a stock Proxmox/Grafana-behind-a-
// homelab-reverse-proxy install ships) is rejected unless the instance
// explicitly opts into InsecureSkipVerify. Uses a plain *http.Client, not
// srv.Client() (which already trusts the test server's cert) — that
// would defeat the point of this test.
func TestDoRequest_SelfSignedCert_RejectedByDefault(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer srv.Close()

	c := New(fakeLookup{ep: inventory.ServiceEndpoint{URL: srv.URL}}, &http.Client{})
	if _, err := c.ListAlerts(context.Background(), "main"); err == nil {
		t.Fatal("expected a certificate verification error against a self-signed cert by default")
	}
}

// TestDoRequest_InsecureSkipVerify proves the opt-in actually works, and
// only for the instance that set it.
func TestDoRequest_InsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer srv.Close()

	c := New(fakeLookup{ep: inventory.ServiceEndpoint{URL: srv.URL, InsecureSkipVerify: true}}, &http.Client{})
	if _, err := c.ListAlerts(context.Background(), "main"); err != nil {
		t.Fatalf("unexpected error with InsecureSkipVerify set: %v", err)
	}

	// A second call for the same instance should reuse the cached
	// insecure client, not rebuild one every request.
	c.mu.Lock()
	n := len(c.insecureClients)
	c.mu.Unlock()
	if _, err := c.ListAlerts(context.Background(), "main"); err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.insecureClients) != n {
		t.Errorf("expected the insecure client cache to stay at %d entries, got %d", n, len(c.insecureClients))
	}
}
