package telnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// session is a live, authenticated Telnet connection to one device.
// Telnet is a single raw character stream — unlike SSH there is no
// concept of concurrent channels — so commands on a session must be
// serialized by the caller (Pool does this with a per-session mutex).
type session struct {
	conn   net.Conn
	filter *iacFilter
}

var loginPrompts = []string{"login:", "username:"}
var passwordPrompts = []string{"password:"}

func dialAndLogin(ctx context.Context, addr, user, password string, dialTimeout, loginTimeout, idleTimeout time.Duration) (*session, error) {
	if password == "" {
		return nil, ErrNoCredentials
	}

	dialer := net.Dialer{Timeout: dialTimeout}
	nc, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("telnet: dial %s: %w", addr, err)
	}

	s := &session{conn: nc, filter: &iacFilter{conn: nc}}

	deadline := time.Now().Add(loginTimeout)

	if _, err := s.readUntilAny(loginPrompts, idleTimeout, time.Until(deadline)); err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("telnet: waiting for login prompt: %w", err)
	}
	if err := s.writeLine(user); err != nil {
		_ = nc.Close()
		return nil, err
	}

	if _, err := s.readUntilAny(passwordPrompts, idleTimeout, time.Until(deadline)); err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("telnet: waiting for password prompt: %w", err)
	}
	if err := s.writeLine(password); err != nil {
		_ = nc.Close()
		return nil, err
	}

	// Drain whatever the device sends after auth (MOTD, first prompt) so
	// it doesn't leak into the first command's output. A login failure
	// here (bad credentials) typically re-shows a login/password prompt,
	// which we detect and report as ErrLoginFailed.
	post, err := s.readUntilIdle(idleTimeout, time.Until(deadline))
	if err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("telnet: waiting for post-login output: %w", err)
	}
	lower := strings.ToLower(post)
	for _, p := range loginPrompts {
		if strings.Contains(lower, p) {
			_ = nc.Close()
			return nil, ErrLoginFailed
		}
	}

	return s, nil
}

// run sends command and collects output until idleTimeout of silence or
// overallTimeout elapses.
func (s *session) run(command string, idleTimeout, overallTimeout time.Duration) (Result, error) {
	start := time.Now()
	if err := s.writeLine(command); err != nil {
		return Result{}, err
	}

	out, err := s.readUntilIdle(idleTimeout, overallTimeout)
	if err != nil {
		return Result{}, err
	}

	// Strip the echoed command from the start of the output, if present
	// (most devices echo what was typed before the response).
	out = strings.TrimPrefix(out, command)
	out = strings.TrimPrefix(out, "\r\n")
	out = strings.TrimPrefix(out, "\n")

	return Result{Stdout: out, Duration: time.Since(start)}, nil
}

func (s *session) writeLine(line string) error {
	_, err := s.conn.Write([]byte(line + "\r\n"))
	if err != nil {
		return fmt.Errorf("telnet: write: %w", err)
	}
	return nil
}

func (s *session) close() error {
	return s.conn.Close()
}

// readUntilAny blocks until the accumulated output contains one of
// markers (case-insensitive) or overallTimeout elapses.
func (s *session) readUntilAny(markers []string, idleTimeout, overallTimeout time.Duration) (string, error) {
	deadline := time.Now().Add(overallTimeout)
	var acc strings.Builder

	for {
		if containsAny(strings.ToLower(acc.String()), markers) {
			return acc.String(), nil
		}
		if time.Now().After(deadline) {
			return acc.String(), fmt.Errorf("timed out waiting for %v", markers)
		}

		chunk, err := s.readChunk(idleTimeout, deadline)
		if err != nil {
			return acc.String(), err
		}
		acc.Write(chunk)
	}
}

// readUntilIdle accumulates output until idleTimeout passes with no new
// data, or overallTimeout elapses — used once a shell prompt has been
// reached and there is no fixed marker to wait for.
func (s *session) readUntilIdle(idleTimeout, overallTimeout time.Duration) (string, error) {
	deadline := time.Now().Add(overallTimeout)
	var acc strings.Builder

	for {
		chunk, err := s.readChunk(idleTimeout, deadline)
		if err != nil {
			if errors.Is(err, errIdle) {
				return acc.String(), nil
			}
			return acc.String(), err
		}
		acc.Write(chunk)
		if time.Now().After(deadline) {
			return acc.String(), nil
		}
	}
}

var errIdle = errors.New("telnet: idle timeout")

// readChunk reads one batch of bytes, applying the IAC filter, and
// returns it. It returns errIdle if idleTimeout elapses with no data,
// distinct from a hard failure so callers can treat idling as "done"
// rather than an error.
func (s *session) readChunk(idleTimeout time.Duration, hardDeadline time.Time) ([]byte, error) {
	readDeadline := time.Now().Add(idleTimeout)
	if hardDeadline.Before(readDeadline) {
		readDeadline = hardDeadline
	}
	if err := s.conn.SetReadDeadline(readDeadline); err != nil {
		return nil, fmt.Errorf("telnet: set read deadline: %w", err)
	}

	buf := make([]byte, 4096)
	n, err := s.conn.Read(buf)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, errIdle
		}
		return nil, fmt.Errorf("telnet: read: %w", err)
	}
	return s.filter.feed(buf[:n]), nil
}

func containsAny(haystack string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(haystack, m) {
			return true
		}
	}
	return false
}
