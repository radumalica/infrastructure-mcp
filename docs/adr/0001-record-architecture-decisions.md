# 1. Record architecture decisions

## Status

Accepted

## Context

`PROGRESS.md` already logs every implementation session chronologically,
including the reasoning behind non-obvious choices. That's the right format
for "what happened and when," but it's a poor fit for "why does the codebase
look like this" — answering that means scanning a 600+ line log for the one
entry that matters.

## Decision

Maintain a small set of topic-scoped Architecture Decision Records under
`docs/adr/`, using a lightweight Context/Decision/Consequences format. Each
ADR is extracted from, and cites, the `PROGRESS.md` entry it originates from,
rather than being a separate source of truth.

## Consequences

- A contributor asking "why does Docker go over SSH instead of the Engine
  API" or "why is legacy SSH crypto opt-in" has one short, named document to
  read instead of grepping the progress log.
- ADRs are not re-derived speculatively — one is added when a decision is
  made (or, for this initial set, backfilled from decisions already logged
  in `PROGRESS.md`), not invented to make the directory look complete.
- Superseding a decision means adding a new ADR that links to the old one,
  not editing it — `docs/adr/` stays an honest history.
