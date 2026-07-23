package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestBuildHostKeyCallback_Insecure(t *testing.T) {
	cb, err := buildHostKeyCallback("/nonexistent/known_hosts", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb == nil {
		t.Fatal("expected non-nil callback")
	}
}

func TestBuildHostKeyCallback_MissingFileFailsClosed(t *testing.T) {
	cb, err := buildHostKeyCallback("/nonexistent/known_hosts", false)
	if err != nil {
		t.Fatalf("unexpected error constructing callback: %v", err)
	}
	_, pub, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(pub)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	if err := cb("host:22", nil, signer.PublicKey()); !errors.Is(err, ErrNoHostKeyVerification) {
		t.Errorf("expected ErrNoHostKeyVerification, got %v", err)
	}
}

func TestBuildHostKeyCallback_ExistingKnownHosts(t *testing.T) {
	_, pub, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(pub)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	line := knownhosts.Line([]string{"example.lab:22"}, signer.PublicKey())
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}

	cb, err := buildHostKeyCallback(path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb == nil {
		t.Fatal("expected non-nil callback")
	}
}
