# 2. Inventory-driven targeting, no hardcoded infrastructure

## Status

Accepted

## Context

An MCP server that lets an AI agent operate real infrastructure needs a hard
boundary: the agent should never be able to reach a hostname/IP it wasn't
explicitly given access to, and no credential should ever pass through the
agent's context window. Tool implementations are also the easiest place for
"just hardcode this one hostname for now" to creep in during development.

## Decision

Every tool resolves its target by **name or tag**, looked up in a single
validated `internal/inventory` YAML document (`go-playground/validator`).
No tool implementation contains an IP address, hostname, or credential.
Secrets in the YAML are `${VAR}` placeholders resolved from the process
environment at load time; a referenced variable that's unset (as opposed to
set-but-empty) fails the load closed rather than silently substituting
an empty string.

Every category (`servers`, `routers`, `switches`, `grafana`, `proxmox`, ...)
is a `map[string]T` keyed by name, not a single struct or a slice — so a
second Grafana instance or a tenth switch is just another map key, never a
schema change. (This was in fact a real fix: `grafana`/`proxmox` originally
shipped as singular fields and had to be migrated to maps once a second
instance came up — see `PROGRESS.md`, 2026-07-24.)

## Consequences

- An agent's tool call surface is `{name-or-tag, parameters}` — it never
  constructs a connection string, so a prompt injection or a model mistake
  can't make it reach infrastructure outside the inventory.
- Rotating a credential means changing an environment variable, not editing
  YAML or code, and the YAML file itself is safe to keep in version control
  history (a leaked *old* copy contains no secrets, only `${VAR}` names).
  The example file ships as `configs/inventory.example.yaml`; the real one
  (`configs/inventory.yaml`) is gitignored.
- Every new integration category must be added as a named map from the
  start, even if only one instance exists on day one — retrofitting it
  later (as happened with Grafana/Proxmox) means a schema migration.
