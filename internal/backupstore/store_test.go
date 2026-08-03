package backupstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndDiff_FirstSnapshot(t *testing.T) {
	s := New(t.TempDir())

	res, err := s.SaveAndDiff("core-sw", "hostname router1\n!\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.FirstSnapshot || res.Changed || res.Diff != "" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestSaveAndDiff_NoChange(t *testing.T) {
	s := New(t.TempDir())

	if _, err := s.SaveAndDiff("core-sw", "hostname router1\n!\n"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	res, err := s.SaveAndDiff("core-sw", "hostname router1\n!\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FirstSnapshot || res.Changed || res.Diff != "" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestSaveAndDiff_Changed(t *testing.T) {
	s := New(t.TempDir())

	if _, err := s.SaveAndDiff("core-sw", "hostname router1\ninterface Gi0/1\n!\n"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	res, err := s.SaveAndDiff("core-sw", "hostname router1\ninterface Gi0/2\n!\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FirstSnapshot || !res.Changed {
		t.Fatalf("unexpected result: %+v", res)
	}
	if !strings.Contains(res.Diff, "-interface Gi0/1") || !strings.Contains(res.Diff, "+interface Gi0/2") {
		t.Errorf("diff missing expected lines: %q", res.Diff)
	}
}

func TestSaveAndDiff_PersistsBetweenStoreInstances(t *testing.T) {
	dir := t.TempDir()

	if _, err := New(dir).SaveAndDiff("core-sw", "v1\n"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	res, err := New(dir).SaveAndDiff("core-sw", "v2\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Changed {
		t.Errorf("expected a change to be detected across separate Store instances sharing dir")
	}
}

func TestSaveAndDiff_InvalidTargetName(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.SaveAndDiff("../../etc/passwd", "x"); err == nil {
		t.Fatal("expected an error for a path-traversal target name")
	}
}

func TestSaveAndDiff_CreatesDirIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "backups")
	s := New(dir)

	if _, err := s.SaveAndDiff("core-sw", "hostname router1\n"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "core-sw.cfg")); err != nil {
		t.Errorf("expected snapshot file to exist: %v", err)
	}
}
