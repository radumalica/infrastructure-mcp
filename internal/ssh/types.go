// Package ssh provides pooled SSH connectivity to inventory servers. It is
// the only package in this codebase allowed to dial infrastructure over
// SSH; everything above it (internal/linux, mcp/tools, ...) goes through
// Pool.
package ssh

import (
	"errors"
	"time"
)

// Result is the outcome of running a single remote command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// ErrNoCredentials is returned when a server has no usable authentication
// method: no key, no password, and no reachable SSH agent.
var ErrNoCredentials = errors.New("ssh: no usable credentials for target")

// ErrNoHostKeyVerification is returned when a target's host key cannot be
// verified because no known_hosts entry exists for it and insecure mode
// was not explicitly enabled.
var ErrNoHostKeyVerification = errors.New("ssh: host key could not be verified (add it to known_hosts or enable insecure mode)")
