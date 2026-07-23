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

// buildAuthMethods derives the ssh.AuthMethod list for a target, in
// preference order: explicit private key, explicit password, keyboard-
// interactive using the same password, then a running SSH agent as a
// fallback (covers proxyjump-only entries and operators who prefer
// agent-based auth over inventory-embedded secrets).
// Old network devices typically have no key at all and rely solely on
// password auth here — and many of them (Cisco IOS and similar) only
// offer the "keyboard-interactive" SSH auth method rather than "password",
// so both are registered whenever a password is configured.
func buildAuthMethods(t inventory.Target) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if t.Key != "" {
		signer, err := loadPrivateKey(t.Key)
		if err != nil {
			return nil, fmt.Errorf("ssh: load key %q: %w", t.Key, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if t.Password != "" {
		methods = append(methods, ssh.Password(t.Password))
		methods = append(methods, ssh.KeyboardInteractive(passwordKeyboardInteractive(t.Password)))
	}

	if agentMethod, ok := agentAuthMethod(); ok {
		methods = append(methods, agentMethod)
	}

	if len(methods) == 0 {
		return nil, ErrNoCredentials
	}

	return methods, nil
}

// passwordKeyboardInteractive answers every keyboard-interactive prompt
// with password, regardless of the prompt text. This covers devices that
// present a single "Password:" challenge over keyboard-interactive instead
// of the SSH "password" method — answering by count rather than by
// matching prompt wording keeps it working across vendors' varied prompts.
func passwordKeyboardInteractive(password string) ssh.KeyboardInteractiveChallenge {
	return func(_, _ string, questions []string, _ []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range answers {
			answers[i] = password
		}
		return answers, nil
	}
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
