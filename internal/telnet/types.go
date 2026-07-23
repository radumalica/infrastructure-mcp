// Package telnet implements a minimal Telnet client for old network
// devices that predate SSH support entirely. It handles just enough of
// RFC 854 option negotiation (refusing every option offered, keeping the
// session in plain character mode) to drive a username/password login and
// run commands against typical embedded-device CLIs.
//
// Telnet has no protocol-level notion of exit status or a separate
// stderr stream (unlike SSH's exec channel), so Result.ExitCode is always
// 0 and Stderr is always empty — command success/failure has to be
// inferred from Stdout by the caller if needed.
package telnet

import (
	"errors"
	"time"
)

// Result is the outcome of running a single command over Telnet.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// ErrLoginFailed is returned when the login/password prompts don't
// resolve into a usable shell within the login timeout — most commonly
// because the credentials were rejected.
var ErrLoginFailed = errors.New("telnet: login failed or timed out")

// ErrNoCredentials is returned when a target has no password configured.
// Telnet devices in this codebase are password-only (see
// internal/inventory.NetworkDevice) — there is no key or agent fallback.
var ErrNoCredentials = errors.New("telnet: no password configured for target")
