package tools

import (
	"context"

	"infrastructure-mcp/internal/cisco"
)

// CiscoDiagnostics is satisfied by *cisco.Client.
type CiscoDiagnostics interface {
	Backup(ctx context.Context, target string) (string, error)
	ShowVersion(ctx context.Context, target string) (cisco.VersionInfo, error)
	Interfaces(ctx context.Context, target string) ([]cisco.InterfaceEntry, error)
	Inventory(ctx context.Context, target string) ([]cisco.InventoryEntry, error)
	Logs(ctx context.Context, target string, limit int) ([]string, error)
}
