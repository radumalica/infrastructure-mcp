package ssh

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"infrastructure-mcp/internal/inventory"
)

const defaultPort = 22

// PoolOption configures a Pool.
type PoolOption func(*Pool)

// WithKnownHostsPath overrides the default known_hosts location
// ($HOME/.ssh/known_hosts).
func WithKnownHostsPath(path string) PoolOption {
	return func(p *Pool) { p.knownHostsPath = path }
}

// WithInsecureIgnoreHostKey disables host key verification. Intended for
// lab/dev environments only — never enable this against production
// infrastructure.
func WithInsecureIgnoreHostKey() PoolOption {
	return func(p *Pool) { p.insecureHostKey = true }
}

// WithDialTimeout overrides the default per-connection dial timeout (10s).
func WithDialTimeout(d time.Duration) PoolOption {
	return func(p *Pool) { p.dialTimeout = d }
}

// Pool maintains reusable SSH connections to inventory servers, resolving
// proxyjump chains transparently. It is safe for concurrent use.
type Pool struct {
	inv *inventory.Inventory

	knownHostsPath  string
	insecureHostKey bool
	dialTimeout     time.Duration

	mu      sync.Mutex
	clients map[string]*ssh.Client
}

// NewPool creates a Pool that resolves targets against inv.
func NewPool(inv *inventory.Inventory, opts ...PoolOption) *Pool {
	home, _ := expandHome("~/.ssh/known_hosts")
	p := &Pool{
		inv:            inv,
		knownHostsPath: home,
		dialTimeout:    10 * time.Second,
		clients:        make(map[string]*ssh.Client),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Run executes command on the named server and waits for it to complete or
// ctx to be cancelled.
func (p *Pool) Run(ctx context.Context, serverName, command string) (Result, error) {
	client, err := p.client(ctx, serverName)
	if err != nil {
		return Result{}, err
	}

	session, err := client.NewSession()
	if err != nil {
		return Result{}, fmt.Errorf("ssh: new session on %q: %w", serverName, err)
	}
	defer func() { _ = session.Close() }()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	type runResult struct {
		err error
	}
	done := make(chan runResult, 1)
	start := time.Now()
	go func() {
		done <- runResult{err: session.Run(command)}
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return Result{}, ctx.Err()
	case r := <-done:
		duration := time.Since(start)
		exitCode := 0
		if r.err != nil {
			var exitErr *ssh.ExitError
			if asExitError(r.err, &exitErr) {
				exitCode = exitErr.ExitStatus()
			} else {
				return Result{}, fmt.Errorf("ssh: run %q on %q: %w", command, serverName, r.err)
			}
		}
		return Result{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
			Duration: duration,
		}, nil
	}
}

func asExitError(err error, target **ssh.ExitError) bool {
	if ee, ok := err.(*ssh.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

// client returns a cached connection to serverName, dialing (and, if
// necessary, chaining through a proxyjump host) on first use.
func (p *Pool) client(ctx context.Context, serverName string) (*ssh.Client, error) {
	p.mu.Lock()
	if c, ok := p.clients[serverName]; ok {
		p.mu.Unlock()
		return c, nil
	}
	p.mu.Unlock()

	c, err := p.dial(ctx, serverName, map[string]bool{})
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.clients[serverName] = c
	p.mu.Unlock()
	return c, nil
}

// dial connects to serverName, recursing through proxyjump if configured.
// visiting guards against proxyjump cycles in a misconfigured inventory.
func (p *Pool) dial(ctx context.Context, serverName string, visiting map[string]bool) (*ssh.Client, error) {
	if visiting[serverName] {
		return nil, fmt.Errorf("ssh: proxyjump cycle detected at %q", serverName)
	}
	visiting[serverName] = true

	target, err := p.inv.Target(serverName)
	if err != nil {
		return nil, err
	}
	if target.Protocol == inventory.ProtocolTelnet {
		return nil, fmt.Errorf("ssh: %q is configured for telnet, not ssh (use internal/remote to dispatch by protocol)", serverName)
	}

	config, err := p.clientConfig(target)
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(target.Hostname, portOrDefault(target.Port))

	if target.ProxyJump == "" {
		conn, err := net.DialTimeout("tcp", addr, p.dialTimeout)
		if err != nil {
			return nil, fmt.Errorf("ssh: dial %q (%s): %w", serverName, addr, err)
		}
		return newClientFromConn(conn, addr, config)
	}

	proxy, err := p.dial(ctx, target.ProxyJump, visiting)
	if err != nil {
		return nil, fmt.Errorf("ssh: proxyjump via %q: %w", target.ProxyJump, err)
	}
	conn, err := proxy.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh: dial %q (%s) via proxyjump %q: %w", serverName, addr, target.ProxyJump, err)
	}
	return newClientFromConn(conn, addr, config)
}

func newClientFromConn(conn net.Conn, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh: handshake with %s: %w", addr, err)
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}

func (p *Pool) clientConfig(t inventory.Target) (*ssh.ClientConfig, error) {
	auth, err := buildAuthMethods(t)
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := buildHostKeyCallback(p.knownHostsPath, p.insecureHostKey)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            t.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         p.dialTimeout,
	}
	if t.LegacyCrypto {
		applyLegacyCrypto(cfg)
	}
	return cfg, nil
}

func portOrDefault(port int) string {
	if port == 0 {
		port = defaultPort
	}
	return fmt.Sprintf("%d", port)
}

// Close closes every pooled connection.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for name, c := range p.clients {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("ssh: close %q: %w", name, err)
		}
		delete(p.clients, name)
	}
	return firstErr
}
