package cisco

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/ssh"
)

// ErrWrongVendor indicates the named inventory target exists but isn't a
// Cisco device, so vendor-specific commands were refused rather than sent
// to a device that wouldn't understand them.
var ErrWrongVendor = errors.New("cisco: target is not a cisco device")

// ErrCommandRejected indicates IOS rejected the command in its response
// text (e.g. "% Invalid input detected", "% Authorization failed") rather
// than via a transport-level exit status. Telnet sessions never carry an
// exit code at all, and even SSH sessions to IOS routinely return exit
// code 0 for a rejected command, so this is the only reliable signal —
// checked on every command's output regardless of transport or ExitCode.
var ErrCommandRejected = errors.New("cisco: device rejected the command")

// iosErrorPrefixes are the line prefixes IOS uses to report a rejected
// command inline in its response, instead of (or in addition to) a
// non-zero exit status.
var iosErrorPrefixes = []string{
	"% invalid input detected",
	"% incomplete command",
	"% ambiguous command",
	"% authorization failed",
	"% access denied",
}

// iosRejected reports whether out contains a line IOS uses to signal a
// rejected command (e.g. insufficient privilege for "show running-config"
// without enable mode).
func iosRejected(out string) (string, bool) {
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.ToLower(strings.TrimSpace(line))
		for _, prefix := range iosErrorPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				return strings.TrimSpace(line), true
			}
		}
	}
	return "", false
}

// CommandRunner executes a command against a named inventory target.
// *remote.Pool satisfies this, matching internal/docker and
// internal/linux's pattern of running vendor commands over the shared
// SSH/Telnet dispatch layer.
type CommandRunner interface {
	Run(ctx context.Context, target, command string) (ssh.Result, error)
}

// VendorLookup resolves an inventory target's connection details,
// including its vendor. *inventory.Inventory satisfies this.
type VendorLookup interface {
	Target(name string) (inventory.Target, error)
}

// Client runs read-only diagnostic commands against Cisco IOS
// routers/switches in the inventory.
type Client struct {
	runner CommandRunner
	inv    VendorLookup
}

// New creates a Client that runs commands via runner against targets
// resolved through inv.
func New(runner CommandRunner, inv VendorLookup) *Client {
	return &Client{runner: runner, inv: inv}
}

// requireCisco returns ErrWrongVendor if target isn't a Cisco device, so a
// vendor-specific command is never sent to, say, a MikroTik router.
func (c *Client) requireCisco(target string) error {
	t, err := c.inv.Target(target)
	if err != nil {
		return err
	}
	if t.Vendor != "cisco" {
		return fmt.Errorf("%w: %q is a %q device", ErrWrongVendor, target, t.Vendor)
	}
	return nil
}

func (c *Client) run(ctx context.Context, target, cmd string) (string, error) {
	if err := c.requireCisco(target); err != nil {
		return "", err
	}
	res, err := c.runner.Run(ctx, target, cmd)
	if err != nil {
		return "", fmt.Errorf("cisco: %q on %q: %w", cmd, target, err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("cisco: %q on %q: command exited %d: %s", cmd, target, res.ExitCode, res.Stderr)
	}
	if line, rejected := iosRejected(res.Stdout); rejected {
		return "", fmt.Errorf("%w: %q on %q: %s", ErrCommandRejected, cmd, target, line)
	}
	return res.Stdout, nil
}

// Backup returns the device's running configuration ("show
// running-config"), as-is — a config dump has no meaningful structured
// form beyond its own text.
//
// This sends a single command over a one-shot session (internal/ssh and
// internal/telnet both run one command per call, with no session setup
// step), so it does NOT send "terminal length 0" first. On a device
// whose session-default page length hasn't been set to unlimited, IOS
// will paginate the response at a "--More--" prompt and this returns only
// the first page. Configure "terminal length 0" (or the per-line
// equivalent) on any device this tool is used against.
func (c *Client) Backup(ctx context.Context, target string) (string, error) {
	return c.run(ctx, target, "show running-config")
}

// ShowVersion returns the parsed result of "show version".
func (c *Client) ShowVersion(ctx context.Context, target string) (VersionInfo, error) {
	out, err := c.run(ctx, target, "show version")
	if err != nil {
		return VersionInfo{}, err
	}
	return parseVersion(out), nil
}

// Interfaces returns the parsed result of "show ip interface brief".
func (c *Client) Interfaces(ctx context.Context, target string) ([]InterfaceEntry, error) {
	out, err := c.run(ctx, target, "show ip interface brief")
	if err != nil {
		return nil, err
	}
	return parseInterfaces(out), nil
}

// Inventory returns the parsed result of "show inventory".
func (c *Client) Inventory(ctx context.Context, target string) ([]InventoryEntry, error) {
	out, err := c.run(ctx, target, "show inventory")
	if err != nil {
		return nil, err
	}
	return parseInventory(out), nil
}

// Logs returns the most recent limit lines of "show logging" (0 or
// negative means all buffered lines).
func (c *Client) Logs(ctx context.Context, target string, limit int) ([]string, error) {
	out, err := c.run(ctx, target, "show logging")
	if err != nil {
		return nil, err
	}
	return parseLogs(out, limit), nil
}
