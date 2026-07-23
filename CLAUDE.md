# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Status

This is a **greenfield project**. The repository currently contains only `README.md` — the full technical architecture and development plan. No Go code, `go.mod`, or CI exists yet. Read `README.md` before implementing anything; it is the authoritative spec.

## What This Project Is

A self-hosted Infrastructure MCP Server in **Go 1.24+** using the **official Model Context Protocol Go SDK**. It sits between AI agents (Claude, Cursor, Hermes, etc.) and infrastructure (Linux/SSH, Docker, Kubernetes, Proxmox, Grafana, Prometheus, Cisco, MikroTik, UniFi, Home Assistant, ...), exposing safe, high-level MCP tools instead of raw shell access.

Explicit non-goals: it is not an SSH wrapper, Ansible replacement, monitoring system, or Kubernetes operator.

## Commands

No build tooling exists yet. Once code is bootstrapped, standard Go commands apply:

```bash
go build ./...              # build
go test ./...               # all tests
go test ./internal/inventory -run TestName   # single test
go vet ./...                # static checks
```

Integration tests are planned via Testcontainers and Docker Compose (Ubuntu, Alpine, Grafana, Prometheus, Loki containers). CI is GitHub Actions.

## Architecture

Three layers, wired top-down:

1. **MCP layer** (`mcp/tools/`, `mcp/prompts/`, `mcp/resources/`) — registers tools with the MCP SDK. Tools call adapters; they never talk to infrastructure directly.
2. **Adapter layer** (`internal/linux/`, `internal/docker/`, `internal/proxmox/`, `internal/grafana/`, `internal/cisco/`, etc.) — one isolated package per integration, all implementing a common interface. Authentication (SSH keys, API tokens, kubeconfig) lives **inside adapters** and is never exposed to AI agents.
3. **Foundation** — `internal/inventory/` (YAML config, validated with go-playground/validator, env-var expansion for secrets like `${GRAFANA_TOKEN}`) and `internal/ssh/` (golang.org/x/crypto/ssh, connection pooling planned). Entry point is `cmd/server/`.

**Everything is inventory-driven.** No IP addresses, hostnames, or credentials in tool implementations — targets are resolved by name/tag from the inventory YAML.

### Tool design rules

- Expose meaningful actions (`check_disk()`, `docker_ps()`, `grafana_alerts()`), never `execute(command)`.
- Every tool: validate inputs, never panic, return structured JSON with `status`/severity, `recommendation`, and `timestamp`.
- Errors are structured (`message`, `recommendation`, `retryable`, `category`) — never raw Go errors.
- Every tool execution emits structured slog logs (tool, user, duration, target, result, error) with no secrets.
- Read-only by default; dangerous actions require confirmation.

### Roadmap order

Versions build incrementally: 0.1 core (list_servers, run_command, uptime, disk/memory) → 0.2 Linux → 0.3 Docker → 0.4 Kubernetes → 0.5 Grafana → 0.6 Proxmox → 0.7 networking (Cisco/MikroTik/UniFi) → 0.8 monitoring (Prometheus/Loki/Alertmanager) → 0.9 Home Assistant → 1.0 high-level "expert" tools (`diagnose_linux()`, `morning_brief()`, `incident_summary()`) that compose the low-level ones.

## Development Rules (from README)

- Small packages, dependency injection, no globals, prefer interfaces
- `context.Context` everywhere; timeout, cancellation, and retry support
- `slog` for logging; `encoding/json` for output; `net/http` for HTTP clients
- Functions under ~50 lines when practical; document exported functions
- No shell interpolation; whitelist targets via inventory; least privilege
- Unit tests target 100% coverage for inventory, adapters, parsing, and validation
