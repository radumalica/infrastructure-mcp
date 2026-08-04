package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/backupstore"
)

// CiscoBackupDiffInput targets a single inventory router/switch.
type CiscoBackupDiffInput struct {
	Device string `json:"device" jsonschema:"the inventory router/switch name"`
}

// TargetServer implements Targeted.
func (in CiscoBackupDiffInput) TargetServer() string { return in.Device }

// CiscoBackupDiffOutput is the result of cisco_backup_diff.
type CiscoBackupDiffOutput struct {
	Device        string `json:"device"`
	Changed       bool   `json:"changed"`
	FirstSnapshot bool   `json:"first_snapshot"`
	Diff          string `json:"diff,omitempty"`
	Timestamp     string `json:"timestamp"`
}

// BackupDiffer persists a snapshot per target and diffs it against
// whatever was previously saved. *backupstore.Store satisfies this.
type BackupDiffer interface {
	SaveAndDiff(target, newContent string) (backupstore.Result, error)
}

// RegisterCiscoBackupDiff adds the cisco_backup_diff tool to server. It
// fetches the device's current running-config via diag (the same call
// cisco_backup makes) and hands it to store, which persists it and
// reports whether it changed since the last time this tool ran against
// the same device — a lightweight config-drift check.
func RegisterCiscoBackupDiff(server *mcp.Server, logger *slog.Logger, diag CiscoDiagnostics, store BackupDiffer) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cisco_backup_diff",
		Description: "Fetch a Cisco device's running configuration and diff it against the last snapshot taken by this tool for the same device — flags unexpected config drift. The first call for a device has nothing to diff against and just establishes the baseline.",
	}, withLogging(logger, "cisco_backup_diff", func(ctx context.Context, req *mcp.CallToolRequest, in CiscoBackupDiffInput) (*mcp.CallToolResult, CiscoBackupDiffOutput, error) {
		config, err := diag.Backup(ctx, in.Device)
		if err != nil {
			return nil, CiscoBackupDiffOutput{}, wrapErr(err)
		}

		res, err := store.SaveAndDiff(in.Device, config)
		if err != nil {
			return nil, CiscoBackupDiffOutput{}, wrapErr(err)
		}

		return nil, CiscoBackupDiffOutput{
			Device:        in.Device,
			Changed:       res.Changed,
			FirstSnapshot: res.FirstSnapshot,
			Diff:          res.Diff,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
