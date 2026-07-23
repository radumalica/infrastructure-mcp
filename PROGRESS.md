# Implementation Progress

Single source of truth for project direction: `README.md`.
This file is the single progress log — updated after each completed feature, with a git commit to match.

## Status Summary

| Version | Scope | Status |
|---|---|---|
| Scaffold | go.mod, directory layout | done |
| Scaffold | `internal/inventory` (YAML load, env-var secrets, validation) | done |
| v0.1 | MCP server skeleton (`cmd/server`) + `list_servers` tool | done |
| v0.1 | Example inventory config | done |
| v0.1 | `internal/ssh` (client, connection pooling, exec) | done |
| v0.1 | `internal/linux` (uptime, disk_usage, memory_usage parsers) | pending |
| v0.1 | Remaining tools: run_command, uptime, disk_usage, memory_usage | pending |
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

### 2026-07-23 — MCP server skeleton + list_servers
- Pulled the official Go SDK API from Context7 (`github.com/modelcontextprotocol/go-sdk`, pinned `v1.0.0`) before building against assumptions — registration pattern is `mcp.AddTool(server, &mcp.Tool{...}, handler)` with `In`/`Out` struct types driving JSON-schema inference.
- `cmd/server/main.go`: loads inventory from `-inventory` flag (default `configs/inventory.yaml`), logs structured startup info via `slog` (JSON handler, stderr — stdout is reserved for the MCP stdio transport), runs the server over `mcp.StdioTransport{}`.
- `mcp/tools/list_servers.go`: first MCP tool. Optional `tag` filter, returns name/hostname/tags only — no credentials ever leave the inventory layer.
- `configs/inventory.example.yaml`: copy of the README's example inventory, safe to commit (secrets are `${ENV_VAR}` placeholders).
- Verified end-to-end, not just "it compiles": `mcp/tools/list_servers_test.go` drives the tool through a real client/server pair over `mcp.NewInMemoryTransports()` (actual MCP protocol round-trip), and the built binary was smoke-tested as a subprocess with the example inventory. 88.9% coverage on `mcp/tools`.
- Found and fixed a real bug during the smoke test: env-var expansion runs over raw YAML bytes before parsing, so a literal `${...}` inside a YAML *comment* also gets matched and required. Fixed by rewording the example file's comment; documented as a known limitation rather than special-cased in the expander (real inventories are unlikely to write `${...}` in comments).

### 2026-07-23 — internal/ssh
- `Pool`: connection cache keyed by inventory server name (`map[string]*ssh.Client`, mutex-guarded), `Run(ctx, server, command)` returning stdout/stderr/exit code/duration, `Close()`.
- Auth precedence: explicit key → explicit password → running SSH agent (`SSH_AUTH_SOCK`) → `ErrNoCredentials`. Matches the inventory-layer decision to not require key/password (see below).
- ProxyJump is resolved recursively (`Pool.dial`), reusing the proxy's already-pooled connection when present, with cycle detection so a misconfigured inventory can't hang the server.
- Host key verification is **fail-closed by default**: connects fail with `ErrNoHostKeyVerification` unless the target is in `known_hosts` (default `~/.ssh/known_hosts`, overridable) or `WithInsecureIgnoreHostKey()` was explicitly passed (lab/dev opt-in only, never the default).
- Context cancellation is honored mid-command (`ssh.SIGKILL` sent to the remote process, `ctx.Err()` returned) rather than only before dialing.
- Tests exercise a real network path: `testserver_test.go` runs a minimal in-process SSH server (ed25519 host key, password auth, exec channel) so `pool_test.go` drives actual TCP + SSH handshake + command exec/exit-status round trips — not mocked. This avoids a Docker/Testcontainers dependency for what is fundamentally client-logic testing; Testcontainers-based integration tests (per README) are still the right tool for later multi-service scenarios (Docker, Kubernetes, Grafana). 81.2% coverage.

## Decisions & Deviations from a literal README reading

- **Module path**: used `infrastructure-mcp` (no VCS host) since this repo has no git remote configured yet. Rename in `go.mod` once the real module path (GitHub org/repo) is decided — this will require updating all internal import paths.
- **Server auth validation**: not enforced at the inventory layer (see above); the SSH adapter is responsible for producing a clear error if a target ends up with no usable credential at connection time.
- **Env-var expansion scope**: applied over raw file bytes before YAML parsing, so it also matches `${...}` inside comments. Documented rather than fixed with comment-stripping (adds complexity for a case unlikely to occur in real files).
