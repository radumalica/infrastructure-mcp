package inventory

import (
	"errors"
	"fmt"
)

// Protocol identifies the transport used to reach a Target.
type Protocol string

const (
	ProtocolSSH    Protocol = "ssh"
	ProtocolTelnet Protocol = "telnet"
)

// Target is a connection-level view of any inventory entry that a remote
// command can be run against: a Linux server, a router, or a switch.
// internal/ssh and internal/telnet operate on Target rather than on
// Server/NetworkDevice directly, so the transport layer doesn't need to
// know which inventory category a name came from.
type Target struct {
	Name      string
	Hostname  string
	Port      int
	User      string
	Password  string
	Key       string
	ProxyJump string

	Protocol     Protocol
	LegacyCrypto bool

	// Vendor is the NetworkDevice's vendor (e.g. "cisco", "mikrotik",
	// "unifi") for routers/switches, or "" for servers (which have no
	// vendor concept). Vendor-specific adapters use this to refuse to run
	// their commands against a device of the wrong kind.
	Vendor string
}

// ErrAmbiguousTarget is returned when a name exists in more than one
// inventory category (servers/routers/switches).
var ErrAmbiguousTarget = errors.New("inventory: ambiguous target name (exists in more than one category)")

// Target resolves name against servers, then routers, then switches, and
// returns a unified Target. Servers implicitly use Protocol ssh with
// LegacyCrypto disabled, matching their existing (unchanged) behavior.
func (inv *Inventory) Target(name string) (Target, error) {
	_, inServers := inv.Servers[name]
	_, inRouters := inv.Routers[name]
	_, inSwitches := inv.Switches[name]

	count := boolToInt(inServers) + boolToInt(inRouters) + boolToInt(inSwitches)
	if count > 1 {
		return Target{}, fmt.Errorf("%w: %q", ErrAmbiguousTarget, name)
	}

	switch {
	case inServers:
		s := inv.Servers[name]
		return Target{
			Name:      name,
			Hostname:  s.Hostname,
			Port:      s.Port,
			User:      s.User,
			Password:  s.Password,
			Key:       s.Key,
			ProxyJump: s.ProxyJump,
			Protocol:  ProtocolSSH,
		}, nil
	case inRouters:
		return networkDeviceTarget(name, inv.Routers[name]), nil
	case inSwitches:
		return networkDeviceTarget(name, inv.Switches[name]), nil
	default:
		return Target{}, fmt.Errorf("%w: target %q", ErrNotFound, name)
	}
}

func networkDeviceTarget(name string, d NetworkDevice) Target {
	protocol := ProtocolSSH
	if d.Protocol == string(ProtocolTelnet) {
		protocol = ProtocolTelnet
	}
	return Target{
		Name:         name,
		Hostname:     d.Hostname,
		Port:         d.Port,
		User:         d.User,
		Password:     d.Password,
		ProxyJump:    d.ProxyJump,
		Protocol:     protocol,
		LegacyCrypto: d.LegacyCrypto,
		Vendor:       d.Vendor,
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
