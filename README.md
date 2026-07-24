# Infrastructure MCP Server

A self-hosted [Model Context Protocol](https://modelcontextprotocol.io) server that lets AI agents (Claude, Cursor, Hermes, OpenAI Agents, ...) safely operate real infrastructure — Linux servers, Docker, Kubernetes, Proxmox, Grafana, network gear, and more — through high-level, auditable tools instead of raw shell access.

> **Status:** early, actively developed. See [Roadmap](#roadmap) for what's implemented and what's coming.

---

## Table of Contents

- [Why](#why)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [Architecture](#architecture)
- [Current Features](#current-features)
- [Installation](#installation)
- [Configuration](#configuration)
- [Connecting an AI Agent](#connecting-an-ai-agent)
- [Tool Design Philosophy](#tool-design-philosophy)
- [Security](#security)
- [Development](#development)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)
- [A Note on How This Project Is Built](#a-note-on-how-this-project-is-built)

---

## Why

AI agents are increasingly asked to help operate infrastructure — check disk space, restart a container, look at recent errors. The naive way to enable this is to hand the agent raw SSH/shell access. That's fast to build and dangerous to run: no audit trail, no input validation, no blast-radius control, and every agent has to independently learn how to safely operate every kind of system.

Infrastructure MCP Server is the alternative: a single, self-hosted service that exposes **meaningful, structured, inventory-driven tools** (`disk_usage()`, `docker_restart()`, `grafana_alerts()`) instead of `execute(command)`. Agents reason about infrastructure; this server is the only thing that actually touches it.

## Goals

- Self-hosted
- Production-ready
- Secure by default
- Extensible
- Vendor agnostic
- Infrastructure as Code friendly
- Stateless where possible
- Structured outputs only
- Human-auditable
- Open Source

## Non-Goals

The project is **not** intended to become:

- another SSH wrapper
- another Ansible replacement
- another monitoring system
- another Kubernetes operator

Instead, it provides AI-friendly infrastructure primitives built on top of those things.

---

## Architecture

```
                 AI Agent
      (Hermes / Claude / Cursor)

                    │
                    │ MCP
                    ▼

        Infrastructure MCP Server

                    │

        ┌───────────┴────────────┐

        │                        │

    Infrastructure APIs      SSH / Telnet Layer

        │                        │

        ▼                        ▼

 Proxmox                  Linux Servers

 Grafana                  Cisco / MikroTik

 Prometheus               UniFi

 Docker                   Home Assistant

 Kubernetes               Storage / Bare Metal
```

Three layers, wired top-down:

1. **MCP layer** (`mcp/tools/`, `mcp/prompts/`, `mcp/resources/`) — registers tools with the MCP SDK. Tools call adapters; they never talk to infrastructure directly.
2. **Adapter layer** (`internal/linux/`, `internal/docker/`, `internal/proxmox/`, `internal/grafana/`, ...) — one isolated package per integration, all implementing a common interface. Authentication (SSH keys, API tokens, kubeconfig) lives **inside adapters** and is never exposed to AI agents.
3. **Foundation** — `internal/inventory/` (YAML config, validated, env-var expansion for secrets) and `internal/ssh` / `internal/telnet` (pooled connections to targets, including legacy devices with obsolete crypto or no SSH at all).

**Everything is inventory-driven.** No IP addresses, hostnames, or credentials appear in tool implementations — targets are resolved by name/tag from the inventory YAML.

### Technology Stack

| Concern | Choice |
|---|---|
| Language | Go 1.24+ |
| MCP SDK | Official [Model Context Protocol Go SDK](https://github.com/modelcontextprotocol/go-sdk) |
| Configuration | YAML (`go-playground/validator`) |
| Logging | `log/slog` (structured, JSON) |
| SSH | `golang.org/x/crypto/ssh` |
| HTTP | `net/http` |
| Serialization | `encoding/json` |
| Testing | Go testing, Testcontainers, Docker Compose |
| CI | GitHub Actions |

### Project Structure

```
infrastructure-mcp/
├── cmd/
│   └── server/            # entry point (stdio MCP transport)
├── internal/
│   ├── inventory/          # YAML config, validation, secret expansion
│   ├── ssh/                 # pooled SSH connectivity
│   ├── telnet/               # legacy device transport
│   ├── remote/                # SSH/Telnet dispatch by target protocol
│   ├── linux/                  # Linux diagnostics adapter
│   ├── docker/                  # Docker CLI-over-SSH adapter
│   ├── toolerr/                  # structured error contract
│   ├── kubernetes/                # planned
│   ├── proxmox/                    # planned
│   ├── grafana/                     # planned
│   ├── prometheus/, loki/            # planned
│   ├── cisco/, mikrotik/, unifi/      # planned
│   └── homeassistant/                  # planned
├── mcp/
│   ├── tools/               # one file per MCP tool
│   ├── prompts/
│   └── resources/
├── configs/                # example inventory
├── docs/
├── examples/
├── scripts/
└── tests/
```

---

## Current Features

Tools implemented and tested against the real MCP protocol so far:

**Core (v0.1)**
- `list_servers` — list inventory targets, optionally filtered by tag
- `run_command` — run an arbitrary command against an inventory target
- `uptime` — uptime and load averages
- `disk_usage` — per-filesystem usage with warning/critical severity
- `memory_usage` — memory utilization with warning/critical severity

**Linux (v0.2)**
- `failed_services` — systemd units in the failed state
- `cpu_usage` — aggregate CPU utilization (Proxmox/KVM guest-time aware)
- `reboot_required` — pending-reboot detection (marker file + kernel comparison)
- `running_processes` — top processes by CPU usage
- `journal_errors` — recent systemd journal entries at error priority+
- `kernel_version` — running kernel release

**Docker (v0.3)**
- `docker_ps` — list containers (running or all)
- `docker_images` — list local images
- `docker_stats` — point-in-time resource usage snapshot
- `docker_logs` — recent combined stdout/stderr log lines
- `docker_restart` — restart a container (destructive; requires `confirm: true`)

**Cross-cutting, since v0.1**
- Legacy network device support: Telnet transport, SSH legacy-crypto negotiation, transparent per-target protocol dispatch
- Structured error contract on every tool (`message`, `recommendation`, `retryable`, `category`)
- Structured per-execution logging (`tool`, `user`, `target`, `duration`, `result`, `error`)
- Host-key verification fail-closed by default, with explicit opt-outs for lab use

See [Roadmap](#roadmap) for what's next.

---

## Installation

### Prerequisites

- Go 1.24 or newer
- SSH access (key- or password-based) to the servers/devices you want to manage
- An MCP-capable client (Claude Desktop, Claude Code, Cursor, or any other MCP client)

### Build from source

```bash
git clone https://github.com/<your-org>/infrastructure-mcp.git
cd infrastructure-mcp
go build -o bin/infrastructure-mcp ./cmd/server
```

### Run tests

```bash
go build ./...              # build
go vet ./...                # static checks
go test ./... -race -cover  # all tests, race detector, coverage
go test ./internal/linux -run TestName   # a single test
```

---

## Configuration

The server is entirely inventory-driven — copy the example inventory and fill in your own targets:

```bash
cp configs/inventory.example.yaml configs/inventory.yaml
```

```yaml
servers:
  archive:
    hostname: 10.0.0.5
    user: hermes
    key: ~/.ssh/archive
    tags:
      - linux
      - ethereum

  pve01:
    hostname: 10.0.0.2
    user: hermes
    proxyjump: jumpbox        # reached only via another inventory target
    tags:
      - proxmox

routers:
  core:
    hostname: 10.0.0.10
    vendor: cisco
    user: admin
    password: ${CORE_ROUTER_PASSWORD}   # resolved from the environment at load time

  legacy-edge:                # no SSH server at all — Telnet only
    hostname: 10.0.0.12
    vendor: cisco
    user: admin
    password: ${LEGACY_EDGE_PASSWORD}
    protocol: telnet

switches:
  ancient-sw:                 # obsolete SSH key exchange/cipher/MAC only
    hostname: 10.0.0.21
    vendor: cisco
    user: admin
    password: ${ANCIENT_SW_PASSWORD}
    legacy_crypto: true

grafana:
  url: https://grafana.lab.local
  token: ${GRAFANA_TOKEN}

proxmox:
  url: https://pve.lab.local:8006
  token: ${PROXMOX_TOKEN}
```

**Secrets are never written in plaintext.** Any `${VAR}` in the YAML is resolved from the process environment at load time, and load fails closed if the variable is unset — real credentials are never committed to the repo.

### Server flags

```bash
./bin/infrastructure-mcp \
  -inventory configs/inventory.yaml \
  -known-hosts ~/.ssh/known_hosts \
  -insecure-ignore-host-key=false
```

| Flag | Default | Description |
|---|---|---|
| `-inventory` | `configs/inventory.yaml` | Path to the inventory YAML file |
| `-known-hosts` | `$HOME/.ssh/known_hosts` | Path used to verify target SSH host keys |
| `-insecure-ignore-host-key` | `false` | Skip host key verification entirely — **lab/dev only, never production** |

Host key verification is **fail-closed by default**: an unrecognized target's connection is refused unless it's in `known_hosts` or you explicitly opt into insecure mode.

---

## Connecting an AI Agent

Infrastructure MCP Server speaks MCP over stdio. Point any MCP-capable client at the built binary, for example in Claude Desktop's `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "infrastructure": {
      "command": "/path/to/bin/infrastructure-mcp",
      "args": ["-inventory", "/path/to/configs/inventory.yaml"]
    }
  }
}
```

Once connected, the agent sees only the tools listed under [Current Features](#current-features) — never raw shell access, never credentials.

---

## Tool Design Philosophy

Never expose raw infrastructure:

```
execute(command)     # avoid
```

Prefer meaningful, structured actions:

```
check_disk()
check_memory()
docker_list()
grafana_alerts()
backup_switch()
```

Every tool:

- validates its inputs
- never panics
- returns structured JSON
- includes severity where relevant
- includes a recommendation on warning/critical states
- includes a timestamp

```json
{
  "status": "warning",
  "server": "archive",
  "disk_usage": 91,
  "recommendation": "Free disk space or expand storage.",
  "timestamp": "2026-07-23T19:10:00Z"
}
```

### Logging

Every tool execution produces a structured log line with:

`tool`, `user`, `duration`, `target`, `result`, `error`

### Error Handling

Tools never return a raw Go error. Every failure is shaped as:

`message`, `recommendation`, `retryable`, `category`

(`category` is one of `not_found`, `auth`, `network`, `timeout`, `invalid_input`, `internal`.)

---

## Security

- No secrets in logs
- No secrets in prompts
- No shell interpolation — any user-supplied value that reaches a remote command is validated against a strict whitelist pattern first
- Validate all inputs
- Whitelist inventory — no target is reachable unless it's named in the inventory
- Least privilege
- Read-only by default
- Dangerous actions require explicit confirmation (e.g. `docker_restart` requires `confirm: true` and is a no-op otherwise)

Found a security issue? Please report it privately rather than opening a public issue — see [Contributing](#contributing).

---

## Development

### Testing standards

- Unit tests target ~100% coverage for inventory, adapters, parsing, and validation
- Integration tests use Testcontainers / Docker Compose (Ubuntu, Alpine, Grafana, Prometheus, Loki)
- Every tool is exercised through a real MCP protocol round-trip (`mcp.NewInMemoryTransports()`), not just unit-tested in isolation

### Performance goals

- SSH/Telnet connection pooling
- Parallel execution
- Context cancellation everywhere
- Streaming responses
- Timeout and retry support

### Code conventions

- Small packages, dependency injection, no globals, prefer interfaces
- `context.Context` everywhere
- `log/slog` for logging
- Avoid reflection when possible
- Functions under ~50 lines when practical
- Structured errors, never raw ones
- Document exported functions
- Never expose credentials to the MCP layer
- Never hardcode infrastructure — always resolve through inventory

---

## Roadmap

Implemented versions are listed under [Current Features](#current-features) and tracked feature-by-feature in [`PROGRESS.md`](PROGRESS.md). Everything below is **not started**.

### v0.4 — Kubernetes
- `kubectl_get_pods`
- `kubectl_logs`
- `kubectl_events`
- `kubectl_describe`
- `kubectl_nodes`

### v0.5 — Grafana
- `grafana_alerts`
- `grafana_dashboards`
- `grafana_query`
- `grafana_annotations`

### v0.6 — Proxmox
- `proxmox_nodes`
- `proxmox_vms`
- `proxmox_tasks`
- `proxmox_start_vm`
- `proxmox_stop_vm`
- `proxmox_snapshot`

### v0.7 — Networking

Cisco: `backup`, `show version`, `interfaces`, `inventory`, `logs`
MikroTik: `backup`, `interfaces`, `routes`, `firewall`, `system resources`
UniFi: `devices`, `clients`, `AP status`, `firmware`, `statistics`

> Note: Telnet transport and SSH legacy-crypto support for these vendors already shipped ahead of schedule in v0.1 — only the vendor-specific tools above remain.

### v0.8 — Monitoring
Prometheus, Loki, Tempo, Alertmanager

### v0.9 — Home Assistant
- `entities`
- `devices`
- `automations`
- `sensors`
- `cameras`

### v1.0 — AI-oriented composite tools

Instead of calling 15 low-level tools, the agent calls one expert tool that composes them:

- `diagnose_linux()`
- `diagnose_docker()`
- `diagnose_kubernetes()`
- `diagnose_network()`
- `diagnose_storage()`
- `morning_brief()`
- `incident_summary()`
- `root_cause_analysis()`
- `backup_everything()`
- `check_infrastructure()`

### Further out

AWS, Azure, GCP, VMware, TrueNAS, Ceph, Synology, IPMI, Redfish, OpenStack, Nomad, Consul.

---

## Contributing

Contributions are welcome — this is an early-stage project and there's a lot of surface area left on the roadmap above.

1. Fork the repo and create a feature branch.
2. Follow the existing package/adapter pattern: one isolated package per integration under `internal/`, a matching set of tools under `mcp/tools/`, wired into `cmd/server/main.go`.
3. Match the existing conventions: structured errors via `internal/toolerr`, structured logging via the `withLogging` wrapper, inventory-driven targets, no shell interpolation of user input.
4. Add tests — unit tests for adapters/parsing, and at least one test that exercises new tools through the real MCP protocol.
5. Run `go build ./... && go vet ./... && gofmt -l . && go test ./... -race -cover` before opening a PR.
6. Open a pull request describing what changed and why.

For larger changes (a new adapter, a new version's worth of tools), consider opening an issue first to discuss the approach.

---

## License

Licensed under the [MIT License](LICENSE) — free to use, modify, and distribute, including commercially.

---

## A Note on How This Project Is Built

This project is **vibe-coded**: the vast majority of the implementation — code, tests, commit-by-commit progress — is written by AI (Claude Code), iterating directly against this README as the specification.

The **design and architecture are human-authored** by the repository owner: the layering (MCP → adapters → foundation), the inventory-driven philosophy, the tool-design rules, the security posture, and the version roadmap were all decided by a person before any code was written. AI implements against that spec; it doesn't set the direction.
