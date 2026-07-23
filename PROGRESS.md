# Implementation Progress

Single source of truth for project direction: `README.md`.
This file is the single progress log — updated after each completed feature, with a git commit to match.

## Status Summary

| Version | Scope | Status |
|---|---|---|
| Scaffold | go.mod, directory layout | done |
| Scaffold | `internal/inventory` (YAML load, env-var secrets, validation) | done |
| v0.1 | `internal/ssh` (client, connection pooling, exec) | in progress |
| v0.1 | `internal/linux` (uptime, disk_usage, memory_usage parsers) | pending |
| v0.1 | MCP server wiring + tools: list_servers, run_command, uptime, disk_usage, memory_usage | pending |
| v0.1 | Example inventory config | pending |
| v0.1 | GitHub Actions CI | pending |
| v0.2+ | Linux extended tools | not started |
| v0.3+ | Docker | not started |
| v0.4+ | Kubernetes | not started |
| v0.5+ | Grafana | not started |
| v0.6+ | Proxmox | not started |
| v0.7+ | Networking (Cisco/MikroTik/UniFi) | not started |
| v0.8+ | Monitoring (Prometheus/Loki/Alertmanager) | not started |
| v0.9+ | Home Assistant | not started |
| v1.0 | AI-oriented composite tools | not started |

## Log

### 2026-07-23 — Scaffold
- `go mod init infrastructure-mcp` (module path is a placeholder; no GitHub remote exists yet — update once the real repo location is known).
- Created directory layout per README: `cmd/server`, `internal/{inventory,ssh,linux}`, `mcp/{tools,prompts,resources}`, `configs`, `docs`, `examples`, `scripts`, `tests`.
- Added `.gitignore` (binaries, coverage output, local inventory files with secrets).

### 2026-07-23 — internal/inventory
- `Inventory`, `Server`, `NetworkDevice`, `ServiceEndpoint` types matching the YAML shape in the README's inventory example.
- `Load`/`Parse`: strict YAML decoding (`KnownFields(true)` — typos in inventory files fail loudly instead of being silently ignored), `${VAR}` environment-variable expansion for secrets (fails closed on unset vars), struct validation via go-playground/validator.
- Deliberately did **not** require `key`/`password` per server: the README's own example (`pve01`) reaches its target only via `proxyjump`, so auth-method presence is left to the SSH layer (agent auth is a valid fallback).
- Lookup helpers: `Server(name)` (returns `ErrNotFound`), `ServerNames(tag)` (sorted, optionally tag-filtered).
- Tests: `internal/inventory/loader_test.go`, 90.3% statement coverage. Covers: valid parse, missing env var, missing required field, unknown field rejection, malformed YAML, lookup hit/miss, tag filtering, file-not-found.

## Decisions & Deviations from a literal README reading

- **Module path**: used `infrastructure-mcp` (no VCS host) since this repo has no git remote configured yet. Rename in `go.mod` once the real module path (GitHub org/repo) is decided — this will require updating all internal import paths.
- **Server auth validation**: not enforced at the inventory layer (see above); the SSH adapter is responsible for producing a clear error if a target ends up with no usable credential at connection time.
