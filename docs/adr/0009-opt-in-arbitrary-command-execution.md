# 9. Arbitrary command execution is opt-in

## Status

Accepted

## Context

The server originally registered `run_command` for every client. Although the
tool kept credentials inside the adapter layer, resolved targets through the
inventory, and logged each execution, its command argument was intentionally
unrestricted. That made the default MCP surface equivalent to shell execution
on every inventory target available to the connected agent and contradicted the
project's named-tool, least-privilege model.

The tool remains useful as a compatibility escape hatch while integrations are
incomplete, but compatibility does not justify granting that privilege to every
client by default.

## Decision

Do not register `run_command` unless the operator explicitly starts the server
with `-enable-run-command`. The flag defaults to false. When enabled, startup
emits a warning that connected agents can execute arbitrary commands on
inventory targets.

Named, validated tools remain the supported default. Agent skills and examples
must use them and must not assume `run_command` is available.

## Consequences

- Existing deployments that intentionally use `run_command` must add
  `-enable-run-command` when upgrading.
- Default stdio and HTTP clients cannot execute arbitrary shell commands.
- Enabling the flag is a deliberate privilege decision and must be reflected in
  the server's deployment review and MCP client trust boundary.
- The tool implementation remains tested and maintained for advanced users and
  for capabilities not yet covered by a named adapter tool.
