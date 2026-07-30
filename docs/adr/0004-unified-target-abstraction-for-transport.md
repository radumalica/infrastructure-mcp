# 4. Unified `Target` abstraction across SSH and Telnet

## Status

Accepted

## Context

The project's original inventory shape distinguished `Server` (Linux hosts,
always SSH) from `NetworkDevice` (routers/switches). But real network gear
in the target environment doesn't cleanly separate: some devices have no SSH
server at all (Telnet only), some speak SSH with obsolete algorithm sets,
and the transport layer (`internal/ssh.Pool`) only knew how to dial a
`Server`. Building vendor-specific Cisco tooling first would have meant
duplicating pooling/proxyjump/host-key logic for a second, parallel
transport stack.

## Decision

- `internal/inventory.Target` is a single connection-level struct
  (`Hostname`, `User`, `Password`, `Key`, `Port`, `ProxyJump`, `Protocol`,
  `LegacyCrypto`, `Vendor`) that `Inventory.Target(name)` produces by
  resolving a name against `servers`, `routers`, and `switches` — the
  transport layer consumes only `Target`, never `Server`/`NetworkDevice`
  directly, so it doesn't care which inventory category a name came from.
  A name present in more than one category is a hard error
  (`ErrAmbiguousTarget`), not a silent first-match.
- `internal/remote.Pool` is a thin composer over `internal/ssh.Pool` and
  `internal/telnet.Pool`: it resolves `Target.Protocol` and dispatches to
  the right one, translating `telnet.Result` into the same `ssh.Result`
  shape callers already use. Both concrete pools are referenced through
  small interfaces (`sshRunner`/`telnetRunner`), so dispatch logic is
  unit-tested with fakes and no real sockets.
- `internal/ssh.Pool.dial` resolves via `inv.Target()`, so routers/switches
  reuse 100% of the existing connection pooling, `ProxyJump`, and host-key
  verification machinery for free — `Server`-only inventories and every
  prior `ssh` package test kept passing unchanged, since `Target()` for a
  `Servers` entry reproduces the old behavior exactly (protocol implicitly
  `ssh`, `LegacyCrypto` false).

## Consequences

- Adding Telnet support required zero changes to `mcp/tools` or
  `internal/linux`: `remote.Pool.Run` has the exact
  `(ctx, target, command) (ssh.Result, error)` signature the existing
  `CommandRunner`/`Runner` interfaces already expected.
- Telnet has no protocol-level "command finished" signal (unlike SSH's exec
  channel + exit status); `internal/telnet` infers completion via an idle
  timeout. `Result.ExitCode` is therefore always `0` and `Stderr` always
  empty for Telnet targets — a documented protocol limitation, not a bug,
  and callers that care about exit codes should prefer SSH-capable targets.
- `uptime`/`disk_usage`/`memory_usage` remain Linux-only (they shell out to
  `/proc`/`df`, meaningless on a Cisco/vendor OS) even though the transport
  now reaches network gear transparently — the unification is at the
  transport layer, not a claim that every tool applies to every target.
