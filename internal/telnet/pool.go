package telnet

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"infrastructure-mcp/internal/inventory"
)

const defaultPort = 23

// PoolOption configures a Pool.
type PoolOption func(*Pool)

// WithDialTimeout overrides the default TCP dial timeout (10s).
func WithDialTimeout(d time.Duration) PoolOption {
	return func(p *Pool) { p.dialTimeout = d }
}

// WithLoginTimeout overrides how long login (waiting for the
// login/password prompts and post-login output) may take (15s default).
func WithLoginTimeout(d time.Duration) PoolOption {
	return func(p *Pool) { p.loginTimeout = d }
}

// WithIdleTimeout overrides how long the client waits for new output
// before deciding a prompt has settled and a command's output is
// complete (2s default). Telnet has no protocol-level "command done"
// signal, so this idle heuristic is what determines when Run returns.
func WithIdleTimeout(d time.Duration) PoolOption {
	return func(p *Pool) { p.idleTimeout = d }
}

// WithCommandTimeout overrides the maximum time a single command may run
// before its output is returned regardless of idle state (30s default).
func WithCommandTimeout(d time.Duration) PoolOption {
	return func(p *Pool) { p.commandTimeout = d }
}

// Pool maintains one Telnet session per inventory target, reused across
// Run calls. Telnet sessions are a single raw stream, so each session
// serializes its own commands (unlike ssh.Pool, which can multiplex
// concurrent SSH channels over one connection).
type Pool struct {
	inv *inventory.Inventory

	dialTimeout    time.Duration
	loginTimeout   time.Duration
	idleTimeout    time.Duration
	commandTimeout time.Duration

	mu       sync.Mutex
	sessions map[string]*pooledSession
}

type pooledSession struct {
	mu sync.Mutex
	s  *session
}

// NewPool creates a Pool that resolves targets against inv.
func NewPool(inv *inventory.Inventory, opts ...PoolOption) *Pool {
	p := &Pool{
		inv:            inv,
		dialTimeout:    10 * time.Second,
		loginTimeout:   15 * time.Second,
		idleTimeout:    2 * time.Second,
		commandTimeout: 30 * time.Second,
		sessions:       make(map[string]*pooledSession),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Run executes command on the named target over Telnet, logging in first
// if there is no cached session yet.
func (p *Pool) Run(ctx context.Context, targetName, command string) (Result, error) {
	ps, err := p.session(ctx, targetName)
	if err != nil {
		return Result{}, err
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	return ps.s.run(command, p.idleTimeout, p.commandTimeout)
}

func (p *Pool) session(ctx context.Context, targetName string) (*pooledSession, error) {
	p.mu.Lock()
	if ps, ok := p.sessions[targetName]; ok {
		p.mu.Unlock()
		return ps, nil
	}
	p.mu.Unlock()

	target, err := p.inv.Target(targetName)
	if err != nil {
		return nil, err
	}
	if target.Protocol != inventory.ProtocolTelnet {
		return nil, fmt.Errorf("telnet: %q is not configured for telnet (use internal/remote to dispatch by protocol)", targetName)
	}

	addr := net.JoinHostPort(target.Hostname, portOrDefault(target.Port))
	s, err := dialAndLogin(ctx, addr, target.User, target.Password, p.dialTimeout, p.loginTimeout, p.idleTimeout)
	if err != nil {
		return nil, fmt.Errorf("telnet: login to %q (%s): %w", targetName, addr, err)
	}

	ps := &pooledSession{s: s}
	p.mu.Lock()
	p.sessions[targetName] = ps
	p.mu.Unlock()
	return ps, nil
}

func portOrDefault(port int) string {
	if port == 0 {
		port = defaultPort
	}
	return fmt.Sprintf("%d", port)
}

// Close closes every pooled session.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for name, ps := range p.sessions {
		if err := ps.s.close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("telnet: close %q: %w", name, err)
		}
		delete(p.sessions, name)
	}
	return firstErr
}
