package cisco

import (
	"regexp"
	"strings"
)

// versionLinePattern matches the first line of "show version" output,
// e.g. `Cisco IOS Software, C2900 Software (C2900-UNIVERSALK9-M),
// Version 15.1(4)M4, RELEASE SOFTWARE (fc2)`.
var versionLinePattern = regexp.MustCompile(`(?i)^Cisco IOS.*Software.*$`)

// uptimeLinePattern matches IOS's stable "<hostname> uptime is <duration>"
// line, present in "show version" since the command's introduction.
var uptimeLinePattern = regexp.MustCompile(`^(\S+) uptime is (.+)$`)

// parseVersion extracts the handful of fields from "show version" that
// have been stable across IOS releases for decades: the software banner
// line and the "<hostname> uptime is ..." line.
func parseVersion(out string) VersionInfo {
	var info VersionInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if info.VersionLine == "" && versionLinePattern.MatchString(line) {
			info.VersionLine = line
			continue
		}
		if m := uptimeLinePattern.FindStringSubmatch(line); m != nil {
			info.Hostname = m[1]
			info.Uptime = m[2]
		}
	}
	return info
}

// interfaceLinePattern matches one data row of "show ip interface brief":
// Interface, IP-Address, OK?, Method, Status (one or two words, e.g.
// "administratively down"), Protocol.
var interfaceLinePattern = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.+?)\s+(\S+)$`)

// parseInterfaces parses "show ip interface brief" into one entry per
// interface, skipping the header row.
func parseInterfaces(out string) []InterfaceEntry {
	var entries []InterfaceEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Interface") {
			continue
		}
		m := interfaceLinePattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entries = append(entries, InterfaceEntry{
			Interface: m[1],
			IPAddress: m[2],
			OK:        m[3],
			Method:    m[4],
			Status:    m[5],
			Protocol:  m[6],
		})
	}
	return entries
}

// inventoryNamePattern matches the first line of a "show inventory"
// block: `NAME: "1", DESCR: "2911 chassis"`.
var inventoryNamePattern = regexp.MustCompile(`^NAME:\s*"(.*?)"\s*,\s*DESCR:\s*"(.*?)"\s*$`)

// inventoryIDPattern matches the second line of a "show inventory"
// block: `PID: CISCO2911/K9        , VID: V05  , SN: FTX1512Q1EF`.
var inventoryIDPattern = regexp.MustCompile(`^PID:\s*(\S*)\s*,\s*VID:\s*(\S*)\s*,\s*SN:\s*(\S*)\s*$`)

// parseInventory parses "show inventory"'s repeated NAME/DESCR + PID/VID/SN
// two-line blocks into one entry per hardware component.
func parseInventory(out string) []InventoryEntry {
	var entries []InventoryEntry
	var current *InventoryEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if m := inventoryNamePattern.FindStringSubmatch(line); m != nil {
			if current != nil {
				entries = append(entries, *current)
			}
			current = &InventoryEntry{Name: m[1], Description: m[2]}
			continue
		}
		if current == nil {
			continue
		}
		if m := inventoryIDPattern.FindStringSubmatch(line); m != nil {
			current.PID, current.VID, current.SerialNumber = m[1], m[2], m[3]
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}
	return entries
}

// logMessagePattern matches an IOS syslog message's "%FACILITY-SEVERITY-
// MNEMONIC:" marker (e.g. "%SYS-5-CONFIG_I:"), present on every logged
// message regardless of whether "service timestamps log" is configured.
// Used to skip "show logging"'s config-summary header block ("Syslog
// logging: enabled ...", "Buffer logging: level ...", etc.), which would
// otherwise count against limit and crowd out actual messages.
var logMessagePattern = regexp.MustCompile(`%[A-Z0-9_]+-\d+-[A-Z0-9_]+:`)

// parseLogs splits "show logging" output into non-empty message lines
// (skipping the leading config-summary header), returning only the last
// limit lines in their original oldest-to-newest order (limit <= 0 means
// return every message line).
func parseLogs(out string, limit int) []string {
	var lines []string
	seenMessage := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !seenMessage {
			if !logMessagePattern.MatchString(line) {
				continue
			}
			seenMessage = true
		}
		lines = append(lines, line)
	}
	if limit > 0 && len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return lines
}
