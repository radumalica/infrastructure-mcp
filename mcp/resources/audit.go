// Package resources registers MCP resources (as opposed to tools) with
// the server — read-only, URI-addressed data an agent can fetch directly
// rather than calling a tool for.
package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/audit"
)

// AuditResourceURI is the URI agents read to see recent mutating tool
// activity.
const AuditResourceURI = "audit://recent"

// RegisterAuditHistory exposes log's most recent entries (see
// internal/audit) as a read-only MCP resource, so an agent can check what
// destructive/state-changing actions have already been taken this session
// — e.g. "did I already restart this container?" — before proposing
// another one.
func RegisterAuditHistory(server *mcp.Server, log *audit.Log) {
	server.AddResource(&mcp.Resource{
		URI:         AuditResourceURI,
		Name:        "audit-history",
		Description: "The most recent mutating tool invocations (docker_restart, proxmox_start_vm, proxmox_stop_vm, proxmox_snapshot) this server has handled, newest first. In-memory only — cleared on restart, not a durable audit trail.",
		MIMEType:    "application/json",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		data, err := json.Marshal(log.Recent())
		if err != nil {
			return nil, fmt.Errorf("resources: marshal audit history: %w", err)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: AuditResourceURI, MIMEType: "application/json", Text: string(data)}},
		}, nil
	})
}
