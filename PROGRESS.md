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
| v0.1 | `internal/linux` (uptime, disk_usage, memory_usage parsers) | done |
| v0.1 | Remaining tools: run_command, uptime, disk_usage, memory_usage | done |
| v0.1 | GitHub Actions CI | done |
| **v0.1** | **Core infrastructure — complete** | **done** |
| pre-v0.7 | Legacy network device connectivity (SSH legacy crypto, Telnet transport) — user request, ahead of v0.7 schedule | in progress |
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

### 2026-07-23 — internal/linux + remaining v0.1 tools
- `internal/linux`: `Client` depends on a small `Runner` interface (`Run(ctx, server, command) (ssh.Result, error)`) rather than a concrete `*ssh.Pool`, so parsing logic is unit-tested without any network dependency. Commands used are portable across distros: `cat /proc/uptime /proc/loadavg` (uptime/load), `df -kP` (POSIX-mode disk usage, immune to line-wrapping on long device names), `cat /proc/meminfo` (memory, preferring kernel-reported `MemAvailable` and falling back to `Free+Buffers+Cached` on older kernels that lack it).
- `mcp/tools`: added `run_command` (thin pass-through to `ssh.Pool.Run`, exposes `CommandRunner` interface), `uptime`, `disk_usage`, `memory_usage` (all three share a `LinuxDiagnostics` interface satisfied by `*linux.Client`).
- Severity/recommendation logic (per the README's `check_disk()`-style tool design example) lives at the tool layer, not in `internal/linux`: `disk_usage` warns at 75% used / critical at 90%, `memory_usage` warns at 85% / critical at 95%. Thresholds are named constants in `mcp/tools/{disk_usage,memory_usage}.go`; shared via `severity.go`.
- `cmd/server/main.go` now wires the full v0.1 dependency graph: inventory → `ssh.Pool` → `linux.Client` → all five tools.
- All new tools verified through the real MCP protocol (`mcp.NewInMemoryTransports()`), including a table-driven test of each disk/memory severity threshold. 86.8% coverage on `mcp/tools`, 87.4% on `internal/linux`.
- **v0.1's tool surface (`list_servers`, `run_command`, `uptime`, `disk_usage`, `memory_usage`) is now complete.** Only GitHub Actions CI remains before v0.1 is fully done.

### 2026-07-23 — GitHub Actions CI, v0.1 complete
- `.github/workflows/ci.yml`: build, `go vet`, `gofmt -l` check, `go test -race -cover`, on push to `main` and on every PR. Uses `go-version-file: go.mod` so CI always tracks whatever Go version the module declares rather than a hardcoded version that can drift.
- Loosened `go.mod`'s `go` directive from the exact local toolchain version (`1.26.5`, an artifact of `go mod init` capturing whatever was installed) down to `1.24` per README's "Go 1.24+"; `go mod tidy` settled it at `1.25.0` (a transitive dependency's minimum). Still satisfies the README's stated floor.
- **v0.1 (core infrastructure) is now fully complete**: `list_servers`, `run_command`, `uptime`, `disk_usage`, `memory_usage`, all inventory-driven, all tested against the real MCP protocol, CI green. Next up per the roadmap is v0.2 (Linux: `failed_services`, `cpu_usage`, `reboot_required`, `running_processes`, `journal_errors`, `kernel_version`).

### 2026-07-23 — Structured error contract, per-execution logging, host-key CLI flags
Flagged by an advisor review before declaring v0.1 done: the tool surface worked but skipped two README-mandated cross-cutting requirements, and both sat on code paths the existing test suite (happy-path + severity only) never exercised.

- `internal/toolerr`: `Wrap(err)` classifies any error into `{message, recommendation, retryable, category}` (categories: `not_found`, `auth`, `network`, `timeout`, `internal`), matching README's Error Handling section verbatim. Idempotent (re-wrapping is a no-op) and preserves the `errors.Is`/`errors.As` chain to the original cause. Its `Error()` renders as JSON — the MCP SDK's default error packing calls `Error()` for the result's text content, so this is how a structured envelope survives without a manual `*mcp.CallToolResult` per tool. 94.7% coverage.
- `mcp/tools/logging.go`: generic `withLogging[In Targeted, Out any]` wraps every tool handler and emits the log README requires (tool, user, target, duration, result, error) on every call, success or failure. `target` comes from a `TargetServer() string` method added to each tool's input struct. `user` is the connecting MCP client's advertised name (`req.Session.InitializeParams().ClientInfo.Name`) — the closest real signal to "user" available; there is no auth/identity layer yet, so this is a documented approximation, not a fabrication, and should be revisited if/when one is added.
- All 5 `RegisterX` functions now take `*slog.Logger` and route handler errors through `wrapErr` (→ `toolerr.Wrap`) instead of returning raw Go errors.
- `cmd/server`: exposes `-known-hosts` and `-insecure-ignore-host-key` flags so the `ssh.Pool`'s already-implemented (but previously unreachable) `PoolOption`s are actually usable without editing code — needed for the binary to be runnable against lab/dev targets that aren't in `~/.ssh/known_hosts`.
- New tests specifically target the previously-untested error path: `internal/toolerr/toolerr_test.go` (classification table), `mcp/tools/diagnostics_test.go::TestDiagnosticsTools_ErrorPath` and `mcp/tools/run_command_test.go::TestRunCommand_ErrorPath_ReturnsStructuredEnvelope` (assert the *actual JSON on the wire* through the real MCP protocol is the structured envelope, not an error string), `mcp/tools/logging_test.go` (captures real `slog.Record`s from a live tool call and asserts on tool/user/target/result fields).

### 2026-07-23 — Legacy network device connectivity, part 1: inventory.Target + SSH legacy crypto
User request: many older switches/routers in the target environment have obsolete SSH crypto stacks (or no SSH at all — Telnet only), and none support public-key auth. Building this ahead of the v0.7 (Networking) schedule since it's foundational transport work, not vendor-specific tooling.

- `internal/inventory`: `NetworkDevice` gains `Port`, `ProxyJump`, `Protocol` (`ssh`|`telnet`, default `ssh`, validated), and `LegacyCrypto` (bool, opt-in per device, never a default). New `Target` type + `(inv *Inventory) Target(name)` unifies lookup across servers/routers/switches into one connection-level shape, so the transport layer doesn't care which inventory category a name came from. Ambiguous names (same name in two categories) are rejected with `ErrAmbiguousTarget` rather than silently picking one.
- `internal/ssh`: `Pool.dial` now resolves via `inv.Target()` instead of `inv.Server()` — this is what let routers/switches (not just Linux servers) reuse the entire existing pooling/proxyjump/host-key machinery for free. A target explicitly configured for `protocol: telnet` is rejected with a clear error if something tries to run it through `ssh.Pool` directly (the real dispatch happens one layer up — see below). New `applyLegacyCrypto()` widens the negotiated key exchange/cipher/MAC/host-key algorithm sets using `golang.org/x/crypto/ssh`'s `InsecureAlgorithms()`, additively (never replacing the modern defaults) and only when `Target.LegacyCrypto` is true — verified this API exists via `go doc` on the actual installed module rather than assuming.
- Backward compatible by construction: existing `Server`-only inventories and all prior `ssh` package tests pass unchanged, since `Target()` for a `Servers` entry reproduces exactly the old `Server`-based behavior (protocol implicitly ssh, `LegacyCrypto` false).
- Tests: `internal/inventory/target_test.go` (resolution across all 3 categories, ambiguity, default protocol, invalid protocol validation), `internal/ssh/legacy_test.go` (algorithm widening, non-mutation of package globals), `internal/ssh/pool_test.go` additions (a `Switches`-sourced target with `LegacyCrypto: true` dials successfully through the fake SSH server; a `telnet`-protocol target is refused by `ssh.Pool.Run`).
- **Still to come in this feature**: `internal/telnet` (new transport for devices with no SSH at all) and `internal/remote` (thin composer that picks SSH vs Telnet per target's `Protocol`, so `mcp/tools`/`internal/linux` keep using the same `Run(ctx, target, command)` shape they already do).

## Decisions & Deviations from a literal README reading

- **Module path**: used `infrastructure-mcp` (no VCS host) since this repo has no git remote configured yet. Rename in `go.mod` once the real module path (GitHub org/repo) is decided — this will require updating all internal import paths.
- **Server auth validation**: not enforced at the inventory layer (see above); the SSH adapter is responsible for producing a clear error if a target ends up with no usable credential at connection time.
- **Env-var expansion scope**: applied over raw file bytes before YAML parsing, so it also matches `${...}` inside comments. Documented rather than fixed with comment-stripping (adds complexity for a case unlikely to occur in real files).
- **"user" in structured logs**: no auth/identity layer exists yet, so `user` is populated from the connecting MCP client's self-reported `ClientInfo.Name` rather than an authenticated identity. Revisit when/if auth is added (not currently on the README roadmap for v0.1-v1.0).
