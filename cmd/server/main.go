// Command server runs the Infrastructure MCP Server over stdio.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/linux"
	"infrastructure-mcp/internal/ssh"
	"infrastructure-mcp/mcp/tools"
)

const version = "0.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	inventoryPath := flag.String("inventory", "configs/inventory.yaml", "path to the inventory YAML file")
	knownHostsPath := flag.String("known-hosts", "", "path to the SSH known_hosts file used to verify target host keys (default: $HOME/.ssh/known_hosts)")
	insecureHostKey := flag.Bool("insecure-ignore-host-key", false, "skip SSH host key verification entirely (lab/dev use only, never for production infrastructure)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	inv, err := inventory.Load(*inventoryPath)
	if err != nil {
		return fmt.Errorf("load inventory: %w", err)
	}
	logger.Info("inventory loaded",
		"path", *inventoryPath,
		"servers", len(inv.Servers),
		"routers", len(inv.Routers),
		"switches", len(inv.Switches),
	)

	var poolOpts []ssh.PoolOption
	if *knownHostsPath != "" {
		poolOpts = append(poolOpts, ssh.WithKnownHostsPath(*knownHostsPath))
	}
	if *insecureHostKey {
		logger.Warn("SSH host key verification disabled (-insecure-ignore-host-key); do not use against production infrastructure")
		poolOpts = append(poolOpts, ssh.WithInsecureIgnoreHostKey())
	}

	sshPool := ssh.NewPool(inv, poolOpts...)
	defer sshPool.Close()
	linuxClient := linux.New(sshPool)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "infrastructure-mcp",
		Version: version,
	}, nil)

	tools.RegisterListServers(server, logger, inv)
	tools.RegisterRunCommand(server, logger, sshPool)
	tools.RegisterUptime(server, logger, linuxClient)
	tools.RegisterDiskUsage(server, logger, linuxClient)
	tools.RegisterMemoryUsage(server, logger, linuxClient)

	logger.Info("starting server", "transport", "stdio")
	return server.Run(context.Background(), &mcp.StdioTransport{})
}
