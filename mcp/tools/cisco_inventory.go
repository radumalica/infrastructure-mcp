package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CiscoInventoryInput targets a single inventory router/switch.
type CiscoInventoryInput struct {
	Device string `json:"device" jsonschema:"the inventory router/switch name"`
}

// TargetServer implements Targeted.
func (in CiscoInventoryInput) TargetServer() string { return in.Device }

// CiscoInventoryEntrySummary describes one hardware component.
type CiscoInventoryEntrySummary struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	PID          string `json:"pid,omitempty"`
	VID          string `json:"vid,omitempty"`
	SerialNumber string `json:"serial_number,omitempty"`
}

// CiscoInventoryOutput is the result of cisco_inventory.
type CiscoInventoryOutput struct {
	Device    string                       `json:"device"`
	Entries   []CiscoInventoryEntrySummary `json:"entries"`
	Timestamp string                       `json:"timestamp"`
}

// RegisterCiscoInventory adds the cisco_inventory tool to server.
func RegisterCiscoInventory(server *mcp.Server, logger *slog.Logger, diag CiscoDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cisco_inventory",
		Description: "List a Cisco device's hardware components — chassis, modules, PSUs — with PID/VID/serial number (\"show inventory\").",
	}, withLogging(logger, "cisco_inventory", func(ctx context.Context, req *mcp.CallToolRequest, in CiscoInventoryInput) (*mcp.CallToolResult, CiscoInventoryOutput, error) {
		entries, err := diag.Inventory(ctx, in.Device)
		if err != nil {
			return nil, CiscoInventoryOutput{}, wrapErr(err)
		}

		summaries := make([]CiscoInventoryEntrySummary, 0, len(entries))
		for _, e := range entries {
			summaries = append(summaries, CiscoInventoryEntrySummary{
				Name:         e.Name,
				Description:  e.Description,
				PID:          e.PID,
				VID:          e.VID,
				SerialNumber: e.SerialNumber,
			})
		}

		return nil, CiscoInventoryOutput{
			Device:    in.Device,
			Entries:   summaries,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
