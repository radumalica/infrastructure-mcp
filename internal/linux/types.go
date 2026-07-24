// Package linux implements Linux-specific diagnostics (uptime, disk usage,
// memory usage) by running portable shell commands over an SSH connection
// and parsing their output. Every command reads from /proc or uses POSIX
// flags (df -kP) so behavior is consistent across distributions.
package linux

import (
	"context"
	"time"

	"infrastructure-mcp/internal/ssh"
)

// Runner executes a command on a named inventory server. *ssh.Pool
// satisfies this interface; tests substitute a fake.
type Runner interface {
	Run(ctx context.Context, server, command string) (ssh.Result, error)
}

// UptimeInfo describes how long a host has been running and its recent
// load.
type UptimeInfo struct {
	Uptime time.Duration
	Load1  float64
	Load5  float64
	Load15 float64
}

// DiskUsage describes space usage for a single mounted filesystem.
type DiskUsage struct {
	Filesystem  string
	MountPoint  string
	TotalKB     int64
	UsedKB      int64
	AvailableKB int64
	UsedPercent int
}

// MemoryUsage describes system memory utilization, derived from
// /proc/meminfo.
type MemoryUsage struct {
	TotalKB     int64
	FreeKB      int64
	AvailableKB int64
	UsedKB      int64
	UsedPercent float64
	SwapTotalKB int64
	SwapFreeKB  int64
}

// FailedService describes one systemd unit in the "failed" state, as
// reported by `systemctl list-units --state=failed`.
type FailedService struct {
	Unit        string
	Load        string
	Active      string
	Sub         string
	Description string
}

// CPUUsage reports aggregate CPU utilization sampled over a short window.
type CPUUsage struct {
	UsedPercent float64
}

// RebootRequired reports whether a host is pending a reboot, and why.
type RebootRequired struct {
	Required      bool
	Reason        string
	RunningKernel string
	NewestKernel  string
}

// ProcessInfo describes one running process, as reported by `ps`.
type ProcessInfo struct {
	PID        int
	PPID       int
	User       string
	CPUPercent float64
	MemPercent float64
	Command    string
}

// JournalEntry describes one systemd journal entry at error priority or
// higher.
type JournalEntry struct {
	Timestamp time.Time
	Unit      string
	Priority  string
	Message   string
}
