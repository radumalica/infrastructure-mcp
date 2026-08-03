// Command server runs the Infrastructure MCP Server, either over stdio
// (the default, for local IDE/agent launch) or as a remote Streamable
// HTTP MCP endpoint (-transport http), so the same binary works with
// clients that spawn a local subprocess and clients that only speak
// remote MCP.
package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/backupstore"
	"infrastructure-mcp/internal/cisco"
	"infrastructure-mcp/internal/docker"
	"infrastructure-mcp/internal/grafana"
	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/kubernetes"
	"infrastructure-mcp/internal/linux"
	"infrastructure-mcp/internal/proxmox"
	"infrastructure-mcp/internal/remote"
	"infrastructure-mcp/internal/ssh"
	"infrastructure-mcp/internal/telnet"
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
	transportKind := flag.String("transport", "stdio", `MCP transport to serve: "stdio" (default, for local subprocess clients) or "http" (remote Streamable HTTP, for clients that connect over the network)`)
	httpAddr := flag.String("http-addr", ":8080", `address to listen on when -transport=http (e.g. ":8080")`)
	healthcheck := flag.Bool("healthcheck", false, "instead of starting the server, GET -healthcheck-url and exit 0/1 on success/failure; used as the container HEALTHCHECK (the distroless base image has no shell/curl to run one externally)")
	healthcheckURL := flag.String("healthcheck-url", "http://127.0.0.1:8080/healthz", "URL checked when -healthcheck is set")
	allowAnonymousHTTP := flag.Bool("allow-anonymous-http", false, "allow -transport=http to serve without a bearer token (lab/dev only — every tool, including run_command, becomes reachable to anyone who can reach the port)")
	backupDir := flag.String("backup-dir", "configs/backups", "directory cisco_backup_diff persists its per-device config snapshots in (created on first use)")
	flag.Parse()

	if *healthcheck {
		return runHealthcheck(*healthcheckURL)
	}

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
		"kubernetes", len(inv.Kubernetes),
		"grafana", len(inv.Grafana),
		"proxmox", len(inv.Proxmox),
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
	telnetPool := telnet.NewPool(inv)
	remotePool := remote.NewPool(inv, sshPool, telnetPool)
	defer func() { _ = remotePool.Close() }()
	linuxClient := linux.New(remotePool)
	dockerClient := docker.New(remotePool)
	kubeClient := kubernetes.New(inv)
	grafanaClient := grafana.New(inv, nil)
	proxmoxClient := proxmox.New(inv, nil)
	ciscoClient := cisco.New(remotePool, inv)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "infrastructure-mcp",
		Version: version,
	}, nil)

	tools.RegisterListServers(server, logger, inv)
	tools.RegisterRunCommand(server, logger, remotePool)
	tools.RegisterUptime(server, logger, linuxClient)
	tools.RegisterDiskUsage(server, logger, linuxClient)
	tools.RegisterMemoryUsage(server, logger, linuxClient)
	tools.RegisterFailedServices(server, logger, linuxClient)
	tools.RegisterCPUUsage(server, logger, linuxClient)
	tools.RegisterRebootRequired(server, logger, linuxClient)
	tools.RegisterRunningProcesses(server, logger, linuxClient)
	tools.RegisterJournalErrors(server, logger, linuxClient)
	tools.RegisterKernelVersion(server, logger, linuxClient)
	tools.RegisterDockerPs(server, logger, dockerClient)
	tools.RegisterDockerImages(server, logger, dockerClient)
	tools.RegisterDockerStats(server, logger, dockerClient)
	tools.RegisterDockerLogs(server, logger, dockerClient)
	tools.RegisterDockerRestart(server, logger, dockerClient)
	tools.RegisterDockerExec(server, logger, dockerClient)
	tools.RegisterKubectlGetPods(server, logger, kubeClient)
	tools.RegisterKubectlLogs(server, logger, kubeClient)
	tools.RegisterKubectlEvents(server, logger, kubeClient)
	tools.RegisterKubectlDescribe(server, logger, kubeClient)
	tools.RegisterKubectlNodes(server, logger, kubeClient)
	tools.RegisterKubectlExec(server, logger, kubeClient)
	tools.RegisterGrafanaAlerts(server, logger, grafanaClient)
	tools.RegisterGrafanaDashboards(server, logger, grafanaClient)
	tools.RegisterGrafanaAnnotations(server, logger, grafanaClient)
	tools.RegisterGrafanaQuery(server, logger, grafanaClient)
	tools.RegisterProxmoxNodes(server, logger, proxmoxClient)
	tools.RegisterProxmoxVMs(server, logger, proxmoxClient)
	tools.RegisterProxmoxTasks(server, logger, proxmoxClient)
	tools.RegisterProxmoxStartVM(server, logger, proxmoxClient)
	tools.RegisterProxmoxStopVM(server, logger, proxmoxClient)
	tools.RegisterProxmoxSnapshot(server, logger, proxmoxClient)
	tools.RegisterCiscoBackup(server, logger, ciscoClient)
	tools.RegisterCiscoVersion(server, logger, ciscoClient)
	tools.RegisterCiscoInterfaces(server, logger, ciscoClient)
	tools.RegisterCiscoInventory(server, logger, ciscoClient)
	tools.RegisterCiscoLogs(server, logger, ciscoClient)
	tools.RegisterCiscoBackupDiff(server, logger, ciscoClient, backupstore.New(*backupDir))

	switch *transportKind {
	case "stdio":
		logger.Info("starting server", "transport", "stdio")
		return server.Run(context.Background(), &mcp.StdioTransport{})
	case "http":
		token, err := resolveHTTPToken(os.Getenv("MCP_HTTP_TOKEN"), *allowAnonymousHTTP, *httpAddr)
		if err != nil {
			return err
		}
		if token == "" {
			logger.Warn("starting -transport=http with no bearer token (-allow-anonymous-http)", "addr", *httpAddr)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runHTTP(ctx, server, logger, *httpAddr, token)
	default:
		return fmt.Errorf("unknown -transport %q (want %q or %q)", *transportKind, "stdio", "http")
	}
}

// runHealthcheck performs a single GET against url and returns nil only
// on a 2xx response, so it can double as the container HEALTHCHECK
// command (`infrastructure-mcp -healthcheck`) on a base image with no
// shell, curl, or wget.
func runHealthcheck(url string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("healthcheck: %s returned status %d", url, resp.StatusCode)
	}
	return nil
}

// resolveHTTPToken decides the bearer token required on /mcp requests.
// It fails closed: -transport=http refuses to start without either a
// token or an explicit -allow-anonymous-http opt-out, since every
// registered tool (including run_command) is reachable over /mcp with
// no other access control.
func resolveHTTPToken(token string, allowAnonymous bool, addr string) (string, error) {
	if token == "" && !allowAnonymous {
		return "", fmt.Errorf("-transport=http requires MCP_HTTP_TOKEN to be set (every registered tool, including run_command, would otherwise be reachable to anyone who can reach %s) — set MCP_HTTP_TOKEN or pass -allow-anonymous-http to explicitly opt out for local/lab use", addr)
	}
	return token, nil
}

// requireBearerToken wraps next so requests must carry an
// "Authorization: Bearer <token>" header matching token exactly
// (compared in constant time to avoid a timing side channel).
func requireBearerToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="infrastructure-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// runHTTP serves the MCP server over Streamable HTTP
// (https://modelcontextprotocol.io/specification/2025-06-18/basic/transports),
// the remote transport most IDEs and hosted agents speak, until ctx is
// canceled. The same *mcp.Server instance is reused across sessions,
// which the SDK documents as safe: "It is OK for getServer to return the
// same server multiple times." /mcp requires token (via
// requireBearerToken) unless token is empty, i.e. -allow-anonymous-http
// was explicitly set. /healthz is never gated — it reveals nothing
// beyond process liveness and container/LB health checks need it.
func runHTTP(ctx context.Context, server *mcp.Server, logger *slog.Logger, addr string, token string) error {
	var handler http.Handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	if token != "" {
		handler = requireBearerToken(token, handler)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/mcp", handler)

	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting server", "transport", "http", "addr", addr, "endpoint", "/mcp")
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutting down http server")
		return httpServer.Shutdown(context.Background())
	}
}
