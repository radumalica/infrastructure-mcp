# Agent Skills

This repository ships one agent skill, `infrastructure-mcp-operator`. It is free
and MIT-licensed like the rest of the repository.

## What It Does

The skill teaches a connected agent to inspect infrastructure through the named
tools of an already running Infrastructure MCP Server: discover targets, choose
the narrowest relevant tool, collect read-only evidence, and report observations
separately from interpretation. It never asks for credentials and never uses the
opt-in `run_command`, `docker_exec` or `kubectl_exec` tools.

## Prerequisites

1. Deploy Infrastructure MCP Server from source, Docker, or Docker Compose.
2. Configure inventory and credentials on the server side.
3. Connect the agent over stdio or authenticated Streamable HTTP.
4. Confirm the agent can see the relevant named MCP tools.

The skill cannot repair a missing server connection or adapter. It reports
missing tools as coverage gaps and does not fall back to shell execution.

## Install

Use the location your agent supports. For agents that implement the Agent Skills
open format, copy the complete `infrastructure-mcp-operator` directory without
renaming it. Common project-level locations include `.github/skills/`,
`.agents/skills/`, and `.claude/skills/`. Every GitHub release also attaches an
`infrastructure-mcp-operator-<version>.zip` archive.

## Security Boundary

The MCP server owns authentication and infrastructure access. The skill must
never ask users to paste credentials into a conversation. Mutating tools keep
their `confirm: true` gate, and the skill requires explicit user approval of the
exact action and target before passing it. `run_command`, `docker_exec` and
`kubectl_exec` are disabled by default (see ADR 0009) and are excluded from the
skill's workflow.

## Packaging And Validation

Build the archive with:

```bash
bash scripts/package-skills.sh dev dist/skills
```

The ZIP has exactly one top-level folder with `SKILL.md` directly inside it and
contains no binary, inventory, credential, executable helper, or server source.

Before a release:

1. Run `go test ./internal/skilltest`. It validates the skill's frontmatter,
   references, safety phrases and fixtures, and fails if any other skill
   directory appears in `.github/skills/`.
2. Run the packaging script and inspect the ZIP.
3. Exercise the prompts in `internal/skilltest/testdata/evaluations.yaml` in each
   agent you claim compatibility with.
