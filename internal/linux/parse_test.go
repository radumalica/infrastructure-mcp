package linux

import (
	"testing"
	"time"
)

func TestParseUptime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    UptimeInfo
		wantErr bool
	}{
		{
			name:  "typical",
			input: "12345.67 98765.43\n0.10 0.05 0.01 1/234 5678\n",
			want: UptimeInfo{
				Uptime: time.Duration(12345.67 * float64(time.Second)),
				Load1:  0.10,
				Load5:  0.05,
				Load15: 0.01,
			},
		},
		{
			name:    "too few lines",
			input:   "12345.67 98765.43\n",
			wantErr: true,
		},
		{
			name:    "malformed uptime line",
			input:   "not-a-number\n0.10 0.05 0.01\n",
			wantErr: true,
		},
		{
			name:    "malformed loadavg line",
			input:   "12345.67 98765.43\nnot enough\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUptime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseUptime() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseDiskUsage(t *testing.T) {
	const output = `Filesystem     1024-blocks     Used Available Capacity Mounted on
/dev/sda1         10485760  5242880   5242880      50% /
tmpfs                65536        0     65536       0% /dev/shm
/dev/sdb1        104857600 94371840  10485760      90% /mnt/data with space
`
	got, err := parseDiskUsage(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(got), got)
	}

	want := DiskUsage{
		Filesystem:  "/dev/sda1",
		MountPoint:  "/",
		TotalKB:     10485760,
		UsedKB:      5242880,
		AvailableKB: 5242880,
		UsedPercent: 50,
	}
	if got[0] != want {
		t.Errorf("entry 0 = %+v, want %+v", got[0], want)
	}

	if got[2].MountPoint != "/mnt/data with space" {
		t.Errorf("expected mount point with space to be preserved, got %q", got[2].MountPoint)
	}
}

func TestParseDiskUsage_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"too few fields", "Filesystem  Blocks\n/dev/sda1 100\n"},
		{"bad total", "header\n/dev/sda1 x 1 1 50% /\n"},
		{"bad used", "header\n/dev/sda1 100 x 1 50% /\n"},
		{"bad available", "header\n/dev/sda1 100 1 x 50% /\n"},
		{"bad percent", "header\n/dev/sda1 100 1 1 x% /\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseDiskUsage(tt.input); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestParseMemInfo(t *testing.T) {
	const output = `MemTotal:       16384000 kB
MemFree:         2048000 kB
MemAvailable:    8192000 kB
Buffers:          512000 kB
Cached:          1024000 kB
SwapTotal:       4096000 kB
SwapFree:        4096000 kB
`
	got, err := parseMemInfo(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := MemoryUsage{
		TotalKB:     16384000,
		FreeKB:      2048000,
		AvailableKB: 8192000,
		UsedKB:      16384000 - 8192000,
		UsedPercent: float64(16384000-8192000) / 16384000 * 100,
		SwapTotalKB: 4096000,
		SwapFreeKB:  4096000,
	}
	if got != want {
		t.Errorf("parseMemInfo() = %+v, want %+v", got, want)
	}
}

func TestParseMemInfo_FallsBackWithoutMemAvailable(t *testing.T) {
	const output = `MemTotal:       1000000 kB
MemFree:         100000 kB
Buffers:          50000 kB
Cached:          150000 kB
`
	got, err := parseMemInfo(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantAvailable := int64(100000 + 50000 + 150000)
	if got.AvailableKB != wantAvailable {
		t.Errorf("AvailableKB = %d, want %d", got.AvailableKB, wantAvailable)
	}
}

func TestParseMemInfo_MissingMemTotal(t *testing.T) {
	_, err := parseMemInfo("MemFree: 1000 kB\n")
	if err == nil {
		t.Fatal("expected error for missing MemTotal")
	}
}
