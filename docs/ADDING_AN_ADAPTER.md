# Adding a New Adapter

This walks through the shape every integration follows — see
[ADR 0003](adr/0003-layered-mcp-adapter-foundation-architecture.md) for the
reasoning behind the three-layer split. Two worked examples exist in the
codebase to copy from:

- **SSH/Telnet-reachable device or host** (a new network vendor, a new
  Linux-adjacent tool): follow `internal/cisco` + `mcp/tools/cisco_*.go`.
- **HTTP API-based service** (a new monitoring/API integration): follow
  `internal/grafana` + `mcp/tools/grafana_*.go`.

## 1. Extend the inventory schema (if needed)

If the new integration doesn't fit an existing category
(`servers`/`routers`/`switches`/a named API-endpoint map), add a new
`map[string]YourType` field to `Inventory` in `internal/inventory/types.go`
— **always a map keyed by name**, even for a service you assume has only
one instance today (see [ADR 0002](adr/0002-inventory-driven-targeting.md)
and the Grafana/Proxmox migration it documents). Add `validate:"..."` tags
for required fields and wire the new map into `loader.go`'s `Validate()`.

If it's SSH/Telnet-reachable network gear, it likely fits the existing
`routers`/`switches` `NetworkDevice` shape as-is — just a new `vendor:`
value, no schema change required.

## 2. Write the adapter package

Under `internal/<name>/`:

- A `Client` (or similar) struct constructed with `New(...)`, taking its
  dependencies by interface, not concrete type — accept interfaces, return
  structs.
  - SSH/Telnet-based: takes an `internal/remote.Pool`-shaped interface
    (`Run(ctx, target, command) (ssh.Result, error)`) plus `*inventory.Inventory`
    if you need to look up device metadata (e.g. `Vendor`).
  - HTTP API-based: takes the inventory plus an `*http.Client` (nil-able,
    defaulting internally — see `grafana.New`), and resolves per-instance
    URL/token from the inventory by instance name.
- One method per capability, each running one command/request and parsing
  its own output. Keep parsers in a separate `parse.go` — pure functions of
  `string`/`[]byte` in, typed struct out, no I/O — so they're trivially
  unit-testable without a fake server.
- Never format unvalidated user input into a command string or URL path.
  If a tool parameter reaches a shell command (a container name, a device
  name chosen freely rather than looked up), validate it against a strict
  whitelist first — see `internal/docker/validate.go`'s
  `validateContainerRef` for the pattern, and
  [ADR 0006](adr/0006-docker-cli-over-ssh-not-engine-api.md) for why it
  matters here specifically.

## 3. Write the MCP tools

Under `mcp/tools/`, one file per tool (`<name>_<verb>.go`):

```go
// FooInput targets a single inventory device.
type FooInput struct {
    Device string `json:"device" jsonschema:"the inventory router/switch name"`
}

// TargetServer implements Targeted — used by withLogging to record what
// this call operated on.
func (in FooInput) TargetServer() string { return in.Device }

// FooOutput is the result of foo.
type FooOutput struct {
    Device    string `json:"device"`
    Timestamp string `json:"timestamp"`
    // ... tool-specific fields
}

func RegisterFoo(server *mcp.Server, logger *slog.Logger, diag FooDiagnostics) {
    mcp.AddTool(server, &mcp.Tool{
        Name:        "foo",
        Description: "One sentence, imperative, says exactly what this returns and which underlying command/endpoint it maps to.",
    }, withLogging(logger, "foo", func(ctx context.Context, req *mcp.CallToolRequest, in FooInput) (*mcp.CallToolResult, FooOutput, error) {
        result, err := diag.Foo(ctx, in.Device)
        if err != nil {
            return nil, FooOutput{}, wrapErr(err)
        }
        return nil, FooOutput{
            Device:    in.Device,
            Timestamp: time.Now().UTC().Format(time.RFC3339),
            // ...
        }, nil
    }))
}
```

Rules that aren't optional (see
[Tool Design Philosophy](../README.md#tool-design-philosophy)):

- The tool's input struct implements `Targeted` (`TargetServer() string`) —
  this is what `withLogging` uses to populate the `target` field in the
  structured log line.
- Errors always go through `wrapErr`/`toolerr.Wrap` — never return a raw Go
  error from a handler.
- Every output includes `timestamp`. Percentage/threshold-based results
  include `status` and, on warning/critical, `recommendation` — see
  `mcp/tools/severity.go`'s `severityForPercent` if your tool fits that
  shape.
- **If the action mutates state** (restarts something, starts/stops a
  guest, deletes a resource), copy the `confirm: true` gate pattern from
  `docker_restart.go` or `proxmox_stop_vm.go` exactly — don't invent a new
  shape for it. See
  [ADR 0007](adr/0007-structured-errors-and-confirm-gated-mutations.md).
- Interfaces the tool depends on (`FooDiagnostics` above) are declared in
  `mcp/tools`, next to the tool that uses them — not exported from the
  adapter package speculatively.

## 4. Wire it into `cmd/server/main.go`

Construct the adapter client alongside the existing ones, and add one
`tools.RegisterFoo(server, logger, fooClient)` call. If it's SSH/Telnet
based, reuse the existing `remotePool` — do not build a second connection
pool.

## 5. Test it

- **Adapter unit tests**: table-driven, fake `Runner`/HTTP transport, one
  test per parser edge case (multi-word fields, empty output, malformed
  input). Follow `internal/cisco/parse_test.go` or
  `internal/grafana/*_test.go`.
- **Tool-level MCP protocol tests**: at least one test per tool that drives
  the *actual* MCP protocol via `mcp.NewInMemoryTransports()` (not just
  calling the handler function directly) — see any `mcp/tools/*_test.go`
  for the pattern. Cover the success path, the structured-error path, and
  (if mutating) all three confirm states: unconfirmed, confirmed-success,
  confirmed-failure.
- Target ~100% coverage on the adapter's parsing/validation code —
  see [Testing standards](../README.md#testing-standards).
- If you have real hardware/a real instance available, verify end-to-end
  once by hand before committing — several bugs in this codebase (e.g. the
  Telnet-banner parsing bug fixed after `cisco_interfaces` was tested live
  against a real Telnet-only Cisco device) were only found this way, not by
  unit tests against hand-written fixtures.

## 6. Document it

- Add the new tools to README's [Current Features](../README.md#current-features)
  section, under the right version heading.
- Add each tool to [`docs/TOOL_REFERENCE.md`](TOOL_REFERENCE.md) with its
  parameters and a synthetic example output.
- If the change involved a genuine architectural decision (not just "add
  tool #39 following the existing pattern"), add an ADR under
  [`docs/adr/`](adr/) — see [ADR 0001](adr/0001-record-architecture-decisions.md).
- Log the work in `PROGRESS.md` under today's date, following the existing
  entries' style: what was built, what was deliberately deferred, and any
  deviation from a literal reading of the README.
