package inventory

import (
	"errors"
	"testing"
)

const validYAML = `
servers:
  archive:
    hostname: 10.0.0.5
    user: hermes
    key: ~/.ssh/archive
    tags: [linux, ethereum]
  pve01:
    hostname: 10.0.0.2
    user: hermes
    proxyjump: jumpbox
    tags: [proxmox]

routers:
  core:
    hostname: 10.0.0.10
    vendor: cisco

switches:
  sw01:
    hostname: 10.0.0.20
    vendor: cisco

grafana:
  url: https://grafana.lab.local
  token: ${TEST_GRAFANA_TOKEN}

proxmox:
  url: https://pve.lab.local:8006
  token: static-token
`

func TestParse_Valid(t *testing.T) {
	t.Setenv("TEST_GRAFANA_TOKEN", "secret-token")

	inv, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(inv.Servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(inv.Servers))
	}
	if inv.Servers["archive"].Hostname != "10.0.0.5" {
		t.Errorf("unexpected hostname: %s", inv.Servers["archive"].Hostname)
	}
	if inv.Grafana.Token != "secret-token" {
		t.Errorf("expected env var to be expanded, got %q", inv.Grafana.Token)
	}
	if inv.Routers["core"].Vendor != "cisco" {
		t.Errorf("unexpected router vendor: %s", inv.Routers["core"].Vendor)
	}
}

func TestParse_MissingEnvVar(t *testing.T) {
	_, err := Parse([]byte(validYAML))
	if err == nil {
		t.Fatal("expected error for missing env var, got nil")
	}
}

func TestParse_MissingRequiredField(t *testing.T) {
	const bad = `
servers:
  archive:
    user: hermes
    key: ~/.ssh/archive
`
	_, err := Parse([]byte(bad))
	if err == nil {
		t.Fatal("expected validation error for missing hostname, got nil")
	}
}

func TestParse_NoAuthMethod(t *testing.T) {
	// A server without an explicit key or password is valid: it falls back
	// to SSH agent authentication (or reaches the target only via a
	// proxyjump host that carries its own credentials).
	const noAuth = `
servers:
  archive:
    hostname: 10.0.0.5
    user: hermes
`
	inv, err := Parse([]byte(noAuth))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Servers["archive"].Key != "" {
		t.Errorf("expected empty key, got %q", inv.Servers["archive"].Key)
	}
}

func TestParse_UnknownField(t *testing.T) {
	const bad = `
servers:
  archive:
    hostname: 10.0.0.5
    user: hermes
    key: ~/.ssh/archive
    typo_field: oops
`
	_, err := Parse([]byte(bad))
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte("servers: [this is not a map"))
	if err == nil {
		t.Fatal("expected parse error for malformed yaml")
	}
}

func TestServer_Lookup(t *testing.T) {
	t.Setenv("TEST_GRAFANA_TOKEN", "secret-token")
	inv, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	s, err := inv.Server("archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.User != "hermes" {
		t.Errorf("unexpected user: %s", s.User)
	}

	_, err = inv.Server("does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestServerNames_FilterByTag(t *testing.T) {
	t.Setenv("TEST_GRAFANA_TOKEN", "secret-token")
	inv, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	all := inv.ServerNames("")
	if len(all) != 2 {
		t.Errorf("expected 2 servers, got %d", len(all))
	}

	tagged := inv.ServerNames("proxmox")
	if len(tagged) != 1 || tagged[0] != "pve01" {
		t.Errorf("expected [pve01], got %v", tagged)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/inventory.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
