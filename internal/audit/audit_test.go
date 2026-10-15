package audit

import (
	"sync"
	"testing"
	"time"
)

func TestLog_RecentIsEmptyInitially(t *testing.T) {
	l := New(3)
	if got := l.Recent(); len(got) != 0 {
		t.Errorf("Recent() = %+v, want empty", got)
	}
}

func TestLog_RecentIsNewestFirst(t *testing.T) {
	l := New(3)
	l.Record(Entry{Tool: "a"})
	l.Record(Entry{Tool: "b"})
	l.Record(Entry{Tool: "c"})

	got := l.Recent()
	want := []string{"c", "b", "a"}
	if len(got) != len(want) {
		t.Fatalf("Recent() = %+v, want %d entries", got, len(want))
	}
	for i, w := range want {
		if got[i].Tool != w {
			t.Errorf("Recent()[%d].Tool = %q, want %q", i, got[i].Tool, w)
		}
	}
}

func TestLog_OverwritesOldestOnceFull(t *testing.T) {
	l := New(2)
	l.Record(Entry{Tool: "a"})
	l.Record(Entry{Tool: "b"})
	l.Record(Entry{Tool: "c"})

	got := l.Recent()
	want := []string{"c", "b"}
	if len(got) != len(want) {
		t.Fatalf("Recent() = %+v, want %d entries", got, len(want))
	}
	for i, w := range want {
		if got[i].Tool != w {
			t.Errorf("Recent()[%d].Tool = %q, want %q", i, got[i].Tool, w)
		}
	}
}

func TestLog_CapacityFlooredAtOne(t *testing.T) {
	l := New(0)
	l.Record(Entry{Tool: "a"})
	l.Record(Entry{Tool: "b"})

	got := l.Recent()
	if len(got) != 1 || got[0].Tool != "b" {
		t.Errorf("Recent() = %+v, want a single entry {Tool: b}", got)
	}
}

func TestLog_FieldsRoundTrip(t *testing.T) {
	l := New(1)
	now := time.Now().UTC()
	l.Record(Entry{Timestamp: now, Tool: "docker_restart", Target: "archive", User: "claude", Status: "restarted"})

	got := l.Recent()
	if len(got) != 1 {
		t.Fatalf("Recent() = %+v, want 1 entry", got)
	}
	e := got[0]
	if e.Tool != "docker_restart" || e.Target != "archive" || e.User != "claude" || e.Status != "restarted" || !e.Timestamp.Equal(now) {
		t.Errorf("unexpected entry: %+v", e)
	}
}

func TestLog_ConcurrentRecordIsSafe(t *testing.T) {
	l := New(50)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Record(Entry{Tool: "docker_restart"})
		}()
	}
	wg.Wait()

	if got := len(l.Recent()); got != 50 {
		t.Errorf("Recent() returned %d entries, want 50 (capacity)", got)
	}
}
