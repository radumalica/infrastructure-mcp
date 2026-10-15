package cisco

import (
	"context"
	"errors"
	"testing"

	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/ssh"
)

type fakeRunner struct {
	result  ssh.Result
	err     error
	sawCmd  string
	sawName string
	sawCmds []string
	// results, if non-nil, is consumed one entry per call (in order),
	// overriding result for that call — used by tests that need to
	// assert on a multi-command sequence (e.g. Backup over Telnet).
	results []ssh.Result
}

func (f *fakeRunner) Run(_ context.Context, target, command string) (ssh.Result, error) {
	f.sawName, f.sawCmd = target, command
	f.sawCmds = append(f.sawCmds, command)
	if len(f.results) > 0 {
		res := f.results[0]
		f.results = f.results[1:]
		return res, f.err
	}
	return f.result, f.err
}

type fakeVendorLookup struct {
	target inventory.Target
	err    error
}

func (f fakeVendorLookup) Target(_ string) (inventory.Target, error) {
	return f.target, f.err
}

func TestBackup(t *testing.T) {
	runner := &fakeRunner{result: ssh.Result{Stdout: "hostname router1\n!\n"}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco"}})

	out, err := c.Backup(context.Background(), "core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hostname router1\n!\n" {
		t.Errorf("unexpected output: %q", out)
	}
	if runner.sawCmd != "show running-config" {
		t.Errorf("sawCmd = %q", runner.sawCmd)
	}
}

func TestBackup_TelnetDisablesPagingFirst(t *testing.T) {
	runner := &fakeRunner{results: []ssh.Result{
		{Stdout: ""},                      // terminal length 0
		{Stdout: "hostname router1\n!\n"}, // show running-config
	}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco", Protocol: inventory.ProtocolTelnet}})

	out, err := c.Backup(context.Background(), "core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hostname router1\n!\n" {
		t.Errorf("unexpected output: %q", out)
	}
	if len(runner.sawCmds) != 2 || runner.sawCmds[0] != "terminal length 0" || runner.sawCmds[1] != "show running-config" {
		t.Errorf("unexpected command sequence: %v", runner.sawCmds)
	}
}

func TestBackup_TelnetPagingDisableFails(t *testing.T) {
	runner := &fakeRunner{results: []ssh.Result{
		{ExitCode: 1, Stderr: "connection reset"},
	}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco", Protocol: inventory.ProtocolTelnet}})

	if _, err := c.Backup(context.Background(), "core"); err == nil {
		t.Fatal("expected an error when disabling paging fails")
	}
	if len(runner.sawCmds) != 1 {
		t.Errorf("expected show running-config to be skipped after the paging command failed, got %v", runner.sawCmds)
	}
}

func TestBackup_SSHSendsOnlyOneCommand(t *testing.T) {
	runner := &fakeRunner{result: ssh.Result{Stdout: "hostname router1\n!\n"}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco", Protocol: inventory.ProtocolSSH}})

	if _, err := c.Backup(context.Background(), "core"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.sawCmds) != 1 || runner.sawCmds[0] != "show running-config" {
		t.Errorf("expected exactly one command over SSH, got %v", runner.sawCmds)
	}
}

func TestShowVersion(t *testing.T) {
	runner := &fakeRunner{result: ssh.Result{Stdout: sampleShowVersion}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco"}})

	info, err := c.ShowVersion(context.Background(), "core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Hostname != "router1" {
		t.Errorf("unexpected info: %+v", info)
	}
}

func TestInterfaces(t *testing.T) {
	runner := &fakeRunner{result: ssh.Result{Stdout: sampleShowIPIntBrief}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco"}})

	entries, err := c.Interfaces(context.Background(), "core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestInventory(t *testing.T) {
	runner := &fakeRunner{result: ssh.Result{Stdout: sampleShowInventory}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco"}})

	entries, err := c.Inventory(context.Background(), "core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestLogs(t *testing.T) {
	runner := &fakeRunner{result: ssh.Result{Stdout: sampleShowLogging}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco"}})

	lines, err := c.Logs(context.Background(), "core", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %+v", len(lines), lines)
	}
}

func TestWrongVendor(t *testing.T) {
	runner := &fakeRunner{}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "mikrotik"}})

	_, err := c.Backup(context.Background(), "core")
	if !errors.Is(err, ErrWrongVendor) {
		t.Errorf("expected ErrWrongVendor, got %v", err)
	}
	if runner.sawCmd != "" {
		t.Errorf("expected runner not to be called, but it saw %q", runner.sawCmd)
	}
}

func TestTargetNotFound(t *testing.T) {
	runner := &fakeRunner{}
	c := New(runner, fakeVendorLookup{err: inventory.ErrNotFound})

	_, err := c.Backup(context.Background(), "does-not-exist")
	if !errors.Is(err, inventory.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestNonZeroExit(t *testing.T) {
	runner := &fakeRunner{result: ssh.Result{ExitCode: 1, Stderr: "% Invalid input"}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco"}})

	_, err := c.Backup(context.Background(), "core")
	if err == nil {
		t.Fatal("expected an error for non-zero exit code")
	}
}

// TestCommandRejectedNoExitCode covers the case a plain exit-code check
// can't: Telnet sessions never carry an exit status at all (internal/telnet
// always returns ExitCode 0), and IOS often reports a rejected command
// (e.g. insufficient privilege for "show running-config") inline in stdout
// with a "% ..." line rather than via any exit status.
func TestCommandRejectedNoExitCode(t *testing.T) {
	runner := &fakeRunner{result: ssh.Result{ExitCode: 0, Stdout: "% Invalid input detected at '^' marker.\n"}}
	c := New(runner, fakeVendorLookup{target: inventory.Target{Vendor: "cisco"}})

	_, err := c.Backup(context.Background(), "core")
	if !errors.Is(err, ErrCommandRejected) {
		t.Errorf("expected ErrCommandRejected, got %v", err)
	}
}
