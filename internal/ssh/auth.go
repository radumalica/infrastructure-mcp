package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"infrastructure-mcp/internal/inventory"
)

// buildAuthMethods derives the ssh.AuthMethod list for a server, in
// preference order: explicit private key, explicit password, then a
// running SSH agent as a fallback (covers proxyjump-only entries and
// operators who prefer agent-based auth over inventory-embedded secrets).
func buildAuthMethods(s inventory.Server) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if s.Key != "" {
		signer, err := loadPrivateKey(s.Key)
		if err != nil {
			return nil, fmt.Errorf("ssh: load key %q: %w", s.Key, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if s.Password != "" {
		methods = append(methods, ssh.Password(s.Password))
	}

	if agentMethod, ok := agentAuthMethod(); ok {
		methods = append(methods, agentMethod)
	}

	if len(methods) == 0 {
		return nil, ErrNoCredentials
	}

	return methods, nil
}

func loadPrivateKey(path string) (ssh.Signer, error) {
	expanded, err := expandHome(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}
	return signer, nil
}

// agentAuthMethod returns an ssh.AuthMethod backed by a running SSH agent,
// or ok=false if SSH_AUTH_SOCK is unset or unreachable.
func agentAuthMethod() (ssh.AuthMethod, bool) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, false
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, false
	}
	client := agent.NewClient(conn)
	return ssh.PublicKeysCallback(client.Signers), true
}

func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}
