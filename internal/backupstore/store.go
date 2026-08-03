// Package backupstore persists the most recent text snapshot (e.g. a
// device's running-config) per named target on disk, and diffs a new
// snapshot against the previously saved one. It has no knowledge of any
// particular vendor or adapter — cisco_backup_diff is its first caller,
// but the shape is generic enough to reuse for any other "detect drift in
// a periodically-fetched text blob" tool.
//
// Deliberately scoped to one snapshot per target, not a rotating history
// of N — the operator need this serves ("did this change since the last
// time I looked?") only requires the immediately-previous snapshot, and
// keeping a bounded history (retention policy, disk growth, concurrent
// rotation) is a meaningfully bigger feature than this one warrants.
package backupstore

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/pmezard/go-difflib/difflib"

	"infrastructure-mcp/internal/toolerr"
)

// targetNamePattern whitelists the characters allowed in a target name
// before it's used to build a filesystem path — the same "no shell
// interpolation" posture internal/docker's validateContainerRef applies
// to a remote command, applied here to a local file path instead. This
// also happens to match the inventory package's own device/server naming
// conventions.
var targetNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

// Result is the outcome of saving a new snapshot and diffing it against
// whatever was previously saved for the same target.
type Result struct {
	// Changed is true if this is not the first snapshot for target and
	// its content differs from the previously saved one.
	Changed bool
	// FirstSnapshot is true if there was no previous snapshot to diff
	// against — Diff is empty in that case.
	FirstSnapshot bool
	// Diff is a unified diff (old -> new), empty if !Changed.
	Diff string
}

// Store persists one text snapshot per target under dir, one file per
// target. Safe for concurrent use.
type Store struct {
	dir string

	mu sync.Mutex
}

// New creates a Store that persists snapshots under dir. dir is created
// on first use if it doesn't already exist.
func New(dir string) *Store {
	return &Store{dir: dir}
}

// SaveAndDiff saves newContent as target's snapshot, returning a diff
// against whatever was previously saved for target (if anything).
func (s *Store) SaveAndDiff(target, newContent string) (Result, error) {
	if !targetNamePattern.MatchString(target) {
		return Result{}, fmt.Errorf("backupstore: invalid target name %q: %w", target, toolerr.ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return Result{}, fmt.Errorf("backupstore: create directory %q: %w", s.dir, err)
	}

	path := filepath.Join(s.dir, target+".cfg")

	// A config backup can legitimately contain secrets (SNMP community
	// strings, local user passwords in cleartext or a weak hash) —
	// 0600, not the default 0644, mirrors the "no secrets exposed
	// needlessly" posture the rest of this project takes with logs and
	// tool output.
	const filePerm = 0o600

	old, err := os.ReadFile(path)
	firstSnapshot := os.IsNotExist(err)
	if err != nil && !firstSnapshot {
		return Result{}, fmt.Errorf("backupstore: read previous snapshot for %q: %w", target, err)
	}

	if err := os.WriteFile(path, []byte(newContent), filePerm); err != nil {
		return Result{}, fmt.Errorf("backupstore: write snapshot for %q: %w", target, err)
	}

	if firstSnapshot {
		return Result{FirstSnapshot: true}, nil
	}

	oldStr := string(old)
	if oldStr == newContent {
		return Result{}, nil
	}

	diffText, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldStr),
		B:        difflib.SplitLines(newContent),
		FromFile: "previous",
		ToFile:   "current",
		Context:  3,
	})
	if err != nil {
		return Result{}, fmt.Errorf("backupstore: compute diff for %q: %w", target, err)
	}

	return Result{Changed: true, Diff: diffText}, nil
}
