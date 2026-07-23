package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/toolerr"
)

// Targeted is implemented by every tool's input struct so the logging
// wrapper can report which inventory target a call was for, without
// per-tool logging boilerplate.
type Targeted interface {
	TargetServer() string
}

// withLogging wraps a tool handler to emit the structured per-execution
// log required by README.md's Logging section: tool, user, duration,
// target, result, error. "user" is the connecting MCP client's advertised
// name — the closest available notion of identity until an auth/identity
// layer is added (see PROGRESS.md).
func withLogging[In Targeted, Out any](
	logger *slog.Logger,
	toolName string,
	handler func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error),
) func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		start := time.Now()
		result, out, err := handler(ctx, req, in)
		duration := time.Since(start)

		attrs := []any{
			"tool", toolName,
			"user", clientName(req),
			"target", in.TargetServer(),
			"duration_ms", duration.Milliseconds(),
		}
		if err != nil {
			logger.Error("tool execution failed", append(attrs, "result", "error", "error", err.Error())...)
		} else {
			logger.Info("tool executed", append(attrs, "result", "ok")...)
		}

		return result, out, err
	}
}

func clientName(req *mcp.CallToolRequest) string {
	if req == nil || req.Session == nil {
		return "unknown"
	}
	params := req.Session.InitializeParams()
	if params == nil || params.ClientInfo == nil || params.ClientInfo.Name == "" {
		return "unknown"
	}
	return params.ClientInfo.Name
}

// wrapErr adapts a raw error into the structured toolerr contract. Every
// tool handler routes its error return through this before handing it
// back to the SDK.
func wrapErr(err error) error {
	return toolerr.Wrap(err)
}
