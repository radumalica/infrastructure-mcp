package resources

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/audit"
)

func newAuditSession(t *testing.T, log *audit.Log) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterAuditHistory(server, log)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go func() { _, _ = server.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestRegisterAuditHistory_Empty(t *testing.T) {
	session := newAuditSession(t, audit.New(10))

	result, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: AuditResourceURI})
	if err != nil {
		t.Fatalf("ReadResource failed: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Contents))
	}

	var entries []audit.Entry
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no entries, got %+v", entries)
	}
	if result.Contents[0].MIMEType != "application/json" {
		t.Errorf("MIMEType = %q, want application/json", result.Contents[0].MIMEType)
	}
}

func TestRegisterAuditHistory_ReturnsRecordedEntriesNewestFirst(t *testing.T) {
	log := audit.New(10)
	log.Record(audit.Entry{Tool: "docker_restart", Target: "archive", Status: "restarted"})
	log.Record(audit.Entry{Tool: "proxmox_snapshot", Target: "lab", Status: "snapshot_requested"})

	session := newAuditSession(t, log)

	result, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: AuditResourceURI})
	if err != nil {
		t.Fatalf("ReadResource failed: %v", err)
	}

	var entries []audit.Entry
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 2 || entries[0].Tool != "proxmox_snapshot" || entries[1].Tool != "docker_restart" {
		t.Errorf("unexpected entries: %+v", entries)
	}
}
