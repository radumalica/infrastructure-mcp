# Infrastructure MCP Skill Distribution

Infrastructure MCP uses a hybrid distribution model. The server and general
operator skill are open source; outcome-specific operational workflows are
separate paid products delivered through Agensi.

## Product Boundary

| Product | Distribution | Purpose |
|---|---|---|
| `infrastructure-mcp-operator` | Free on GitHub and Agensi | Target discovery, named-tool selection, safety boundaries, and evidence-based inspection |
| Infrastructure Incident Triage | Paid on Agensi | Cross-layer diagnosis, evidence scoring, ranked hypotheses, and escalation-ready incident reports |
| Infrastructure Health Brief | Paid on Agensi | Bounded fleet checks, severity normalization, coverage accounting, and shift-ready reports |
| Infrastructure Safe Remediation | Paid on Agensi | Approval-gated change plans, blast-radius checks, rollback guidance, and independent verification |

The premium skill source, tests, and release packages must live in a separate
private repository. They must never be copied into `.github/skills/`, public test
fixtures, public CI artifacts, GitHub releases, issues, or documentation examples
in this repository. This repository's CI deliberately fails when an additional
public skill directory appears.

## Prerequisites

1. Deploy Infrastructure MCP Server from source, Docker, or Docker Compose.
2. Configure inventory and credentials on the server side.
3. Connect the agent over stdio or authenticated Streamable HTTP.
4. Confirm the agent can see the relevant named MCP tools.

The skills cannot repair a missing server connection or adapter. They report
missing tools as coverage gaps and do not fall back to shell execution.

## Install The Free Skill From GitHub

Use the installation location supported by the agent. For agents that implement
the Agent Skills open format, copy one complete skill directory without changing
its folder name. Common project-level locations include `.github/skills/`,
`.agents/skills/`, and `.claude/skills/`.

Every GitHub release contains one `infrastructure-mcp-operator` archive. The
premium products are not attached to public GitHub releases.

## Agensi Distribution

Publish `infrastructure-mcp-operator` as a free listing whose canonical source is
this repository. Disclose its MIT license and link to its source directory. Use
it as the acquisition entry point for the paid products.

Publish the three premium products from their private source repository as
separate one-time purchases, then group them into an Agensi bundle when the
platform permits it. Initial pricing:

| Product | Launch price |
|---|---:|
| Infrastructure Incident Triage | $9 |
| Infrastructure Health Brief | $7 |
| Infrastructure Safe Remediation | $9 |
| Complete operations bundle | $19 |

Do not claim that the paid products are open source or covered by this
repository's MIT license. Their Agensi buyer licence and creator ownership terms
apply separately.

Every listing must state all of the following:

- An independently deployed and configured Infrastructure MCP Server is required.
- No hosted service, infrastructure access, inventory, credentials, API credits,
  or paid support is included.
- The currently shipped integrations are Linux, Docker, Kubernetes, Grafana,
  Proxmox, and Cisco. Roadmap integrations are not included.
- The skill uses named tools and does not require `run_command`.
- Agent compatibility is claimed only for agents in which the release was tested.

Build submission archives with:

```bash
bash scripts/package-skills.sh dev dist/skills
```

The public script builds only the free operator archive. Premium archives must be
built and validated in the private repository. Each ZIP must have exactly one
top-level folder with `SKILL.md` directly inside it and contain no binary,
inventory, credential, executable helper, or server source.

## Funnel

1. A user discovers Infrastructure MCP through GitHub or the free Agensi listing.
2. The user deploys the server and installs the free operator skill.
3. The operator skill demonstrates safe named-tool access without requiring a
   purchase.
4. Public documentation and the creator profile describe the three advanced
   outcomes without exposing their instructions.
5. Users purchase only the workflow matching their operational need, or the
   discounted bundle.
6. Measure free installs, first uses, paid conversion, refunds, support requests,
   and GitHub traffic for 30 days before changing prices.

## Security Boundary

The MCP server owns authentication and infrastructure access. Skills must never
ask users to paste credentials into a conversation. Named mutating tools preserve
their `confirm: true` execution gate, and the remediation skill requires explicit
approval of the exact action and target before passing it.

`run_command` is an unrestricted compatibility escape hatch. It is disabled by
default at the server and excluded from every skill workflow. Operators who start
the server with `-enable-run-command` accept a broader trust boundary that these
skills neither require nor use.

## Validation

Before publishing a release:

1. Run `go test ./internal/skilltest` to validate that only the free operator is
   publicly packaged and that its frontmatter, references, and fixtures are valid.
2. Run the packaging script and inspect the operator ZIP.
3. Exercise the public positive and negative prompts in
   `internal/skilltest/testdata/evaluations.yaml` in every agent named on the
   listing.
4. Test missing-server, missing-tool, healthy, incident, declined-change,
   approved-change, failed-mutation, and failed-verification scenarios.
5. Upload the exact free release artifact to Agensi and review its reported
   permissions before submitting it for human review. Validate premium artifacts
   independently in their private repository.

Agensi account creation, creator verification, listing submission, and review
remain manual because they require the repository owner's marketplace account.