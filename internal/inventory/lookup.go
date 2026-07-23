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
