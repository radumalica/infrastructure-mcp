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
// adapter (Cisco, MikroTik, ...). Many such devices are old: some speak
// SSH only with obsolete key exchange/cipher/MAC algorithms, some have no
// SSH server at all and are reachable only via Telnet, and essentially
// none support public-key auth — user/password is the norm.
type NetworkDevice struct {
	Hostname string   `yaml:"hostname" validate:"required"`
	Vendor   string   `yaml:"vendor" validate:"required"`
	User     string   `yaml:"user"`
	Password string   `yaml:"password"`
	Tags     []string `yaml:"tags"`

	// Port overrides the default port for Protocol (22 for ssh, 23 for
	// telnet).
	Port int `yaml:"port"`

	// ProxyJump, if set, is the inventory name of a server this device is
	// only reachable through (SSH connections only; Telnet has no
	// equivalent of an SSH jump host in this codebase).
	ProxyJump string `yaml:"proxyjump"`

	// Protocol selects the transport: "ssh" (default) or "telnet".
	Protocol string `yaml:"protocol" validate:"omitempty,oneof=ssh telnet"`

	// LegacyCrypto opts this device into obsolete SSH key exchange,
	// cipher, MAC, and host-key algorithms (golang.org/x/crypto/ssh's
	// InsecureAlgorithms) so old switch/router SSH stacks can still be
	// reached. Only takes effect when Protocol is "ssh" (or empty).
	// Never enabled by default — it must be opted into per device.
	LegacyCrypto bool `yaml:"legacy_crypto"`
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
