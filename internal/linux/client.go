package linux

import (
	"context"
	"fmt"
	"strings"
)

// Client runs Linux diagnostics against inventory servers via a Runner
// (normally an *ssh.Pool).
type Client struct {
	runner Runner
}

// New creates a Client backed by runner.
func New(runner Runner) *Client {
	return &Client{runner: runner}
}

// Uptime returns how long the named server has been running and its
// current load averages.
func (c *Client) Uptime(ctx context.Context, server string) (UptimeInfo, error) {
	res, err := c.runner.Run(ctx, server, "cat /proc/uptime /proc/loadavg")
	if err != nil {
		return UptimeInfo{}, fmt.Errorf("linux: uptime on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return UptimeInfo{}, fmt.Errorf("linux: uptime on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseUptime(res.Stdout)
}

// DiskUsage returns per-filesystem space usage for the named server.
func (c *Client) DiskUsage(ctx context.Context, server string) ([]DiskUsage, error) {
	res, err := c.runner.Run(ctx, server, "df -kP")
	if err != nil {
		return nil, fmt.Errorf("linux: disk usage on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("linux: disk usage on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseDiskUsage(res.Stdout)
}

// MemoryUsage returns memory utilization for the named server.
func (c *Client) MemoryUsage(ctx context.Context, server string) (MemoryUsage, error) {
	res, err := c.runner.Run(ctx, server, "cat /proc/meminfo")
	if err != nil {
		return MemoryUsage{}, fmt.Errorf("linux: memory usage on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return MemoryUsage{}, fmt.Errorf("linux: memory usage on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseMemInfo(res.Stdout)
}

// FailedServices returns systemd units currently in the "failed" state.
func (c *Client) FailedServices(ctx context.Context, server string) ([]FailedService, error) {
	res, err := c.runner.Run(ctx, server, "systemctl list-units --type=service --state=failed --no-legend --plain --no-pager")
	if err != nil {
		return nil, fmt.Errorf("linux: failed services on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("linux: failed services on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseFailedServices(res.Stdout), nil
}

// CPUUsage samples /proc/stat twice, one second apart, and returns the
// aggregate CPU utilization over that window.
func (c *Client) CPUUsage(ctx context.Context, server string) (CPUUsage, error) {
	res, err := c.runner.Run(ctx, server, "sh -c 'cat /proc/stat; sleep 1; cat /proc/stat'")
	if err != nil {
		return CPUUsage{}, fmt.Errorf("linux: cpu usage on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return CPUUsage{}, fmt.Errorf("linux: cpu usage on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseCPUUsage(res.Stdout)
}

// RebootRequired reports whether the named server is pending a reboot,
// combining the Debian/Ubuntu reboot-required marker file with a
// running-vs-newest-installed kernel comparison (best-effort on
// distributions that don't use /boot/vmlinuz-* naming).
func (c *Client) RebootRequired(ctx context.Context, server string) (RebootRequired, error) {
	const cmd = `sh -c 'test -f /var/run/reboot-required && echo REBOOT_REQUIRED=1 || echo REBOOT_REQUIRED=0; uname -r; ls -1 /boot 2>/dev/null | grep -E "^vmlinuz-" | sed "s/^vmlinuz-//" | sort -V | tail -1'`
	res, err := c.runner.Run(ctx, server, cmd)
	if err != nil {
		return RebootRequired{}, fmt.Errorf("linux: reboot required on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return RebootRequired{}, fmt.Errorf("linux: reboot required on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseRebootRequired(res.Stdout)
}

// defaultProcessLimit bounds RunningProcesses when the caller doesn't
// specify one; maxProcessLimit bounds it against unreasonable requests.
const (
	defaultProcessLimit = 20
	maxProcessLimit     = 500
)

// RunningProcesses returns the top processes on the named server, sorted
// by CPU usage descending. limit <= 0 uses defaultProcessLimit; values
// above maxProcessLimit are clamped.
func (c *Client) RunningProcesses(ctx context.Context, server string, limit int) ([]ProcessInfo, error) {
	limit = clampLimit(limit, defaultProcessLimit, maxProcessLimit)
	cmd := fmt.Sprintf("ps -eo pid,ppid,user:20,pcpu,pmem,comm --no-headers --sort=-pcpu | head -n %d", limit)
	res, err := c.runner.Run(ctx, server, cmd)
	if err != nil {
		return nil, fmt.Errorf("linux: running processes on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("linux: running processes on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseProcesses(res.Stdout)
}

// defaultJournalLimit/maxJournalLimit bound JournalErrors the same way
// RunningProcesses bounds its process count.
const (
	defaultJournalLimit = 20
	maxJournalLimit     = 500
)

// JournalErrors returns the most recent systemd journal entries at error
// priority or higher. limit <= 0 uses defaultJournalLimit; values above
// maxJournalLimit are clamped.
func (c *Client) JournalErrors(ctx context.Context, server string, limit int) ([]JournalEntry, error) {
	limit = clampLimit(limit, defaultJournalLimit, maxJournalLimit)
	cmd := fmt.Sprintf("journalctl -p err -n %d --no-pager -o json", limit)
	res, err := c.runner.Run(ctx, server, cmd)
	if err != nil {
		return nil, fmt.Errorf("linux: journal errors on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("linux: journal errors on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseJournalErrors(res.Stdout)
}

// KernelVersion returns the running kernel release string (`uname -r`).
func (c *Client) KernelVersion(ctx context.Context, server string) (string, error) {
	res, err := c.runner.Run(ctx, server, "uname -r")
	if err != nil {
		return "", fmt.Errorf("linux: kernel version on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("linux: kernel version on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return strings.TrimSpace(res.Stdout), nil
}

func clampLimit(limit, def, max int) int {
	switch {
	case limit <= 0:
		return def
	case limit > max:
		return max
	default:
		return limit
	}
}
