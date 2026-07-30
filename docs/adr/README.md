# Architecture Decision Records

This directory records the significant architectural decisions made in this
project, in the lightweight [MADR](https://adr.github.io/madr/)-style format:
Context, Decision, Consequences. They exist so a future contributor (human or
AI) can see *why* the code is shaped the way it is, not just *what* it does —
the same reasoning already logged chronologically in
[`PROGRESS.md`](../../PROGRESS.md), pulled out here per-topic for easier
reference.

An ADR is a record of a decision at the time it was made. If circumstances
change, add a new ADR that supersedes the old one (linking back to it) rather
than editing history.

| ADR | Title |
|---|---|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions |
| [0002](0002-inventory-driven-targeting.md) | Inventory-driven targeting, no hardcoded infrastructure |
| [0003](0003-layered-mcp-adapter-foundation-architecture.md) | Layered MCP / adapter / foundation architecture |
| [0004](0004-unified-target-abstraction-for-transport.md) | Unified `Target` abstraction across SSH and Telnet |
| [0005](0005-opt-in-legacy-ssh-crypto.md) | Opt-in legacy SSH crypto, never a default |
| [0006](0006-docker-cli-over-ssh-not-engine-api.md) | Docker via CLI-over-SSH, not the Engine API |
| [0007](0007-structured-errors-and-confirm-gated-mutations.md) | Structured errors and confirm-gated mutating tools |
| [0008](0008-ssh-public-key-auth-for-network-devices.md) | SSH public-key auth for routers/switches |
