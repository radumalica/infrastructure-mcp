# 7. Structured errors and confirm-gated mutating tools

## Status

Accepted

## Context

An agent consuming raw Go errors (`connection refused`, a bare stack
trace) has no reliable way to decide whether to retry, ask the user for
different credentials, or give up — and a tool that mutates real
infrastructure (`docker restart`, a Proxmox VM stop) is far higher blast
radius than a read-only query, but the MCP tool-call interface makes both
look identical to the agent unless the server itself draws a line.

## Decision

**Errors.** `internal/toolerr.Wrap(err)` classifies any error into
`{message, recommendation, retryable, category}`, where `category` is one
of `not_found`, `auth`, `network`, `timeout`, `invalid_input`, `internal`.
It's idempotent (re-wrapping is a no-op), preserves the `errors.Is`/`As`
chain to the original cause, and renders as JSON from `Error()` — which is
how the MCP SDK's default error packing turns it into structured content
without a hand-built `*mcp.CallToolResult` in every tool. Every
`RegisterX` tool constructor routes handler errors through this, via a
shared `wrapErr` helper.

**Mutating actions require confirmation.** `docker_restart` was the first
destructive, *named* action (as opposed to `run_command`, which is
inherently unconstrained and therefore not gated the same way): its input
includes a `confirm bool`, and without `confirm: true` the tool returns
`status: "confirmation_required"` and does not call the adapter at all.
Every subsequent mutating tool (`proxmox_start_vm`, `proxmox_stop_vm`,
`proxmox_snapshot`) copies this exact pattern rather than inventing a new
one.

## Consequences

- An agent (or the human reading its transcript) can branch on
  `category` programmatically instead of pattern-matching error strings,
  and knows from `retryable` whether retrying the same call is worth
  attempting.
- A two-step confirm flow costs one extra round trip for every mutating
  call, in exchange for making "the agent restarted a container it
  shouldn't have" require an explicit, visible second tool call with
  `confirm: true` rather than a single ambiguous one — a deliberate
  latency-for-safety tradeoff that matches the project's "dangerous
  actions require confirmation" goal.
- Any new mutating tool (a future `kubectl_delete`, a MikroTik firewall
  change) is expected to reuse this exact `confirm: true` shape rather
  than a bespoke one, so the safety property is recognizable across the
  whole tool surface instead of tool-specific.
