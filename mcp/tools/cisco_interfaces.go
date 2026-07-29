package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CiscoInterfacesInput targets a single inventory router/switch.
type CiscoInterfacesInput struct {
	Device string `json:"device" jsonschema:"the inventory router/switch name"`
}

// TargetServer implements Targeted.
func (in CiscoInterfacesInput) TargetServer() string { return in.Device }

// CiscoInterfaceSummary describes one interface's brief status.
type CiscoInterfaceSummary struct {
	Interface string `json:"interface"`
	IPAddress string `json:"ip_address"`
	OK        string `json:"ok"`
	Method    string `json:"method"`
	Status    string `json:"status"`
	Protocol  string `json:"protocol"`
}

// CiscoInterfacesOutput is the result of cisco_interfaces.
type CiscoInterfacesOutput struct {
	Device     string                  `json:"device"`
	Interfaces []CiscoInterfaceSummary `json:"interfaces"`
	Timestamp  string                  `json:"timestamp"`
}

// RegisterCiscoInterfaces adds the cisco_interfaces tool to server.
func RegisterCiscoInterfaces(server *mcp.Server, logger *slog.Logger, diag CiscoDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cisco_interfaces",
		Description: "List a Cisco device's interfaces with IP address, admin/link status, and line protocol state (\"show ip interface brief\").",
	}, withLogging(logger, "cisco_interfaces", func(ctx context.Context, req *mcp.CallToolRequest, in CiscoInterfacesInput) (*mcp.CallToolResult, CiscoInterfacesOutput, error) {
		entries, err := diag.Interfaces(ctx, in.Device)
		if err != nil {
			return nil, CiscoInterfacesOutput{}, wrapErr(err)
		}

		summaries := make([]CiscoInterfaceSummary, 0, len(entries))
		for _, e := range entries {
			summaries = append(summaries, CiscoInterfaceSummary{
				Interface: e.Interface,
				IPAddress: e.IPAddress,
				OK:        e.OK,
				Method:    e.Method,
				Status:    e.Status,
				Protocol:  e.Protocol,
			})
		}

		return nil, CiscoInterfacesOutput{
			Device:     in.Device,
			Interfaces: summaries,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
