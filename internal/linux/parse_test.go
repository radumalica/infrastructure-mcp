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

func TestParseFailedServices(t *testing.T) {
	input := "nginx.service    loaded failed failed A high performance web server\n" +
		"myapp.service    loaded failed failed\n" +
		"\n"
	got := parseFailedServices(input)
	want := []FailedService{
		{Unit: "nginx.service", Load: "loaded", Active: "failed", Sub: "failed", Description: "A high performance web server"},
		{Unit: "myapp.service", Load: "loaded", Active: "failed", Sub: "failed", Description: ""},
	}
	if len(got) != len(want) {
		t.Fatalf("parseFailedServices() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseFailedServices_Empty(t *testing.T) {
	got := parseFailedServices("")
	if len(got) != 0 {
		t.Errorf("expected no entries, got %+v", got)
	}
}

func TestParseCPUUsage(t *testing.T) {
	// user nice system idle iowait irq softirq steal guest guest_nice
	sample0 := "cpu  1000 0 500 8000 100 0 0 0 0 0\n" +
		"cpu0 500 0 250 4000 50 0 0 0 0 0\n"
	sample1 := "cpu  1200 0 600 8200 100 0 0 0 0 0\n" +
		"cpu0 600 0 300 4100 50 0 0 0 0 0\n"
	got, err := parseCPUUsage(sample0 + sample1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// totalDelta = (1200-1000)+(600-500)+(8200-8000)+(100-100) = 200+100+200+0 = 500
	// idleDelta (idle+iowait) = (8200-8000)+(100-100) = 200
	// used = (500-200)/500*100 = 60
	want := 60.0
	if got.UsedPercent != want {
		t.Errorf("UsedPercent = %v, want %v", got.UsedPercent, want)
	}
}

// TestParseCPUUsage_GuestNotDoubleCounted guards against summing
// guest/guest_nice (indices 8-9) on top of user/nice, which the kernel
// already folds them into — a real risk here, since this project targets
// Proxmox/KVM hosts where guest time is a large fraction of total CPU.
func TestParseCPUUsage_GuestNotDoubleCounted(t *testing.T) {
	// user nice system idle iowait irq softirq steal guest guest_nice
	sample0 := "cpu  1000 0 500 8000 100 0 0 0 300 0\n"
	sample1 := "cpu  1200 0 600 8200 100 0 0 0 400 0\n"
	got, err := parseCPUUsage(sample0 + sample1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Guest time is already included in the user delta (200), so the
	// totalDelta/idleDelta/result must be identical to the guest=0 case:
	// totalDelta = 500, idleDelta = 200, used = 60%.
	want := 60.0
	if got.UsedPercent != want {
		t.Errorf("UsedPercent = %v, want %v (guest/guest_nice must not be double-counted)", got.UsedPercent, want)
	}
}

func TestParseCPUUsage_MissingSecondSample(t *testing.T) {
	_, err := parseCPUUsage("cpu  1000 0 500 8000 100 0 0 0 0 0\n")
	if err == nil {
		t.Fatal("expected error for a single sample")
	}
}

func TestParseCPUUsage_MalformedField(t *testing.T) {
	input := "cpu  not-a-number 0 500 8000 100 0 0 0 0 0\n" +
		"cpu  1200 0 600 8200 100 0 0 0 0 0\n"
	_, err := parseCPUUsage(input)
	if err == nil {
		t.Fatal("expected error for a malformed field")
	}
}

func TestParseRebootRequired(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    RebootRequired
		wantErr bool
	}{
		{
			name:  "marker file present",
			input: "REBOOT_REQUIRED=1\n6.8.0-1\n6.8.0-1\n",
			want: RebootRequired{
				Required: true, Reason: "/var/run/reboot-required marker file is present",
				RunningKernel: "6.8.0-1", NewestKernel: "6.8.0-1",
			},
		},
		{
			name:  "kernel mismatch",
			input: "REBOOT_REQUIRED=0\n6.8.0-1\n6.8.0-2\n",
			want: RebootRequired{
				Required: true, Reason: "running kernel 6.8.0-1 differs from newest installed kernel 6.8.0-2",
				RunningKernel: "6.8.0-1", NewestKernel: "6.8.0-2",
			},
		},
		{
			name:  "up to date",
			input: "REBOOT_REQUIRED=0\n6.8.0-1\n6.8.0-1\n",
			want: RebootRequired{
				Required: false, RunningKernel: "6.8.0-1", NewestKernel: "6.8.0-1",
			},
		},
		{
			name:  "no newest kernel detected",
			input: "REBOOT_REQUIRED=0\n6.8.0-1\n\n",
			want: RebootRequired{
				Required: false, RunningKernel: "6.8.0-1", NewestKernel: "",
			},
		},
		{
			name:    "too few lines",
			input:   "REBOOT_REQUIRED=0\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRebootRequired(tt.input)
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
				t.Errorf("parseRebootRequired() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseProcesses(t *testing.T) {
	input := "1234   1 root    12.3  4.5 nginx\n" +
		"5678 1234 www-data 1.0  0.5 nginx: worker process\n"
	got, err := parseProcesses(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []ProcessInfo{
		{PID: 1234, PPID: 1, User: "root", CPUPercent: 12.3, MemPercent: 4.5, Command: "nginx"},
		{PID: 5678, PPID: 1234, User: "www-data", CPUPercent: 1.0, MemPercent: 0.5, Command: "nginx: worker process"},
	}
	if len(got) != len(want) {
		t.Fatalf("parseProcesses() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseProcesses_MalformedLine(t *testing.T) {
	_, err := parseProcesses("only three fields\n")
	if err == nil {
		t.Fatal("expected error for malformed line")
	}
}

func TestParseJournalErrors(t *testing.T) {
	input := `{"__REALTIME_TIMESTAMP":"1700000000000000","PRIORITY":"3","_SYSTEMD_UNIT":"nginx.service","MESSAGE":"worker exited on signal 11"}
{"__REALTIME_TIMESTAMP":"1700000001000000","PRIORITY":"3","SYSLOG_IDENTIFIER":"kernel","MESSAGE":[111,111,112,115]}
`
	got, err := parseJournalErrors(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].Unit != "nginx.service" || got[0].Message != "worker exited on signal 11" || got[0].Priority != "3" {
		t.Errorf("entry 0 = %+v", got[0])
	}
	if !got[0].Timestamp.Equal(time.UnixMicro(1700000000000000).UTC()) {
		t.Errorf("entry 0 timestamp = %v", got[0].Timestamp)
	}
	if got[1].Unit != "kernel" || got[1].Message != "oops" {
		t.Errorf("entry 1 = %+v", got[1])
	}
}

func TestParseJournalErrors_MalformedJSON(t *testing.T) {
	_, err := parseJournalErrors("not json\n")
	if err == nil {
		t.Fatal("expected error for malformed JSON line")
	}
}
