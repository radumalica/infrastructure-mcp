package inventory

import (
	"fmt"
	"sort"
)

// Server returns the named server, or ErrNotFound if it does not exist.
func (inv *Inventory) Server(name string) (Server, error) {
	s, ok := inv.Servers[name]
	if !ok {
		return Server{}, fmt.Errorf("%w: server %q", ErrNotFound, name)
	}
	return s, nil
}

// KubeCluster returns the named Kubernetes cluster, or ErrNotFound if it
// does not exist.
func (inv *Inventory) KubeCluster(name string) (KubeCluster, error) {
	k, ok := inv.Kubernetes[name]
	if !ok {
		return KubeCluster{}, fmt.Errorf("%w: kubernetes cluster %q", ErrNotFound, name)
	}
	return k, nil
}

// GrafanaEndpoint returns the named Grafana instance, or ErrNotFound if
// it does not exist. (Named GrafanaEndpoint, not Grafana, because
// Inventory already has a Grafana field — Go forbids a method and a
// field sharing a name on the same type.)
func (inv *Inventory) GrafanaEndpoint(name string) (ServiceEndpoint, error) {
	g, ok := inv.Grafana[name]
	if !ok {
		return ServiceEndpoint{}, fmt.Errorf("%w: grafana instance %q", ErrNotFound, name)
	}
	return g, nil
}

// ProxmoxEndpoint returns the named Proxmox cluster, or ErrNotFound if it
// does not exist.
func (inv *Inventory) ProxmoxEndpoint(name string) (ServiceEndpoint, error) {
	p, ok := inv.Proxmox[name]
	if !ok {
		return ServiceEndpoint{}, fmt.Errorf("%w: proxmox instance %q", ErrNotFound, name)
	}
	return p, nil
}

// ServerNames returns the names of all servers, optionally filtered to
// those carrying the given tag. An empty tag returns every server name.
// Results are sorted for deterministic tool output.
func (inv *Inventory) ServerNames(tag string) []string {
	names := make([]string, 0, len(inv.Servers))
	for name, s := range inv.Servers {
		if tag == "" || hasTag(s.Tags, tag) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
