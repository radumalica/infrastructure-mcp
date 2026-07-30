# 3. Layered MCP / adapter / foundation architecture

## Status

Accepted

## Context

The server has to support a growing, heterogeneous list of integrations
(Linux, Docker, Kubernetes, Grafana, Proxmox, Cisco, and more on the
roadmap) without each one becoming a special case, and without MCP-protocol
concerns (tool schemas, structured errors, logging) leaking into
vendor-specific code, or vendor-specific auth leaking out to the MCP layer
where an agent could see it.

## Decision

Three layers, wired top-down and never skipped:

1. **MCP layer** (`mcp/tools/`) — one file per tool, registers with the MCP
   SDK, validates input, calls exactly one adapter method, and shapes the
   result through the structured error/logging contract
   ([ADR 0007](0007-structured-errors-and-confirm-gated-mutations.md)).
   Tools never talk to infrastructure directly.
2. **Adapter layer** (`internal/linux/`, `internal/docker/`,
   `internal/proxmox/`, `internal/grafana/`, `internal/cisco/`, ...) — one
   isolated package per integration, each exposing a small interface that
   the corresponding tools depend on (accept interfaces, return structs).
   Authentication (SSH keys, API tokens, kubeconfig) lives *inside* the
   adapter/foundation layer and is never exposed to the MCP layer or,
   transitively, to the agent.
3. **Foundation** — `internal/inventory/` (target resolution,
   [ADR 0002](0002-inventory-driven-targeting.md)) and the transport pools
   (`internal/ssh`, `internal/telnet`, `internal/remote`,
   [ADR 0004](0004-unified-target-abstraction-for-transport.md)).

A new integration is additive: a new adapter package, a matching set of
tools, and a few lines wiring both into `cmd/server/main.go`. Nothing in an
existing layer needs to change to add one.

## Consequences

- `cisco_backup`/`cisco_version`/etc. share zero code with
  `docker_ps`/`docker_restart` beyond the generic `withLogging` /
  `toolerr.Wrap` machinery and (where applicable) the same underlying
  `remote.Pool` — a bug or a design change in one adapter can't silently
  affect another.
- Interfaces are defined at the point of use (the tool's dependency), not
  exported speculatively from the adapter — see `internal/docker/`'s
  `Runner` interface, which `internal/remote.Pool` satisfies with zero
  adapter-specific glue because both sides agreed on
  `Run(ctx, target, command) (Result, error)`.
- The tradeoff is more small files/packages than a monolithic design would
  have — accepted deliberately (see the project's file-organization
  convention: many small, cohesive files over few large ones).
