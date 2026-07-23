package linux

import (
	"context"
	"fmt"
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
