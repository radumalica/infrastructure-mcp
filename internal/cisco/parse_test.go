package cisco

import (
	"strings"
	"testing"
)

const sampleShowVersion = `Cisco IOS Software, C2900 Software (C2900-UNIVERSALK9-M), Version 15.1(4)M4, RELEASE SOFTWARE (fc2)
Technical Support: http://www.cisco.com/techsupport
Copyright (c) 1986-2013 by Cisco Systems, Inc.
Compiled Wed 09-Oct-13 11:59 by prod_rel_team

ROM: System Bootstrap, Version 15.0(1r)M9, RELEASE SOFTWARE (fc1)

router1 uptime is 3 weeks, 2 days, 4 hours, 30 minutes
System returned to ROM by power-on
System restarted at 09:15:23 UTC Mon Jan 1 2024
System image file is "flash:c2900-universalk9-mz.SPA.151-4.M4.bin"
`

func TestParseVersion(t *testing.T) {
	info := parseVersion(sampleShowVersion)
	if info.VersionLine != "Cisco IOS Software, C2900 Software (C2900-UNIVERSALK9-M), Version 15.1(4)M4, RELEASE SOFTWARE (fc2)" {
		t.Errorf("unexpected VersionLine: %q", info.VersionLine)
	}
	if info.Hostname != "router1" {
		t.Errorf("Hostname = %q, want router1", info.Hostname)
	}
	if info.Uptime != "3 weeks, 2 days, 4 hours, 30 minutes" {
		t.Errorf("unexpected Uptime: %q", info.Uptime)
	}
}

const sampleShowIPIntBrief = `Interface                  IP-Address      OK? Method Status                Protocol
FastEthernet0/0             192.168.1.1     YES manual up                    up
FastEthernet0/1             unassigned      YES unset  administratively down down
Vlan1                       unassigned      YES unset  down                  down
`

func TestParseInterfaces(t *testing.T) {
	entries := parseInterfaces(sampleShowIPIntBrief)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Interface != "FastEthernet0/0" || entries[0].IPAddress != "192.168.1.1" || entries[0].Status != "up" || entries[0].Protocol != "up" {
		t.Errorf("unexpected entry[0]: %+v", entries[0])
	}
	if entries[1].Status != "administratively down" || entries[1].Protocol != "down" {
		t.Errorf("unexpected entry[1] (multi-word status): %+v", entries[1])
	}
}

// sampleShowIPIntBriefTelnetBanner reproduces the "Load for five secs" /
// "Time source is NTP" banner that some Telnet sessions echo immediately
// before the "show ip interface brief" table — these are six
// whitespace-separated tokens too, and were previously mistaken for data
// rows.
const sampleShowIPIntBriefTelnetBanner = `Load for five secs: 16%/0%; one minute: 16%; five minutes: 16%
Time source is NTP, 13:49:57.544 EEST Thu Jul 30 2026

Interface                  IP-Address      OK? Method Status                Protocol
Vlan100                     172.16.1.5      YES NVRAM  up                    up
`

func TestParseInterfaces_SkipsTelnetBanner(t *testing.T) {
	entries := parseInterfaces(sampleShowIPIntBriefTelnetBanner)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (banner lines skipped), got %d: %+v", len(entries), entries)
	}
	if entries[0].Interface != "Vlan100" {
		t.Errorf("unexpected entry[0]: %+v", entries[0])
	}
}

const sampleShowInventory = `NAME: "1", DESCR: "2911 chassis"
PID: CISCO2911/K9        , VID: V05  , SN: FTX1512Q1EF

NAME: "NIM subslot 0/0", DESCR: "Front Panel 3 ports Gigabitethernet Module"
PID: NIM-ES2-4            , VID: V01  , SN: FOC15127Q7F
`

func TestParseInventory(t *testing.T) {
	entries := parseInventory(sampleShowInventory)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Name != "1" || entries[0].PID != "CISCO2911/K9" || entries[0].SerialNumber != "FTX1512Q1EF" {
		t.Errorf("unexpected entry[0]: %+v", entries[0])
	}
	if entries[1].Description != "Front Panel 3 ports Gigabitethernet Module" || entries[1].VID != "V01" {
		t.Errorf("unexpected entry[1]: %+v", entries[1])
	}
}

const sampleShowLogging = `Syslog logging: enabled (0 messages dropped, 0 messages rate-limited)
    Console logging: level debugging, 42 messages logged
    Buffer logging: level debugging, 42 messages logged

Log Buffer (4096 bytes):
*Mar  1 00:00:12.345: %SYS-5-CONFIG_I: Configured from console by vty0
*Mar  1 00:05:01.001: %LINK-3-UPDOWN: Interface FastEthernet0/1, changed state to down
*Mar  1 00:05:02.002: %LINEPROTO-5-UPDOWN: Line protocol on Interface FastEthernet0/1, changed state to down
`

func TestParseLogs(t *testing.T) {
	all := parseLogs(sampleShowLogging, 0)
	if len(all) != 3 {
		t.Fatalf("expected 3 message lines (header skipped), got %d: %+v", len(all), all)
	}
	if !strings.Contains(all[0], "%SYS-5-CONFIG_I") {
		t.Errorf("expected header lines to be skipped, got first line: %q", all[0])
	}

	last2 := parseLogs(sampleShowLogging, 2)
	if len(last2) != 2 || !strings.Contains(last2[0], "%LINK-3-UPDOWN") || !strings.Contains(last2[1], "%LINEPROTO-5-UPDOWN") {
		t.Errorf("unexpected tail: %+v", last2)
	}
}
