package inventory

import (
	"errors"
	"testing"
)

const targetTestYAML = `
servers:
  archive:
    hostname: 10.0.0.5
    user: hermes
    key: ~/.ssh/archive

routers:
  old-core:
    hostname: 10.0.0.10
    vendor: cisco
    user: admin
    password: admin123
    protocol: telnet

  modern-core:
    hostname: 10.0.0.11
    vendor: cisco
    user: admin
    password: admin123

switches:
  ancient-sw:
    hostname: 10.0.0.20
    vendor: cisco
    user: admin
    password: admin123
    protocol: ssh
    legacy_crypto: true
    port: 22022
`

func TestTarget_Server(t *testing.T) {
	inv, err := Parse([]byte(targetTestYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	target, err := inv.Target("archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Protocol != ProtocolSSH {
		t.Errorf("Protocol = %v, want ssh", target.Protocol)
	}
	if target.LegacyCrypto {
		t.Error("servers should never default to LegacyCrypto")
	}
	if target.Key != "~/.ssh/archive" {
		t.Errorf("Key = %q, want ~/.ssh/archive", target.Key)
	}
	if target.Vendor != "" {
		t.Errorf("Vendor = %q, want empty for a server", target.Vendor)
	}
}

func TestTarget_TelnetRouter(t *testing.T) {
	inv, err := Parse([]byte(targetTestYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	target, err := inv.Target("old-core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Protocol != ProtocolTelnet {
		t.Errorf("Protocol = %v, want telnet", target.Protocol)
	}
	if target.Password != "admin123" {
		t.Errorf("Password = %q, want admin123", target.Password)
	}
	if target.Vendor != "cisco" {
		t.Errorf("Vendor = %q, want cisco", target.Vendor)
	}
}

func TestTarget_DefaultsToSSH(t *testing.T) {
	inv, err := Parse([]byte(targetTestYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	target, err := inv.Target("modern-core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Protocol != ProtocolSSH {
		t.Errorf("Protocol = %v, want ssh (default)", target.Protocol)
	}
}

func TestTarget_LegacyCryptoSwitch(t *testing.T) {
	inv, err := Parse([]byte(targetTestYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	target, err := inv.Target("ancient-sw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !target.LegacyCrypto {
		t.Error("expected LegacyCrypto=true")
	}
	if target.Port != 22022 {
		t.Errorf("Port = %d, want 22022", target.Port)
	}
}

func TestTarget_NotFound(t *testing.T) {
	inv, err := Parse([]byte(targetTestYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = inv.Target("does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTarget_Ambiguous(t *testing.T) {
	inv := &Inventory{
		Servers: map[string]Server{"dup": {Hostname: "10.0.0.1", User: "x"}},
		Routers: map[string]NetworkDevice{"dup": {Hostname: "10.0.0.2", Vendor: "cisco"}},
	}

	_, err := inv.Target("dup")
	if !errors.Is(err, ErrAmbiguousTarget) {
		t.Errorf("expected ErrAmbiguousTarget, got %v", err)
	}
}

func TestParse_InvalidProtocol(t *testing.T) {
	const bad = `
routers:
  core:
    hostname: 10.0.0.10
    vendor: cisco
    protocol: rlogin
`
	_, err := Parse([]byte(bad))
	if err == nil {
		t.Fatal("expected validation error for unsupported protocol")
	}
}
