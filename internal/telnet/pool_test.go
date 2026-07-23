package telnet

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"infrastructure-mcp/internal/inventory"
)

func testInventoryFor(t *testing.T, srv *fakeTelnetServer) *inventory.Inventory {
	t.Helper()
	host, portStr, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	return &inventory.Inventory{
		Routers: map[string]inventory.NetworkDevice{
			"old-router": {
				Hostname: host,
				Port:     port,
				Vendor:   "cisco",
				User:     srv.User,
				Password: srv.Password,
				Protocol: "telnet",
			},
		},
	}
}

func newTestPool(inv *inventory.Inventory) *Pool {
	return NewPool(inv,
		WithDialTimeout(2*time.Second),
		WithLoginTimeout(3*time.Second),
		WithIdleTimeout(200*time.Millisecond),
		WithCommandTimeout(3*time.Second),
	)
}

func TestPool_Run_Success(t *testing.T) {
	srv := startFakeTelnetServer(t, map[string]string{
		"show version": "IOS 12.4",
	})
	inv := testInventoryFor(t, srv)
	pool := newTestPool(inv)
	defer pool.Close()

	res, err := pool.Run(context.Background(), "old-router", "show version")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(res.Stdout, "IOS 12.4") {
		t.Errorf("expected output to contain command response, got %q", res.Stdout)
	}
}

func TestPool_Run_ReusesSession(t *testing.T) {
	srv := startFakeTelnetServer(t, map[string]string{"show version": "IOS 12.4"})
	inv := testInventoryFor(t, srv)
	pool := newTestPool(inv)
	defer pool.Close()
	ctx := context.Background()

	if _, err := pool.Run(ctx, "old-router", "show version"); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if _, err := pool.Run(ctx, "old-router", "show version"); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	pool.mu.Lock()
	n := len(pool.sessions)
	pool.mu.Unlock()
	if n != 1 {
		t.Errorf("expected 1 pooled session after 2 runs, got %d", n)
	}
}

func TestPool_Run_BadCredentials(t *testing.T) {
	srv := startFakeTelnetServer(t, map[string]string{})
	host, portStr, _ := net.SplitHostPort(srv.Addr)
	port, _ := strconv.Atoi(portStr)

	inv := &inventory.Inventory{
		Routers: map[string]inventory.NetworkDevice{
			"old-router": {
				Hostname: host,
				Port:     port,
				Vendor:   "cisco",
				User:     "admin",
				Password: "wrong-password",
				Protocol: "telnet",
			},
		},
	}
	pool := newTestPool(inv)
	defer pool.Close()

	_, err := pool.Run(context.Background(), "old-router", "show version")
	if err == nil {
		t.Fatal("expected login failure for bad credentials")
	}
}

func TestPool_Run_NoPassword(t *testing.T) {
	inv := &inventory.Inventory{
		Routers: map[string]inventory.NetworkDevice{
			"old-router": {Hostname: "10.0.0.1", Vendor: "cisco", User: "admin", Protocol: "telnet"},
		},
	}
	pool := newTestPool(inv)
	defer pool.Close()

	_, err := pool.Run(context.Background(), "old-router", "show version")
	if err == nil {
		t.Fatal("expected error for missing password")
	}
}

func TestPool_Run_UnknownTarget(t *testing.T) {
	inv := &inventory.Inventory{}
	pool := newTestPool(inv)
	defer pool.Close()

	_, err := pool.Run(context.Background(), "does-not-exist", "show version")
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestPool_Run_RefusesNonTelnetTarget(t *testing.T) {
	inv := &inventory.Inventory{
		Servers: map[string]inventory.Server{
			"linux-box": {Hostname: "10.0.0.1", User: "hermes", Key: "~/.ssh/id"},
		},
	}
	pool := newTestPool(inv)
	defer pool.Close()

	_, err := pool.Run(context.Background(), "linux-box", "uptime")
	if err == nil {
		t.Fatal("expected error: telnet.Pool must refuse an ssh-protocol target")
	}
}

func TestPool_Close_Idempotent(t *testing.T) {
	srv := startFakeTelnetServer(t, map[string]string{"show version": "IOS 12.4"})
	inv := testInventoryFor(t, srv)
	pool := newTestPool(inv)

	if _, err := pool.Run(context.Background(), "old-router", "show version"); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("second close should be a no-op, got: %v", err)
	}
}
