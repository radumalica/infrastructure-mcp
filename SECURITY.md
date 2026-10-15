# Security Policy

Infrastructure MCP Server sits between AI agents and real infrastructure, so
security reports are taken seriously.

## Reporting a vulnerability

**Do not open a public issue for a security problem.**

Report it privately through GitHub:
[Report a vulnerability](https://github.com/radumalica/infrastructure-mcp/security/advisories/new).

Please include the affected version or commit, a description of the impact, and
steps to reproduce. Do not include real credentials, hostnames, or inventory
data; use synthetic values.

You can expect an acknowledgement within a few days. Fixes for confirmed issues
are released as a patch version and credited in the advisory unless you ask
otherwise.

## Supported versions

Only the latest release receives security fixes. The project is pre-1.0.

## Scope

In scope: bypasses of the inventory allow-list, input-validation or `confirm`
gate bypasses, credential or secret exposure in tool output or logs, HTTP
transport authentication issues, and vulnerabilities in shipped dependencies
that are reachable from the server.

Out of scope: risks of deliberately enabling `-enable-run-command` (it grants
agents arbitrary command execution by design, see ADR 0009), running with
`-allow-anonymous-http` or insecure host-key mode on an untrusted network, and
issues in your own inventory, network, or client configuration.

## Hardening checklist for operators

- Keep `run_command`, `docker_exec`, and `kubectl_exec` disabled unless needed.
- Set `MCP_HTTP_TOKEN` and put the HTTP endpoint behind TLS and network access
  control.
- Use a dedicated least-privilege account on every target.
- Keep `known_hosts` verification on; never use insecure host-key mode outside a
  lab.
- Treat tool output (logs, device configs) as sensitive data.
