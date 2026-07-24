package linux

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseUptime parses the combined output of
// `cat /proc/uptime /proc/loadavg` (one file's contents per line, in that
// order).
func parseUptime(output string) (UptimeInfo, error) {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) < 2 {
		return UptimeInfo{}, fmt.Errorf("linux: parse uptime: expected 2 lines, got %d", len(lines))
	}

	uptimeFields := strings.Fields(lines[0])
	if len(uptimeFields) < 1 {
		return UptimeInfo{}, fmt.Errorf("linux: parse uptime: malformed /proc/uptime line: %q", lines[0])
	}
	seconds, err := strconv.ParseFloat(uptimeFields[0], 64)
	if err != nil {
		return UptimeInfo{}, fmt.Errorf("linux: parse uptime seconds: %w", err)
	}

	loadFields := strings.Fields(lines[1])
	if len(loadFields) < 3 {
		return UptimeInfo{}, fmt.Errorf("linux: parse uptime: malformed /proc/loadavg line: %q", lines[1])
	}
	load1, err := strconv.ParseFloat(loadFields[0], 64)
	if err != nil {
		return UptimeInfo{}, fmt.Errorf("linux: parse load1: %w", err)
	}
	load5, err := strconv.ParseFloat(loadFields[1], 64)
	if err != nil {
		return UptimeInfo{}, fmt.Errorf("linux: parse load5: %w", err)
	}
	load15, err := strconv.ParseFloat(loadFields[2], 64)
	if err != nil {
		return UptimeInfo{}, fmt.Errorf("linux: parse load15: %w", err)
	}

	return UptimeInfo{
		Uptime: time.Duration(seconds * float64(time.Second)),
		Load1:  load1,
		Load5:  load5,
		Load15: load15,
	}, nil
}

// parseDiskUsage parses the output of `df -kP`, whose POSIX-mode columns
// are: Filesystem 1024-blocks Used Available Capacity Mounted-on.
func parseDiskUsage(output string) ([]DiskUsage, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var result []DiskUsage
	first := true

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if first {
			// Header row (starts with "Filesystem").
			first = false
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			return nil, fmt.Errorf("linux: parse disk usage: malformed line: %q", line)
		}

		total, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("linux: parse disk usage total: %w", err)
		}
		used, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("linux: parse disk usage used: %w", err)
		}
		avail, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("linux: parse disk usage available: %w", err)
		}
		pctStr := strings.TrimSuffix(fields[4], "%")
		pct, err := strconv.Atoi(pctStr)
		if err != nil {
			return nil, fmt.Errorf("linux: parse disk usage percent: %w", err)
		}

		// Mount point is everything from field index 5 onward, rejoined,
		// to tolerate paths containing spaces.
		mountPoint := strings.Join(fields[5:], " ")

		result = append(result, DiskUsage{
			Filesystem:  fields[0],
			MountPoint:  mountPoint,
			TotalKB:     total,
			UsedKB:      used,
			AvailableKB: avail,
			UsedPercent: pct,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("linux: parse disk usage: %w", err)
	}
	return result, nil
}

// parseMemInfo parses the output of `cat /proc/meminfo`.
func parseMemInfo(output string) (MemoryUsage, error) {
	values := map[string]int64{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		rest := strings.Fields(line[idx+1:])
		if len(rest) == 0 {
			continue
		}
		v, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil {
			continue
		}
		values[key] = v
	}
	if err := scanner.Err(); err != nil {
		return MemoryUsage{}, fmt.Errorf("linux: parse meminfo: %w", err)
	}

	total, ok := values["MemTotal"]
	if !ok {
		return MemoryUsage{}, fmt.Errorf("linux: parse meminfo: missing MemTotal")
	}
	free := values["MemFree"]

	// Prefer the kernel-reported MemAvailable (accounts for reclaimable
	// cache); fall back to Free+Buffers+Cached on older kernels that lack
	// it.
	available, ok := values["MemAvailable"]
	if !ok {
		available = free + values["Buffers"] + values["Cached"]
	}

	used := total - available
	if used < 0 {
		used = 0
	}
	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return MemoryUsage{
		TotalKB:     total,
		FreeKB:      free,
		AvailableKB: available,
		UsedKB:      used,
		UsedPercent: usedPercent,
		SwapTotalKB: values["SwapTotal"],
		SwapFreeKB:  values["SwapFree"],
	}, nil
}

// parseFailedServices parses the output of
// `systemctl list-units --type=service --state=failed --no-legend --plain`.
// Each line is: UNIT LOAD ACTIVE SUB DESCRIPTION (description may contain
// spaces, so it is rejoined from the remaining fields).
func parseFailedServices(output string) []FailedService {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var result []FailedService
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		description := ""
		if len(fields) > 4 {
			description = strings.Join(fields[4:], " ")
		}
		result = append(result, FailedService{
			Unit:        fields[0],
			Load:        fields[1],
			Active:      fields[2],
			Sub:         fields[3],
			Description: description,
		})
	}
	return result
}

// parseCPUUsage parses the combined output of two `/proc/stat` reads taken
// one second apart and returns utilization over that window, computed from
// the aggregate "cpu " line (kernel/Documentation/filesystems/proc.rst):
// fields after the label are user, nice, system, idle, iowait, irq,
// softirq, steal, guest, guest_nice (jiffies since boot).
func parseCPUUsage(output string) (CPUUsage, error) {
	var samples [][]int64
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		vals := make([]int64, 0, len(fields))
		for _, f := range fields {
			v, err := strconv.ParseInt(f, 10, 64)
			if err != nil {
				return CPUUsage{}, fmt.Errorf("linux: parse cpu usage: field %q: %w", f, err)
			}
			vals = append(vals, v)
		}
		samples = append(samples, vals)
		if len(samples) == 2 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return CPUUsage{}, fmt.Errorf("linux: parse cpu usage: %w", err)
	}
	if len(samples) != 2 {
		return CPUUsage{}, fmt.Errorf("linux: parse cpu usage: expected 2 samples of the aggregate cpu line, got %d", len(samples))
	}

	// Sum only user..steal (indices 0-7): the kernel already folds guest
	// into user and guest_nice into nice, so including guest/guest_nice
	// (indices 8-9) as well would double-count that time.
	sum := func(vals []int64) (total, idle int64) {
		for i, v := range vals {
			if i > 7 {
				break
			}
			total += v
			if i == 3 || i == 4 { // idle, iowait
				idle += v
			}
		}
		return
	}
	total0, idle0 := sum(samples[0])
	total1, idle1 := sum(samples[1])

	totalDelta := total1 - total0
	idleDelta := idle1 - idle0
	if totalDelta <= 0 {
		return CPUUsage{UsedPercent: 0}, nil
	}
	usedPercent := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	if usedPercent < 0 {
		usedPercent = 0
	}
	return CPUUsage{UsedPercent: usedPercent}, nil
}

// parseRebootRequired parses the three-line output produced by
// RebootRequired's shell command: the reboot-required marker flag, the
// running kernel release, and the newest kernel installed under /boot
// (empty if none could be determined).
func parseRebootRequired(output string) (RebootRequired, error) {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) < 2 {
		return RebootRequired{}, fmt.Errorf("linux: parse reboot required: expected at least 2 lines, got %d", len(lines))
	}

	markerSet := strings.TrimSpace(lines[0]) == "REBOOT_REQUIRED=1"
	runningKernel := strings.TrimSpace(lines[1])
	newestKernel := ""
	if len(lines) > 2 {
		newestKernel = strings.TrimSpace(lines[2])
	}

	required := markerSet
	reason := ""
	switch {
	case markerSet:
		reason = "/var/run/reboot-required marker file is present"
	case newestKernel != "" && newestKernel != runningKernel:
		required = true
		reason = fmt.Sprintf("running kernel %s differs from newest installed kernel %s", runningKernel, newestKernel)
	}

	return RebootRequired{
		Required:      required,
		Reason:        reason,
		RunningKernel: runningKernel,
		NewestKernel:  newestKernel,
	}, nil
}

// parseProcesses parses the output of
// `ps -eo pid,ppid,user:20,pcpu,pmem,comm --no-headers --sort=-pcpu`.
func parseProcesses(output string) ([]ProcessInfo, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var result []ProcessInfo
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			return nil, fmt.Errorf("linux: parse processes: malformed line: %q", line)
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("linux: parse processes pid: %w", err)
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("linux: parse processes ppid: %w", err)
		}
		pcpu, err := strconv.ParseFloat(fields[3], 64)
		if err != nil {
			return nil, fmt.Errorf("linux: parse processes pcpu: %w", err)
		}
		pmem, err := strconv.ParseFloat(fields[4], 64)
		if err != nil {
			return nil, fmt.Errorf("linux: parse processes pmem: %w", err)
		}

		result = append(result, ProcessInfo{
			PID:        pid,
			PPID:       ppid,
			User:       fields[2],
			CPUPercent: pcpu,
			MemPercent: pmem,
			Command:    strings.Join(fields[5:], " "),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("linux: parse processes: %w", err)
	}
	return result, nil
}

// journalLine mirrors the subset of `journalctl -o json` fields
// JournalErrors needs. MESSAGE is captured raw because journald emits it
// as a JSON string for text log lines but as an array of byte values for
// non-UTF-8 binary payloads.
type journalLine struct {
	RealtimeTimestamp string          `json:"__REALTIME_TIMESTAMP"`
	Priority          string          `json:"PRIORITY"`
	SystemdUnit       string          `json:"_SYSTEMD_UNIT"`
	SyslogIdentifier  string          `json:"SYSLOG_IDENTIFIER"`
	Message           json.RawMessage `json:"MESSAGE"`
}

// parseJournalErrors parses the newline-delimited JSON produced by
// `journalctl -o json` (one JSON object per entry, not a JSON array).
func parseJournalErrors(output string) ([]JournalEntry, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var result []JournalEntry
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var jl journalLine
		if err := json.Unmarshal([]byte(line), &jl); err != nil {
			return nil, fmt.Errorf("linux: parse journal errors: %w", err)
		}

		unit := jl.SystemdUnit
		if unit == "" {
			unit = jl.SyslogIdentifier
		}

		var timestamp time.Time
		if usec, err := strconv.ParseInt(jl.RealtimeTimestamp, 10, 64); err == nil {
			timestamp = time.UnixMicro(usec).UTC()
		}

		result = append(result, JournalEntry{
			Timestamp: timestamp,
			Unit:      unit,
			Priority:  jl.Priority,
			Message:   decodeJournalMessage(jl.Message),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("linux: parse journal errors: %w", err)
	}
	return result, nil
}

// decodeJournalMessage handles both journald MESSAGE encodings: a plain
// JSON string, or (for non-UTF-8 payloads) a JSON array of byte values.
func decodeJournalMessage(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var bytes []byte
	if err := json.Unmarshal(raw, &bytes); err == nil {
		return string(bytes)
	}
	return ""
}
