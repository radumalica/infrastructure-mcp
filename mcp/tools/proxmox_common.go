package tools

import (
	"context"

	"infrastructure-mcp/internal/proxmox"
)

// ProxmoxDiagnostics is satisfied by *proxmox.Client.
type ProxmoxDiagnostics interface {
	ListNodes(ctx context.Context, instance string) ([]proxmox.NodeEntry, error)
	ListVMs(ctx context.Context, instance, node string) ([]proxmox.VMEntry, error)
	ListTasks(ctx context.Context, instance, node string, limit int) ([]proxmox.TaskEntry, error)
	StartVM(ctx context.Context, instance, node, guestType string, vmid int) (string, error)
	StopVM(ctx context.Context, instance, node, guestType string, vmid int) (string, error)
	Snapshot(ctx context.Context, instance, node, guestType string, vmid int, snapname string) (string, error)
}
