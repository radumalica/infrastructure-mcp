package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// FailedServicesInput targets a single inventory server.
type FailedServicesInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
}

// TargetServer implements Targeted.
func (in FailedServicesInput) TargetServer() string { return in.Server }

// FailedServiceEntry describes one systemd unit in the failed state.
type FailedServiceEntry struct {
	Unit        string `json:"unit"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description,omitempty"`
}

// FailedServicesOutput is the result of failed_services.
type FailedServicesOutput struct {
	Server         string               `json:"server"`
	Services       []FailedServiceEntry `json:"services"`
	Status         string               `json:"status"`
	Recommendation string               `json:"recommendation,omitempty"`
	Timestamp      string               `json:"timestamp"`
}

// RegisterFailedServices adds the failed_services tool to server.
func RegisterFailedServices(server *mcp.Server, logger *slog.Logger, diag LinuxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "failed_services",
		Description: "List systemd services currently in the failed state on a Linux server.",
	}, withLogging(logger, "failed_services", func(ctx context.Context, req *mcp.CallToolRequest, in FailedServicesInput) (*mcp.CallToolResult, FailedServicesOutput, error) {
		services, err := diag.FailedServices(ctx, in.Server)
		if err != nil {
			return nil, FailedServicesOutput{}, wrapErr(err)
		}

		entries := make([]FailedServiceEntry, 0, len(services))
		for _, s := range services {
			entries = append(entries, FailedServiceEntry{
				Unit:        s.Unit,
				Load:        s.Load,
				Active:      s.Active,
				Sub:         s.Sub,
				Description: s.Description,
			})
		}

		status := "ok"
		recommendation := ""
		if len(entries) > 0 {
			status = "warning"
			recommendation = "Investigate and restart or fix the failed service(s)."
		}

		return nil, FailedServicesOutput{
			Server:         in.Server,
			Services:       entries,
			Status:         status,
			Recommendation: recommendation,
			Timestamp:      time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
