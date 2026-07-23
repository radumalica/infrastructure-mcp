package linux

import (
	"bufio"
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
