package ssh

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"infrastructure-mcp/internal/inventory"
)

func testInventoryFor(t *testing.T, srv *fakeSSHServer) *inventory.Inventory {
	t.Helper()
	host, portStr, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	inv := &inventory.Inventory{
		Servers: map[string]inventory.Server{
			"target": {
				Hostname: host,
				Port:     port,
				User:     srv.User,
				Password: srv.Password,
			},
		},
	}
	return inv
}

func TestPool_Run_Success(t *testing.T) {
	srv := startFakeSSHServer(t, map[string]commandOutcome{
		"uptime": {stdout: "up 3 days\n", exitCode: 0},
	})
	inv := testInventoryFor(t, srv)
	pool := NewPool(inv, WithInsecureIgnoreHostKey(), WithDialTimeout(2*time.Second))
	defer pool.Close()

	res, err := pool.Run(context.Background(), "target", "uptime")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if res.Stdout != "up 3 days\n" {
		t.Errorf("unexpected stdout: %q", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Errorf("unexpected exit code: %d", res.ExitCode)
	}
}

func TestPool_Run_NonZeroExit(t *testing.T) {
	srv := startFakeSSHServer(t, map[string]commandOutcome{
		"false": {exitCode: 1},
	})
	inv := testInventoryFor(t, srv)
	pool := NewPool(inv, WithInsecureIgnoreHostKey())
	defer pool.Close()

	res, err := pool.Run(context.Background(), "target", "false")
	if err != nil {
		t.Fatalf("Run should not return a Go error for a nonzero exit, got: %v", err)
	}
	if res.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", res.ExitCode)
	}
}

func TestPool_Run_ReusesConnection(t *testing.T) {
	srv := startFakeSSHServer(t, map[string]commandOutcome{
		"echo hi": {stdout: "hi\n"},
	})
	inv := testInventoryFor(t, srv)
	pool := NewPool(inv, WithInsecureIgnoreHostKey())
	defer pool.Close()
	ctx := context.Background()

	if _, err := pool.Run(ctx, "target", "echo hi"); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if _, err := pool.Run(ctx, "target", "echo hi"); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	pool.mu.Lock()
	n := len(pool.clients)
	pool.mu.Unlock()
	if n != 1 {
		t.Errorf("expected 1 pooled client after 2 runs to the same target, got %d", n)
	}
}

func TestPool_Run_UnknownServer(t *testing.T) {
	inv := &inventory.Inventory{Servers: map[string]inventory.Server{}}
	pool := NewPool(inv)
	defer pool.Close()

	_, err := pool.Run(context.Background(), "does-not-exist", "uptime")
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
}

func TestPool_Run_HostKeyRejectedWithoutKnownHosts(t *testing.T) {
	srv := startFakeSSHServer(t, map[string]commandOutcome{"uptime": {}})
	inv := testInventoryFor(t, srv)
	// No WithInsecureIgnoreHostKey and no known_hosts file at this path.
	pool := NewPool(inv, WithKnownHostsPath("/nonexistent/known_hosts"))
	defer pool.Close()

	_, err := pool.Run(context.Background(), "target", "uptime")
	if err == nil {
		t.Fatal("expected host key verification error")
	}
}

func TestPool_Run_ContextCancelled(t *testing.T) {
	srv := startFakeSSHServer(t, map[string]commandOutcome{"uptime": {}})
	inv := testInventoryFor(t, srv)
	pool := NewPool(inv, WithInsecureIgnoreHostKey())
	defer pool.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.Run(ctx, "target", "uptime")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestPool_Close_Idempotent(t *testing.T) {
	srv := startFakeSSHServer(t, map[string]commandOutcome{"uptime": {}})
	inv := testInventoryFor(t, srv)
	pool := NewPool(inv, WithInsecureIgnoreHostKey())

	if _, err := pool.Run(context.Background(), "target", "uptime"); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("second close should be a no-op, got: %v", err)
	}
}

func TestPool_Run_NetworkDeviceSwitch(t *testing.T) {
	srv := startFakeSSHServer(t, map[string]commandOutcome{"show version": {stdout: "IOS 15.2\n"}})
	host, portStr, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, _ := strconv.Atoi(portStr)

	inv := &inventory.Inventory{
		Switches: map[string]inventory.NetworkDevice{
			"old-switch": {
				Hostname:     host,
				Port:         port,
				Vendor:       "cisco",
				User:         srv.User,
				Password:     srv.Password,
				LegacyCrypto: true,
			},
		},
	}
	pool := NewPool(inv, WithInsecureIgnoreHostKey())
	defer pool.Close()

	res, err := pool.Run(context.Background(), "old-switch", "show version")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if res.Stdout != "IOS 15.2\n" {
		t.Errorf("unexpected stdout: %q", res.Stdout)
	}
}

func TestPool_Run_KeyboardInteractiveOnlyDevice(t *testing.T) {
	srv := startFakeSSHServerKeyboardInteractive(t, map[string]commandOutcome{
		"show version": {stdout: "IOS 12.4\n"},
	})
	host, portStr, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, _ := strconv.Atoi(portStr)

	inv := &inventory.Inventory{
		Routers: map[string]inventory.NetworkDevice{
			"old-router": {
				Hostname: host,
				Port:     port,
				Vendor:   "cisco",
				User:     srv.User,
				Password: srv.Password,
			},
		},
	}
	pool := NewPool(inv, WithInsecureIgnoreHostKey())
	defer pool.Close()

	res, err := pool.Run(context.Background(), "old-router", "show version")
	if err != nil {
		t.Fatalf("Run failed against a keyboard-interactive-only server: %v", err)
	}
	if res.Stdout != "IOS 12.4\n" {
		t.Errorf("unexpected stdout: %q", res.Stdout)
	}
}

func TestPool_Run_RefusesTelnetOnlyTarget(t *testing.T) {
	inv := &inventory.Inventory{
		Routers: map[string]inventory.NetworkDevice{
			"telnet-only": {
				Hostname: "10.0.0.1",
				Vendor:   "cisco",
				User:     "admin",
				Password: "admin",
				Protocol: "telnet",
			},
		},
	}
	pool := NewPool(inv)
	defer pool.Close()

	_, err := pool.Run(context.Background(), "telnet-only", "show version")
	if err == nil {
		t.Fatal("expected an error: ssh.Pool must refuse a telnet-only target rather than silently misdialing")
	}
}
