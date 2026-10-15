package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/inventory"
)

// CheckInventoryHealthInput takes no target — it validates the whole
// inventory file, not one entry in it.
type CheckInventoryHealthInput struct{}

// TargetServer implements Targeted; this tool has no single target.
func (CheckInventoryHealthInput) TargetServer() string { return "" }

// CheckInventoryHealthOutput reports every problem found in one pass,
// not just the first — an operator fixing a broken inventory wants the
// whole list, not one error per rerun.
type CheckInventoryHealthOutput struct {
	Status     string   `json:"status"`
	Problems   []string `json:"problems,omitempty"`
	Servers    int      `json:"servers"`
	Routers    int      `json:"routers"`
	Switches   int      `json:"switches"`
	Grafana    int      `json:"grafana"`
	Proxmox    int      `json:"proxmox"`
	Kubernetes int      `json:"kubernetes"`
	Timestamp  string   `json:"timestamp"`
}

// RegisterCheckInventoryHealth adds the check_inventory_health tool to
// server. inventoryPath is re-read and re-parsed from disk on every call
// rather than reusing the *inventory.Inventory every other tool shares —
// deliberately, so this tool catches drift since the server started: an
// environment variable that got unset, an inventory file edited but not
// yet reloaded (a restart is required for edits to take effect; this at
// least confirms they'd succeed), or an SSH key/kubeconfig file rotated
// out from under a still-valid-looking path.
func RegisterCheckInventoryHealth(server *mcp.Server, logger *slog.Logger, inventoryPath string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_inventory_health",
		Description: "Validate the inventory file: it parses and passes structural validation, every referenced environment variable (${VAR}) is set, and every SSH key / kubeconfig file it references exists and is readable. Re-reads the inventory file from disk on every call, so it reflects the file's current state — including drift since the server started — not just what's currently loaded in memory. Read-only, takes no arguments.",
	}, withLogging(logger, "check_inventory_health", func(ctx context.Context, req *mcp.CallToolRequest, in CheckInventoryHealthInput) (*mcp.CallToolResult, CheckInventoryHealthOutput, error) {
		out := CheckInventoryHealthOutput{Timestamp: time.Now().UTC().Format(time.RFC3339)}

		inv, err := inventory.Load(inventoryPath)
		if err != nil {
			out.Status = "unhealthy"
			out.Problems = []string{err.Error()}
			return nil, out, nil
		}

		out.Servers = len(inv.Servers)
		out.Routers = len(inv.Routers)
		out.Switches = len(inv.Switches)
		out.Grafana = len(inv.Grafana)
		out.Proxmox = len(inv.Proxmox)
		out.Kubernetes = len(inv.Kubernetes)

		var problems []string
		for name, s := range inv.Servers {
			problems = append(problems, checkKeyReadable("server", name, s.Key)...)
		}
		for name, r := range inv.Routers {
			problems = append(problems, checkKeyReadable("router", name, r.Key)...)
		}
		for name, sw := range inv.Switches {
			problems = append(problems, checkKeyReadable("switch", name, sw.Key)...)
		}
		for name, k := range inv.Kubernetes {
			if err := checkFileReadable(k.Kubeconfig); err != nil {
				problems = append(problems, fmt.Sprintf("kubernetes %q kubeconfig %q: %s", name, k.Kubeconfig, err))
			}
		}

		out.Status = "healthy"
		if len(problems) > 0 {
			out.Status = "unhealthy"
			out.Problems = problems
		}

		return nil, out, nil
	}))
}

// checkKeyReadable is a no-op (returns nil) when key is unset — most
// servers/routers/switches authenticate with a password or an SSH agent
// instead, so an unset Key is not itself a problem.
func checkKeyReadable(kind, name, key string) []string {
	if key == "" {
		return nil
	}
	if err := checkFileReadable(key); err != nil {
		return []string{fmt.Sprintf("%s %q key %q: %s", kind, name, key, err)}
	}
	return nil
}

// checkFileReadable expands a leading "~" the same way internal/ssh's
// expandHome does for SSH key paths, then confirms the file opens for
// reading — the same operation the real connection path performs, so this
// check reflects what would actually happen at connect time.
func checkFileReadable(path string) error {
	expanded, err := expandHomeForHealthCheck(path)
	if err != nil {
		return err
	}
	f, err := os.Open(expanded)
	if err != nil {
		return err
	}
	return f.Close()
}

func expandHomeForHealthCheck(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}
