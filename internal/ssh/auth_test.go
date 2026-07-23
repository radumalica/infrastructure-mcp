package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"

	"infrastructure-mcp/internal/inventory"
)

func writeTestPrivateKey(t *testing.T, dir string) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	path := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return path
}

func TestBuildAuthMethods_KeyOnly(t *testing.T) {
	keyPath := writeTestPrivateKey(t, t.TempDir())
	target := inventory.Target{Hostname: "10.0.0.1", User: "hermes", Key: keyPath}
	methods, err := buildAuthMethods(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) < 1 {
		t.Fatalf("expected at least 1 auth method, got %d", len(methods))
	}
}

func TestBuildAuthMethods_PasswordOnly(t *testing.T) {
	target := inventory.Target{Hostname: "10.0.0.1", User: "hermes", Password: "secret"}
	methods, err := buildAuthMethods(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) < 1 {
		t.Fatalf("expected at least 1 auth method, got %d", len(methods))
	}
}

func TestBuildAuthMethods_MissingKeyFile(t *testing.T) {
	target := inventory.Target{Hostname: "10.0.0.1", User: "hermes", Key: "/nonexistent/key"}
	_, err := buildAuthMethods(target)
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
}

func TestBuildAuthMethods_NoCredentialsNoAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	target := inventory.Target{Hostname: "10.0.0.1", User: "hermes"}
	_, err := buildAuthMethods(target)
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials, got %v", err)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}
	got, err := expandHome("~/.ssh/id_rsa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, ".ssh/id_rsa")
	if got != want {
		t.Errorf("expandHome() = %q, want %q", got, want)
	}

	// Non-tilde paths pass through unchanged.
	got, err = expandHome("/absolute/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/absolute/path" {
		t.Errorf("expandHome() = %q, want unchanged path", got)
	}
}
