// Package audit is a small, fixed-capacity, in-memory record of the most
// recent mutating tool invocations (docker_restart, proxmox_start_vm,
// proxmox_stop_vm, proxmox_snapshot). It exists so an agent can check
// "what destructive/state-changing actions has this server already taken
// this session" before proposing another one — not as a durable audit
// trail. Entries are lost on restart; there is no persistence layer here,
// deliberately: a real audit trail (retention, tamper-evidence, multi-
// instance aggregation) is a meaningfully bigger feature than this one
// warrants. See mcp/resources for how this is exposed to agents.
package audit

import (
	"sync"
	"time"
)

// Entry is one recorded tool invocation.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Tool      string    `json:"tool"`
	Target    string    `json:"target"`
	User      string    `json:"user"`
	Status    string    `json:"status"`
}

// Log is a fixed-capacity ring buffer of Entry, safe for concurrent use.
type Log struct {
	mu       sync.Mutex
	entries  []Entry
	capacity int
	next     int
	full     bool
}

// New creates a Log that retains the most recent capacity entries. Older
// entries are silently overwritten once capacity is reached. capacity is
// floored at 1.
func New(capacity int) *Log {
	if capacity < 1 {
		capacity = 1
	}
	return &Log{entries: make([]Entry, capacity), capacity: capacity}
}

// Record appends e, overwriting the oldest entry once the log is full.
func (l *Log) Record(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries[l.next] = e
	l.next = (l.next + 1) % l.capacity
	if l.next == 0 {
		l.full = true
	}
}

// Recent returns every recorded entry, newest first.
func (l *Log) Recent() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	n := l.next
	if l.full {
		n = l.capacity
	}

	out := make([]Entry, 0, n)
	for i := 0; i < n; i++ {
		idx := (l.next - 1 - i + l.capacity) % l.capacity
		out = append(out, l.entries[idx])
	}
	return out
}
