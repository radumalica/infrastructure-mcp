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
	if _, err := c.FailedServices(context.Background(), "unscripted"); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.CPUUsage(context.Background(), "unscripted"); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.RebootRequired(context.Background(), "unscripted"); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.RunningProcesses(context.Background(), "unscripted", 0); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.JournalErrors(context.Background(), "unscripted", 0); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.KernelVersion(context.Background(), "unscripted"); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
}

func TestClient_FailedServices(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|systemctl list-units --type=service --state=failed --no-legend --plain --no-pager": {
			Stdout:   "nginx.service loaded failed failed A web server\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.FailedServices(context.Background(), "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Unit != "nginx.service" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_FailedServices_NonZeroExit(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|systemctl list-units --type=service --state=failed --no-legend --plain --no-pager": {ExitCode: 1, Stderr: "not found"},
	}}
	c := New(runner)

	if _, err := c.FailedServices(context.Background(), "archive"); err == nil {
		t.Fatal("expected error for nonzero exit")
	}
}

func TestClient_CPUUsage(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|sh -c 'cat /proc/stat; sleep 1; cat /proc/stat'": {
			Stdout: "cpu  1000 0 500 8000 100 0 0 0 0 0\n" +
				"cpu  1200 0 600 8200 100 0 0 0 0 0\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.CPUUsage(context.Background(), "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UsedPercent != 60 {
		t.Errorf("UsedPercent = %v, want 60", got.UsedPercent)
	}
}

func TestClient_RebootRequired(t *testing.T) {
	const cmd = `sh -c 'test -f /var/run/reboot-required && echo REBOOT_REQUIRED=1 || echo REBOOT_REQUIRED=0; uname -r; ls -1 /boot 2>/dev/null | grep -E "^vmlinuz-" | sed "s/^vmlinuz-//" | sort -V | tail -1'`
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|" + cmd: {Stdout: "REBOOT_REQUIRED=1\n6.8.0-1\n6.8.0-1\n", ExitCode: 0},
	}}
	c := New(runner)

	got, err := c.RebootRequired(context.Background(), "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Required {
		t.Errorf("Required = false, want true: %+v", got)
	}
}

func TestClient_RunningProcesses(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|ps -eo pid,ppid,user:20,pcpu,pmem,comm --no-headers --sort=-pcpu | head -n 20": {
			Stdout:   "1234 1 root 12.3 4.5 nginx\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.RunningProcesses(context.Background(), "archive", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].PID != 1234 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_RunningProcesses_LimitClamping(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|ps -eo pid,ppid,user:20,pcpu,pmem,comm --no-headers --sort=-pcpu | head -n 500": {
			Stdout:   "1234 1 root 12.3 4.5 nginx\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	if _, err := c.RunningProcesses(context.Background(), "archive", 10000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_JournalErrors(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|journalctl -p err -n 20 --no-pager -o json": {
			Stdout:   `{"__REALTIME_TIMESTAMP":"1700000000000000","PRIORITY":"3","_SYSTEMD_UNIT":"nginx.service","MESSAGE":"boom"}` + "\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.JournalErrors(context.Background(), "archive", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Message != "boom" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_KernelVersion(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|uname -r": {Stdout: "6.8.0-1-generic\n", ExitCode: 0},
	}}
	c := New(runner)

	got, err := c.KernelVersion(context.Background(), "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "6.8.0-1-generic" {
		t.Errorf("KernelVersion = %q, want %q", got, "6.8.0-1-generic")
	}
}

func TestClient_KernelVersion_NonZeroExit(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|uname -r": {ExitCode: 1, Stderr: "denied"},
	}}
	c := New(runner)

	if _, err := c.KernelVersion(context.Background(), "archive"); err == nil {
		t.Fatal("expected error for nonzero exit")
	}
}
