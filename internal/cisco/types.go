// Package cisco runs read-only diagnostic commands against Cisco IOS
// routers/switches over the shared SSH/Telnet remote layer
// (internal/remote), and parses their CLI output. Unlike internal/grafana
// or internal/proxmox, there is no JSON API here: Cisco IOS has no formal
// machine-readable output format, so parsing targets the long-stable
// plaintext conventions used by classic IOS (and largely unchanged in
// IOS-XE) — the same conventions every major network automation tool
// (Netmiko, NAPALM, RANCID) has relied on for decades. See PROGRESS.md's
// v0.7 entry for what has and hasn't been verified against a live device.
package cisco

// VersionInfo is the parsed result of "show version".
type VersionInfo struct {
	Hostname    string `json:"hostname,omitempty"`
	VersionLine string `json:"version_line"`
	Uptime      string `json:"uptime,omitempty"`
}

// InterfaceEntry is one row of "show ip interface brief".
type InterfaceEntry struct {
	Interface string `json:"interface"`
	IPAddress string `json:"ip_address"`
	OK        string `json:"ok"`
	Method    string `json:"method"`
	Status    string `json:"status"`
	Protocol  string `json:"protocol"`
}

// InventoryEntry is one NAME/DESCR/PID/VID/SN block of "show inventory".
type InventoryEntry struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	PID          string `json:"pid,omitempty"`
	VID          string `json:"vid,omitempty"`
	SerialNumber string `json:"serial_number,omitempty"`
}
