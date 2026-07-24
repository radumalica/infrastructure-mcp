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
  main:
    url: https://grafana.lab.local
    token: ${TEST_GRAFANA_TOKEN}
  staging:
    url: https://grafana-staging.lab.local
    token: static-token

proxmox:
  lab:
    url: https://pve.lab.local:8006
    token: static-token

kubernetes:
  home:
    kubeconfig: /etc/infrastructure-mcp/kubeconfig-home
    context: home-admin
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
	if len(inv.Grafana) != 2 {
		t.Errorf("expected 2 grafana instances, got %d", len(inv.Grafana))
	}
	if inv.Grafana["main"].Token != "secret-token" {
		t.Errorf("expected env var to be expanded, got %q", inv.Grafana["main"].Token)
	}
	if inv.Grafana["staging"].URL != "https://grafana-staging.lab.local" {
		t.Errorf("unexpected staging grafana url: %s", inv.Grafana["staging"].URL)
	}
	if len(inv.Proxmox) != 1 || inv.Proxmox["lab"].URL != "https://pve.lab.local:8006" {
		t.Errorf("unexpected proxmox entries: %+v", inv.Proxmox)
	}
	if inv.Routers["core"].Vendor != "cisco" {
		t.Errorf("unexpected router vendor: %s", inv.Routers["core"].Vendor)
	}
	if len(inv.Kubernetes) != 1 || inv.Kubernetes["home"].Context != "home-admin" {
		t.Errorf("unexpected kubernetes entries: %+v", inv.Kubernetes)
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

func TestKubeCluster_Lookup(t *testing.T) {
	t.Setenv("TEST_GRAFANA_TOKEN", "secret-token")
	inv, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	k, err := inv.KubeCluster("home")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k.Kubeconfig != "/etc/infrastructure-mcp/kubeconfig-home" {
		t.Errorf("unexpected kubeconfig path: %s", k.Kubeconfig)
	}

	_, err = inv.KubeCluster("does-not-exist")
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
