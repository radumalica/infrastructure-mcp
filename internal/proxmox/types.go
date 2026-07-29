// Package proxmox talks to a Proxmox VE cluster's HTTP API — node status,
// VM/container listing, task history, and start/stop/snapshot actions.
// All authentication (an API token in the form "user@realm!tokenid=uuid")
// lives inside the referenced inventory.ServiceEndpoint and is never
// exposed to callers.
package proxmox

// NodeEntry describes one cluster node from GET /nodes.
type NodeEntry struct {
	Node   string  `json:"node"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	MaxCPU int     `json:"max_cpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"max_mem"`
	Uptime int64   `json:"uptime"`
}

// VMEntry describes one guest (VM or container) from
// GET /nodes/{node}/qemu or GET /nodes/{node}/lxc.
type VMEntry struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Type   string  `json:"type"` // "qemu" or "lxc"
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	MaxMem int64   `json:"max_mem"`
	Mem    int64   `json:"mem"`
	Uptime int64   `json:"uptime"`
}

// TaskEntry describes one recorded task from GET /nodes/{node}/tasks.
type TaskEntry struct {
	UPID      string `json:"upid"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	User      string `json:"user"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time,omitempty"`
}
