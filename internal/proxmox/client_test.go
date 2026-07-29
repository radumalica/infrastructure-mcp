package proxmox

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

func (f fakeLookup) ProxmoxEndpoint(name string) (inventory.ServiceEndpoint, error) {
	return f.ep, f.err
}

func testClient(t *testing.T, srv *httptest.Server, token string) *Client {
	t.Helper()
	return New(fakeLookup{ep: inventory.ServiceEndpoint{URL: srv.URL, Token: token}}, srv.Client())
}

func TestListNodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "PVEAPIToken=secret" {
			t.Errorf("expected PVEAPIToken header, got %q", got)
		}
		if r.URL.Path != "/api2/json/nodes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"node": "pve01", "status": "online", "cpu": 0.05, "maxcpu": 8, "mem": 1024, "maxmem": 8192, "uptime": 3600},
			},
		})
	}))
	defer srv.Close()

	nodes, err := testClient(t, srv, "secret").ListNodes(context.Background(), "lab")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Node != "pve01" || nodes[0].Status != "online" {
		t.Errorf("unexpected nodes: %+v", nodes)
	}
}

func TestListVMs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/pve01/qemu":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"vmid": 100, "name": "web01", "status": "running", "cpu": 0.1, "maxmem": 2048, "mem": 1024, "uptime": 100},
				},
			})
		case "/api2/json/nodes/pve01/lxc":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"vmid": 200, "name": "ct01", "status": "stopped", "cpu": 0, "maxmem": 512, "mem": 0, "uptime": 0},
				},
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	vms, err := testClient(t, srv, "secret").ListVMs(context.Background(), "lab", "pve01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vms) != 2 {
		t.Fatalf("expected 2 guests, got %d: %+v", len(vms), vms)
	}
	if vms[0].Type != "qemu" || vms[0].VMID != 100 {
		t.Errorf("unexpected qemu entry: %+v", vms[0])
	}
	if vms[1].Type != "lxc" || vms[1].VMID != 200 {
		t.Errorf("unexpected lxc entry: %+v", vms[1])
	}
}

func TestListTasks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve01/tasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %q", r.URL.Query().Get("limit"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"upid": "UPID:pve01:...", "type": "vzstart", "status": "OK", "user": "root@pam", "starttime": 1000, "endtime": 1005},
			},
		})
	}))
	defer srv.Close()

	tasks, err := testClient(t, srv, "secret").ListTasks(context.Background(), "lab", "pve01", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Status != "OK" {
		t.Errorf("unexpected tasks: %+v", tasks)
	}
}

func TestStartStopSnapshot(t *testing.T) {
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/api2/json/nodes/pve01/qemu/100/snapshot" {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			if r.PostForm.Get("snapname") != "before-upgrade" {
				t.Errorf("expected snapname=before-upgrade, got %q", r.PostForm.Get("snapname"))
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve01:..."})
	}))
	defer srv.Close()

	c := testClient(t, srv, "secret")
	if upid, err := c.StartVM(context.Background(), "lab", "pve01", "qemu", 100); err != nil {
		t.Fatalf("StartVM: %v", err)
	} else if upid != "UPID:pve01:..." {
		t.Errorf("StartVM upid = %q", upid)
	}
	if upid, err := c.StopVM(context.Background(), "lab", "pve01", "qemu", 100); err != nil {
		t.Fatalf("StopVM: %v", err)
	} else if upid != "UPID:pve01:..." {
		t.Errorf("StopVM upid = %q", upid)
	}
	if upid, err := c.Snapshot(context.Background(), "lab", "pve01", "qemu", 100, "before-upgrade"); err != nil {
		t.Fatalf("Snapshot: %v", err)
	} else if upid != "UPID:pve01:..." {
		t.Errorf("Snapshot upid = %q", upid)
	}

	want := []string{
		"POST /api2/json/nodes/pve01/qemu/100/status/start",
		"POST /api2/json/nodes/pve01/qemu/100/status/stop",
		"POST /api2/json/nodes/pve01/qemu/100/snapshot",
	}
	if len(gotPaths) != len(want) {
		t.Fatalf("got paths %v, want %v", gotPaths, want)
	}
	for i := range want {
		if gotPaths[i] != want[i] {
			t.Errorf("path[%d] = %q, want %q", i, gotPaths[i], want[i])
		}
	}
}

func TestDoRequest_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := testClient(t, srv, "bad-token").ListNodes(context.Background(), "lab")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDoRequest_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := testClient(t, srv, "secret").ListNodes(context.Background(), "lab")
	if !errors.Is(err, inventory.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDoRequest_InstanceNotFound(t *testing.T) {
	c := New(fakeLookup{err: inventory.ErrNotFound}, http.DefaultClient)
	_, err := c.ListNodes(context.Background(), "does-not-exist")
	if !errors.Is(err, inventory.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDoRequest_NoToken(t *testing.T) {
	c := New(fakeLookup{ep: inventory.ServiceEndpoint{URL: "https://pve.lab.local:8006"}}, http.DefaultClient)
	_, err := c.ListNodes(context.Background(), "lab")
	if !errors.Is(err, ErrNoToken) {
		t.Errorf("expected ErrNoToken, got %v", err)
	}
}
