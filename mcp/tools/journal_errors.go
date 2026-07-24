package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// JournalErrorsInput targets a single inventory server.
type JournalErrorsInput struct {
	Server string `json:"server" jsonschema:"the inventory server name to check"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of journal entries to return, most recent first (default 20)"`
}

// TargetServer implements Targeted.
func (in JournalErrorsInput) TargetServer() string { return in.Server }

// JournalEntryOutput describes one systemd journal entry.
type JournalEntryOutput struct {
	Timestamp string `json:"timestamp"`
	Unit      string `json:"unit,omitempty"`
	Priority  string `json:"priority"`
	Message   string `json:"message"`
}

// JournalErrorsOutput is the result of journal_errors.
type JournalErrorsOutput struct {
	Server    string               `json:"server"`
	Entries   []JournalEntryOutput `json:"entries"`
	Status    string               `json:"status"`
	Timestamp string               `json:"timestamp"`
}

// RegisterJournalErrors adds the journal_errors tool to server.
func RegisterJournalErrors(server *mcp.Server, logger *slog.Logger, diag LinuxDiagnostics) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "journal_errors",
		Description: "Report the most recent systemd journal entries at error priority or higher on a Linux server.",
	}, withLogging(logger, "journal_errors", func(ctx context.Context, req *mcp.CallToolRequest, in JournalErrorsInput) (*mcp.CallToolResult, JournalErrorsOutput, error) {
		entries, err := diag.JournalErrors(ctx, in.Server, in.Limit)
		if err != nil {
			return nil, JournalErrorsOutput{}, wrapErr(err)
		}

		out := make([]JournalEntryOutput, 0, len(entries))
		for _, e := range entries {
			out = append(out, JournalEntryOutput{
				Timestamp: e.Timestamp.Format(time.RFC3339),
				Unit:      e.Unit,
				Priority:  e.Priority,
				Message:   e.Message,
			})
		}

		status := "ok"
		if len(out) > 0 {
			status = "warning"
		}

		return nil, JournalErrorsOutput{
			Server:    in.Server,
			Entries:   out,
			Status:    status,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}))
}
