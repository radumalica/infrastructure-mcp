---
name: infrastructure-mcp-operator
description: 'Operates infrastructure through the named tools exposed by an existing infrastructure-mcp server. Use when the user asks to inspect servers, containers, clusters, dashboards, hypervisors, or Cisco devices through infrastructure-mcp.'
---

# Infrastructure MCP Operator

Use an already configured `infrastructure-mcp` connection to inspect real
infrastructure without handling its credentials.

## Safety Contract

1. Verify that the required named MCP tools are available before calling them.
2. Never ask the user to paste SSH keys, passwords, kubeconfigs, or API tokens.
3. Do not use `run_command`, even if available. It is an unrestricted opt-in
   escape hatch, not part of this workflow.
4. Start read-only. Do not call a mutating tool unless the user requested the
   change and explicitly approved the exact target and action.
5. Use only inventory target names and service instance names accepted by the
   tools. Never invent a hostname, address, namespace, or credential.
6. Separate observed tool output from interpretation. Mark missing evidence and
   unavailable adapters explicitly.

## Procedure

1. Identify the user's operational question and the smallest relevant domain.
2. Check the available MCP tool surface. If infrastructure-mcp is unavailable,
   stop and explain that the server must be installed and connected first.
3. Discover targets with `list_servers` when the server name or tag is unknown.
4. Select the narrowest named tools using the
   [tool routing reference](./references/workflow.md).
5. Collect read-only evidence from broad status to specific detail. Avoid
   repeated or unbounded log requests.
6. Return observations, interpretation, unknowns, and the next safe action.
7. For incident diagnosis, recurring fleet checks, or state changes, use the
   dedicated incident-triage, health-brief, or safe-remediation workflow when it
   is installed.

## Output

- **Scope:** targets and systems inspected
- **Observed:** facts returned by tools, including timestamps
- **Assessment:** conclusions supported by those facts
- **Unknowns:** missing tools, adapters, targets, or evidence
- **Next safe action:** a read-only follow-up or a separately approved change
