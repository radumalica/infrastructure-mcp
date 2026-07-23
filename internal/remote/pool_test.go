package remote

import (
	"context"
	"errors"
	"testing"
	"time"

	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/ssh"
	"infrastructure-mcp/internal/telnet"
)

type fakeSSHRunner struct {
	calledWith string
	result     ssh.Result
	err        error
	closed     bool
}

func (f *fakeSSHRunner) Run(_ context.Context, target, _ string) (ssh.Result, error) {
	f.calledWith = target
	return f.result, f.err
}
func (f *fakeSSHRunner) Close() error { f.closed = true; return nil }

type fakeTelnetRunner struct {
	calledWith string
	result     telnet.Result
	err        error
	closed     bool
}

func (f *fakeTelnetRunner) Run(_ context.Context, target, _ string) (telnet.Result, error) {
	f.calledWith = target
	return f.result, f.err
}
func (f *fakeTelnetRunner) Close() error { f.closed = true; return nil }

func testInventory() *inventory.Inventory {
	return &inventory.Inventory{
		Servers: map[string]inventory.Server{
			"archive": {Hostname: "10.0.0.5", User: "hermes", Key: "~/.ssh/archive"},
		},
		Routers: map[string]inventory.NetworkDevice{
			"modern-core": {Hostname: "10.0.0.10", Vendor: "cisco", User: "admin", Password: "x"},
			"old-core":    {Hostname: "10.0.0.11", Vendor: "cisco", User: "admin", Password: "x", Protocol: "telnet"},
		},
	}
}

func newTestPool(sshR *fakeSSHRunner, telnetR *fakeTelnetRunner) *Pool {
	return &Pool{inv: testInventory(), ssh: sshR, telnet: telnetR}
}

func TestPool_Run_DispatchesSSHForServer(t *testing.T) {
	sshR := &fakeSSHRunner{result: ssh.Result{Stdout: "ssh-out"}}
	telnetR := &fakeTelnetRunner{}
	pool := newTestPool(sshR, telnetR)

	res, err := pool.Run(context.Background(), "archive", "uptime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "ssh-out" {
		t.Errorf("Stdout = %q, want ssh-out", res.Stdout)
	}
	if sshR.calledWith != "archive" {
		t.Errorf("ssh runner called with %q, want archive", sshR.calledWith)
	}
	if telnetR.calledWith != "" {
		t.Error("telnet runner should not have been called")
	}
}

func TestPool_Run_DispatchesSSHForModernRouter(t *testing.T) {
	sshR := &fakeSSHRunner{result: ssh.Result{Stdout: "ssh-out"}}
	telnetR := &fakeTelnetRunner{}
	pool := newTestPool(sshR, telnetR)

	if _, err := pool.Run(context.Background(), "modern-core", "show version"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sshR.calledWith != "modern-core" {
		t.Errorf("ssh runner called with %q, want modern-core", sshR.calledWith)
	}
}

func TestPool_Run_DispatchesTelnetForOldRouter(t *testing.T) {
	sshR := &fakeSSHRunner{}
	telnetR := &fakeTelnetRunner{result: telnet.Result{Stdout: "telnet-out", Duration: time.Second}}
	pool := newTestPool(sshR, telnetR)

	res, err := pool.Run(context.Background(), "old-core", "show version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "telnet-out" {
		t.Errorf("Stdout = %q, want telnet-out", res.Stdout)
	}
	if res.Duration != time.Second {
		t.Errorf("Duration = %v, want 1s", res.Duration)
	}
	if telnetR.calledWith != "old-core" {
		t.Errorf("telnet runner called with %q, want old-core", telnetR.calledWith)
	}
	if sshR.calledWith != "" {
		t.Error("ssh runner should not have been called")
	}
}

func TestPool_Run_UnknownTarget(t *testing.T) {
	pool := newTestPool(&fakeSSHRunner{}, &fakeTelnetRunner{})

	_, err := pool.Run(context.Background(), "does-not-exist", "uptime")
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestPool_Run_PropagatesTelnetError(t *testing.T) {
	telnetR := &fakeTelnetRunner{err: errors.New("login failed")}
	pool := newTestPool(&fakeSSHRunner{}, telnetR)

	_, err := pool.Run(context.Background(), "old-core", "show version")
	if err == nil {
		t.Fatal("expected error to propagate from telnet runner")
	}
}

func TestPool_Close_ClosesBothRunners(t *testing.T) {
	sshR := &fakeSSHRunner{}
	telnetR := &fakeTelnetRunner{}
	pool := newTestPool(sshR, telnetR)

	if err := pool.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sshR.closed {
		t.Error("expected ssh runner to be closed")
	}
	if !telnetR.closed {
		t.Error("expected telnet runner to be closed")
	}
}
