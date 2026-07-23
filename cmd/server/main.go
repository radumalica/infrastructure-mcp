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

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "infrastructure-mcp",
		Version: version,
	}, nil)

	tools.RegisterListServers(server, inv)

	logger.Info("starting server", "transport", "stdio")
	return server.Run(context.Background(), &mcp.StdioTransport{})
}
