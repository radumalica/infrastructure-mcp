package linux

import (
	"context"
	"errors"
	"testing"

	"infrastructure-mcp/internal/ssh"
)

// fakeRunner is a scripted Runner: each server+command pair maps to a
// canned ssh.Result or error.
type fakeRunner struct {
	results map[string]ssh.Result
	errs    map[string]error
}

func (f *fakeRunner) Run(_ context.Context, server, command string) (ssh.Result, error) {
	key := server + "|" + command
	if err, ok := f.errs[key]; ok {
		return ssh.Result{}, err
	}
	if res, ok := f.results[key]; ok {
		return res, nil
	}
	return ssh.Result{}, errors.New("fakeRunner: no script for " + key)
}

func TestClient_Uptime(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|cat /proc/uptime /proc/loadavg": {
			Stdout:   "100.0 200.0\n0.5 0.4 0.3 1/1 1\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.Uptime(context.Background(), "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Load1 != 0.5 {
		t.Errorf("Load1 = %v, want 0.5", got.Load1)
	}
}

func TestClient_Uptime_NonZeroExit(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|cat /proc/uptime /proc/loadavg": {ExitCode: 1, Stderr: "permission denied"},
	}}
	c := New(runner)

	_, err := c.Uptime(context.Background(), "archive")
	if err == nil {
		t.Fatal("expected error for nonzero exit")
	}
}

func TestClient_DiskUsage(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|df -kP": {
			Stdout: "Filesystem 1024-blocks Used Available Capacity Mounted on\n" +
				"/dev/sda1 100 50 50 50% /\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.DiskUsage(context.Background(), "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].UsedPercent != 50 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_MemoryUsage(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|cat /proc/meminfo": {
			Stdout:   "MemTotal: 1000 kB\nMemFree: 500 kB\nMemAvailable: 700 kB\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.MemoryUsage(context.Background(), "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalKB != 1000 {
		t.Errorf("TotalKB = %d, want 1000", got.TotalKB)
	}
}

func TestClient_RunnerError(t *testing.T) {
	runner := &fakeRunner{}
	c := New(runner)

	if _, err := c.Uptime(context.Background(), "unscripted"); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.DiskUsage(context.Background(), "unscripted"); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.MemoryUsage(context.Background(), "unscripted"); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
}
