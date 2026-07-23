// Package inventory loads and validates the static description of all
// infrastructure targets (servers, routers, switches, and platform APIs)
// that MCP tools are allowed to operate against. No tool implementation may
// hardcode a hostname, IP address, or credential — everything is resolved
// through this package.
package inventory

// Server describes a single SSH-reachable Linux/Unix host.
type Server struct {
	Hostname  string   `yaml:"hostname" validate:"required"`
	User      string   `yaml:"user" validate:"required"`
	Port      int      `yaml:"port"`
	Key       string   `yaml:"key"`
	Password  string   `yaml:"password"`
	ProxyJump string   `yaml:"proxyjump"`
	Tags      []string `yaml:"tags"`
}

// NetworkDevice describes a router or switch managed via a vendor-specific
// adapter (Cisco, MikroTik, ...).
type NetworkDevice struct {
	Hostname string   `yaml:"hostname" validate:"required"`
	Vendor   string   `yaml:"vendor" validate:"required"`
	User     string   `yaml:"user"`
	Password string   `yaml:"password"`
	Tags     []string `yaml:"tags"`
}

// ServiceEndpoint describes an HTTP(S) API-based integration such as
// Grafana or Proxmox.
type ServiceEndpoint struct {
	URL      string `yaml:"url" validate:"required,url"`
	Token    string `yaml:"token"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// Inventory is the root document loaded from the inventory YAML file.
type Inventory struct {
	Servers  map[string]Server        `yaml:"servers"`
	Routers  map[string]NetworkDevice `yaml:"routers"`
	Switches map[string]NetworkDevice `yaml:"switches"`
	Grafana  *ServiceEndpoint         `yaml:"grafana"`
	Proxmox  *ServiceEndpoint         `yaml:"proxmox"`
}
