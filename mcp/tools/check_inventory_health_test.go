package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newInventoryHealthSession(t *testing.T, inventoryPath string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterCheckInventoryHealth(server, testLogger(), inventoryPath)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go func() { _, _ = server.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callInventoryHealth(t *testing.T, session *mcp.ClientSession) CheckInventoryHealthOutput {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "check_inventory_health"})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var out CheckInventoryHealthOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func TestCheckInventoryHealth_Healthy(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_rsa")
	if err := os.WriteFile(keyPath, []byte("dummy key"), 0o600); err != nil {
		t.Fatal(err)
	}

	invPath := filepath.Join(dir, "inventory.yaml")
	invYAML := `
servers:
  archive:
    hostname: 10.0.0.5
    user: hermes
    key: ` + keyPath + `

grafana:
  main:
    url: https://grafana.lab.local
    token: hardcoded-fine-for-test
`
	if err := os.WriteFile(invPath, []byte(invYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	session := newInventoryHealthSession(t, invPath)
	out := callInventoryHealth(t, session)

	if out.Status != "healthy" {
		t.Errorf("Status = %q, want healthy; problems=%v", out.Status, out.Problems)
	}
	if out.Servers != 1 || out.Grafana != 1 {
		t.Errorf("unexpected counts: %+v", out)
	}
}

func TestCheckInventoryHealth_MissingEnvVar(t *testing.T) {
	dir := t.TempDir()
	invPath := filepath.Join(dir, "inventory.yaml")
	invYAML := `
grafana:
  main:
    url: https://grafana.lab.local
    token: ${DEFINITELY_UNSET_TEST_VAR}
`
	if err := os.WriteFile(invPath, []byte(invYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	session := newInventoryHealthSession(t, invPath)
	out := callInventoryHealth(t, session)

	if out.Status != "unhealthy" {
		t.Errorf("Status = %q, want unhealthy", out.Status)
	}
	if len(out.Problems) != 1 {
		t.Fatalf("Problems = %v, want 1 entry", out.Problems)
	}
}

func TestCheckInventoryHealth_UnreadableSSHKey(t *testing.T) {
	dir := t.TempDir()
	invPath := filepath.Join(dir, "inventory.yaml")
	invYAML := `
servers:
  archive:
    hostname: 10.0.0.5
    user: hermes
    key: ` + filepath.Join(dir, "does-not-exist") + `
`
	if err := os.WriteFile(invPath, []byte(invYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	session := newInventoryHealthSession(t, invPath)
	out := callInventoryHealth(t, session)

	if out.Status != "unhealthy" {
		t.Errorf("Status = %q, want unhealthy", out.Status)
	}
	if len(out.Problems) != 1 {
		t.Fatalf("Problems = %v, want 1 entry", out.Problems)
	}
}

func TestCheckInventoryHealth_UnreadableKubeconfig(t *testing.T) {
	dir := t.TempDir()
	invPath := filepath.Join(dir, "inventory.yaml")
	invYAML := `
kubernetes:
  home:
    kubeconfig: ` + filepath.Join(dir, "does-not-exist") + `
`
	if err := os.WriteFile(invPath, []byte(invYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	session := newInventoryHealthSession(t, invPath)
	out := callInventoryHealth(t, session)

	if out.Status != "unhealthy" {
		t.Errorf("Status = %q, want unhealthy", out.Status)
	}
	if len(out.Problems) != 1 {
		t.Fatalf("Problems = %v, want 1 entry", out.Problems)
	}
}

func TestCheckInventoryHealth_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	invPath := filepath.Join(dir, "inventory.yaml")
	if err := os.WriteFile(invPath, []byte("servers: [this is not a map"), 0o600); err != nil {
		t.Fatal(err)
	}

	session := newInventoryHealthSession(t, invPath)
	out := callInventoryHealth(t, session)

	if out.Status != "unhealthy" || len(out.Problems) != 1 {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestCheckInventoryHealth_MissingFile(t *testing.T) {
	session := newInventoryHealthSession(t, "/nonexistent/inventory.yaml")
	out := callInventoryHealth(t, session)

	if out.Status != "unhealthy" || len(out.Problems) != 1 {
		t.Errorf("unexpected output: %+v", out)
	}
}
