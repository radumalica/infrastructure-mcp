// Package tools registers MCP tools with the server. Each tool is a thin
// adapter: it validates input, delegates to an internal/* package, and
// shapes the result into structured JSON. Tools never talk to
// infrastructure directly and never expose credentials.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/inventory"
)

// ListServersInput filters the servers returned by list_servers.
type ListServersInput struct {
	Tag string `json:"tag,omitempty" jsonschema:"only return servers carrying this tag; omit to return all servers"`
}

// ServerSummary is the public, credential-free view of an inventory server.
type ServerSummary struct {
	Name     string   `json:"name" jsonschema:"the inventory name used to target this server in other tools"`
	Hostname string   `json:"hostname" jsonschema:"hostname or IP address"`
	Tags     []string `json:"tags,omitempty" jsonschema:"tags associated with this server"`
}

// ListServersOutput is the result of list_servers.
type ListServersOutput struct {
	Servers []ServerSummary `json:"servers" jsonschema:"servers matching the requested filter"`
}

// RegisterListServers adds the list_servers tool to server. inv is the
// loaded inventory to enumerate.
func RegisterListServers(server *mcp.Server, inv *inventory.Inventory) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_servers",
		Description: "List servers known to the inventory, optionally filtered by tag. Never returns credentials.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ListServersInput) (*mcp.CallToolResult, ListServersOutput, error) {
		names := inv.ServerNames(in.Tag)
		summaries := make([]ServerSummary, 0, len(names))
		for _, name := range names {
			s, err := inv.Server(name)
			if err != nil {
				// Inventory is read-only after load; a name returned by
				// ServerNames is guaranteed to resolve. Treat divergence
				// as a programming error rather than a user-facing one.
				return nil, ListServersOutput{}, err
			}
			summaries = append(summaries, ServerSummary{
				Name:     name,
				Hostname: s.Hostname,
				Tags:     s.Tags,
			})
		}
		return nil, ListServersOutput{Servers: summaries}, nil
	})
}
