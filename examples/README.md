# Examples

All values in this directory are synthetic placeholders (IPs like
`10.0.0.x`, made-up hostnames) — copy the shape, fill in your own targets
and secrets, never commit the result. See the main
[README's Configuration section](../README.md#configuration) and
[`docs/TOOL_REFERENCE.md`](../docs/TOOL_REFERENCE.md) for full context.

## `mcp-client-config/`

Ready-to-adapt MCP client configuration snippets:

- [`claude_desktop_stdio.json`](mcp-client-config/claude_desktop_stdio.json) —
  local subprocess (stdio transport), the default and simplest setup.
- [`remote_http_via_mcp-remote.json`](mcp-client-config/remote_http_via_mcp-remote.json) —
  connecting a stdio-only client to a remotely-running
  `-transport=http` server through the `mcp-remote` bridge.

## `inventory-snippets/`

Focused, single-concept inventory fragments — each demonstrates one
pattern in isolation, rather than one large file mixing all of them (see
`configs/inventory.example.yaml` for the combined version):

- [`linux-server.yaml`](inventory-snippets/linux-server.yaml) — a
  key-authenticated Linux server, and one reached through a jump host.
- [`legacy-network-device.yaml`](inventory-snippets/legacy-network-device.yaml) —
  a Telnet-only router and an SSH switch with obsolete crypto
  (`legacy_crypto: true`).
- [`key-auth-network-device.yaml`](inventory-snippets/key-auth-network-device.yaml) —
  a router/switch authenticated with an SSH key instead of a password (see
  [ADR 0008](../docs/adr/0008-ssh-public-key-auth-for-network-devices.md)).
- [`multi-instance-services.yaml`](inventory-snippets/multi-instance-services.yaml) —
  two Grafana instances and two Proxmox clusters side by side, each a named
  map entry (see [ADR 0002](../docs/adr/0002-inventory-driven-targeting.md)).

## `sample-tool-outputs/`

Real (synthetic-data) JSON shapes returned by a few representative tools,
one warning/critical case and one clean case each, for quick reference
without reading Go source:

- [`disk_usage.json`](sample-tool-outputs/disk_usage.json)
- [`docker_ps.json`](sample-tool-outputs/docker_ps.json)
- [`cisco_interfaces.json`](sample-tool-outputs/cisco_interfaces.json)
- [`grafana_alerts.json`](sample-tool-outputs/grafana_alerts.json)
- [`docker_restart_confirmation_required.json`](sample-tool-outputs/docker_restart_confirmation_required.json) —
  the confirm-gate pattern every mutating tool uses.

Every tool's shape (not just these five) is documented in
[`docs/TOOL_REFERENCE.md`](../docs/TOOL_REFERENCE.md).
