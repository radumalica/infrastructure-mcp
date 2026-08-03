package proxmox

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
var ErrUnauthorized = errors.New("proxmox: unauthorized")

// ErrNoToken indicates the inventory entry for an instance has no API
// token configured.
var ErrNoToken = errors.New("proxmox: no API token configured")

// EndpointLookup resolves an inventory Proxmox cluster name to its
// connection details. *inventory.Inventory satisfies this; tests
// substitute a fake.
type EndpointLookup interface {
	ProxmoxEndpoint(name string) (inventory.ServiceEndpoint, error)
}

// Client calls the HTTP API of any Proxmox VE cluster in the inventory.
//
// Authentication is API-token only (Proxmox's "user@realm!tokenid=uuid"
// scheme, sent as an Authorization: PVEAPIToken header) — the
// ticket/cookie login flow is not implemented, matching the
// token-in-inventory shape used everywhere else in this codebase
// (see ServiceEndpoint and configs/inventory.example.yaml).
type Client struct {
	inv        EndpointLookup
	httpClient *http.Client

	mu              sync.Mutex
	insecureClients map[string]*http.Client
}

// New creates a Client that resolves cluster names against inv, using
// httpClient for requests. Pass nil to use http.DefaultClient with a
// 15s timeout.
func New(inv EndpointLookup, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{inv: inv, httpClient: httpClient, insecureClients: make(map[string]*http.Client)}
}

// httpClientFor returns the Client's normal shared http.Client, unless
// instance's inventory entry opts into InsecureSkipVerify — a stock
// Proxmox install's self-signed :8006 certificate being the motivating
// case, see the "Known limitation" note in this package's PROGRESS.md
// history — in which case it returns a dedicated client (built once,
// cached) with TLS certificate verification disabled for that one
// instance only.
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
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit, per-instance, documented lab/dev opt-in — see ServiceEndpoint.InsecureSkipVerify
	cl := &http.Client{Timeout: c.httpClient.Timeout, Transport: transport}
	c.insecureClients[instance] = cl
	return cl
}

func (c *Client) doRequest(ctx context.Context, instance, method, path string, query url.Values, form url.Values) ([]byte, error) {
	ep, err := c.inv.ProxmoxEndpoint(instance)
	if err != nil {
		return nil, err
	}
	if ep.Token == "" {
		return nil, fmt.Errorf("%w: instance %q (ticket/cookie auth is not supported)", ErrNoToken, instance)
	}

	fullURL := strings.TrimRight(ep.URL, "/") + "/api2/json" + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var reqBody io.Reader
	if form != nil {
		reqBody = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("proxmox: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "PVEAPIToken="+ep.Token)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClientFor(instance, ep).Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxmox: request to instance %q: %w", instance, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("proxmox: read response from instance %q: %w", instance, err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: instance %q returned %d", ErrUnauthorized, instance, resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%w: %s %s on instance %q", inventory.ErrNotFound, method, path, instance)
	case resp.StatusCode >= 300:
		return nil, fmt.Errorf("proxmox: instance %q returned %d: %s", instance, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ListNodes returns every node in the cluster along with its resource
// usage summary.
func (c *Client) ListNodes(ctx context.Context, instance string) ([]NodeEntry, error) {
	body, err := c.doRequest(ctx, instance, http.MethodGet, "/nodes", nil, nil)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Data []struct {
			Node   string  `json:"node"`
			Status string  `json:"status"`
			CPU    float64 `json:"cpu"`
			MaxCPU int     `json:"maxcpu"`
			Mem    int64   `json:"mem"`
			MaxMem int64   `json:"maxmem"`
			Uptime int64   `json:"uptime"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("proxmox: decode nodes response: %w", err)
	}

	nodes := make([]NodeEntry, 0, len(raw.Data))
	for _, n := range raw.Data {
		nodes = append(nodes, NodeEntry{
			Node:   n.Node,
			Status: n.Status,
			CPU:    n.CPU,
			MaxCPU: n.MaxCPU,
			Mem:    n.Mem,
			MaxMem: n.MaxMem,
			Uptime: n.Uptime,
		})
	}
	return nodes, nil
}

// ListVMs returns every QEMU VM and LXC container on node.
func (c *Client) ListVMs(ctx context.Context, instance, node string) ([]VMEntry, error) {
	var vms []VMEntry
	for _, guestType := range []string{"qemu", "lxc"} {
		body, err := c.doRequest(ctx, instance, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/"+guestType, nil, nil)
		if err != nil {
			return nil, err
		}

		var raw struct {
			Data []struct {
				VMID   int     `json:"vmid"`
				Name   string  `json:"name"`
				Status string  `json:"status"`
				CPU    float64 `json:"cpu"`
				MaxMem int64   `json:"maxmem"`
				Mem    int64   `json:"mem"`
				Uptime int64   `json:"uptime"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("proxmox: decode %s response: %w", guestType, err)
		}

		for _, v := range raw.Data {
			vms = append(vms, VMEntry{
				VMID:   v.VMID,
				Name:   v.Name,
				Type:   guestType,
				Status: v.Status,
				CPU:    v.CPU,
				MaxMem: v.MaxMem,
				Mem:    v.Mem,
				Uptime: v.Uptime,
			})
		}
	}
	return vms, nil
}

// ListTasks returns node's recorded tasks, most recent first, capped at
// limit (Proxmox's default/max page size applies if limit is 0).
func (c *Client) ListTasks(ctx context.Context, instance, node string, limit int) ([]TaskEntry, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}

	body, err := c.doRequest(ctx, instance, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/tasks", q, nil)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Data []struct {
			UPID      string `json:"upid"`
			Type      string `json:"type"`
			Status    string `json:"status"`
			User      string `json:"user"`
			StartTime int64  `json:"starttime"`
			EndTime   int64  `json:"endtime"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("proxmox: decode tasks response: %w", err)
	}

	tasks := make([]TaskEntry, 0, len(raw.Data))
	for _, t := range raw.Data {
		tasks = append(tasks, TaskEntry{
			UPID:      t.UPID,
			Type:      t.Type,
			Status:    t.Status,
			User:      t.User,
			StartTime: t.StartTime,
			EndTime:   t.EndTime,
		})
	}
	return tasks, nil
}

// guestPath builds the /nodes/{node}/{qemu|lxc}/{vmid} path segment shared
// by StartVM, StopVM, and Snapshot.
func guestPath(node, guestType string, vmid int) string {
	return "/nodes/" + url.PathEscape(node) + "/" + guestType + "/" + strconv.Itoa(vmid)
}

// decodeUPID unmarshals the {"data": "UPID:..."} response Proxmox returns
// from an async action endpoint into the task's UPID string.
func decodeUPID(body []byte) (string, error) {
	var raw struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("proxmox: decode task response: %w", err)
	}
	return raw.Data, nil
}

// StartVM requests that the given guest (guestType is "qemu" or "lxc")
// start. Like all Proxmox guest actions this is asynchronous: it returns
// the UPID of the task Proxmox created, not confirmation that the guest
// has actually started — follow up with ListTasks to check the outcome.
func (c *Client) StartVM(ctx context.Context, instance, node, guestType string, vmid int) (string, error) {
	if err := validateGuestType(guestType); err != nil {
		return "", err
	}
	body, err := c.doRequest(ctx, instance, http.MethodPost, guestPath(node, guestType, vmid)+"/status/start", nil, url.Values{})
	if err != nil {
		return "", err
	}
	return decodeUPID(body)
}

// StopVM requests a force-stop (equivalent to pulling power, not a
// graceful shutdown) of the given guest (guestType is "qemu" or "lxc").
// Asynchronous — see StartVM's doc comment.
func (c *Client) StopVM(ctx context.Context, instance, node, guestType string, vmid int) (string, error) {
	if err := validateGuestType(guestType); err != nil {
		return "", err
	}
	body, err := c.doRequest(ctx, instance, http.MethodPost, guestPath(node, guestType, vmid)+"/status/stop", nil, url.Values{})
	if err != nil {
		return "", err
	}
	return decodeUPID(body)
}

// Snapshot requests a new snapshot named snapname for the given guest
// (guestType is "qemu" or "lxc"). Asynchronous — see StartVM's doc
// comment.
func (c *Client) Snapshot(ctx context.Context, instance, node, guestType string, vmid int, snapname string) (string, error) {
	if err := validateGuestType(guestType); err != nil {
		return "", err
	}
	form := url.Values{"snapname": {snapname}}
	body, err := c.doRequest(ctx, instance, http.MethodPost, guestPath(node, guestType, vmid)+"/snapshot", nil, form)
	if err != nil {
		return "", err
	}
	return decodeUPID(body)
}
