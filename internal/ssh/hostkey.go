package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// buildHostKeyCallback returns a callback that verifies host keys against
// knownHostsPath. If insecure is true, it skips verification entirely
// (development/lab use only — never the default).
//
// Security is fail-closed: if knownHostsPath does not exist and insecure
// is false, every connection attempt is rejected with
// ErrNoHostKeyVerification rather than silently trusting unknown hosts.
func buildHostKeyCallback(knownHostsPath string, insecure bool) (ssh.HostKeyCallback, error) {
	if insecure {
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // explicit opt-in
	}

	if _, err := os.Stat(knownHostsPath); errors.Is(err, os.ErrNotExist) {
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return ErrNoHostKeyVerification
		}, nil
	}

	cb, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("ssh: load known_hosts %q: %w", knownHostsPath, err)
	}
	return cb, nil
}
