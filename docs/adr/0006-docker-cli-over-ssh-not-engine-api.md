# 6. Docker via CLI-over-SSH, not the Engine API

## Status

Accepted

## Context

Docker tooling could talk to the Docker Engine API remotely (over TLS with
client certs) or run `docker` CLI commands over an existing SSH connection
to the host. The inventory schema has no dedicated Docker category, no
API-token/TLS-cert fields for it — Docker targets are just `servers` that
happen to run the daemon.

## Decision

`internal/docker.Client` runs `docker` CLI commands over the same
`internal/remote.Pool` that `internal/linux` already uses, rather than
speaking the Engine API. `docker.Runner` has the identical
`Run(ctx, server, command) (ssh.Result, error)` shape as `linux.Runner`, so
`remote.Pool` satisfies it with zero Docker-specific adapter code.

`docker ps`/`docker images`/`docker stats --no-stream` are all run with
`--format '{{json .}}'` and parsed as newline-delimited JSON (one object
per line) rather than the human-readable table columns.

## Consequences

- Docker adds no new authentication surface: reaching a Docker host reuses
  100% of the SSH pooling/proxyjump/host-key/legacy-crypto machinery for
  free, exactly as `internal/linux` does.
- This is a deliberate exception to the general "API tokens live inside
  adapters" pattern (true for Grafana and Proxmox) — justified specifically
  because Docker here is colocated with a Linux host already reached over
  SSH, not a separately networked service with its own auth. A future
  "manage a remote Docker Swarm/registry with no SSH access" use case would
  need a genuinely different adapter, not an extension of this one.
- Any user-supplied value that reaches a shell command this way (a
  container name, for `docker_logs`/`docker_stats`/`docker_restart`) must
  be validated against a strict whitelist *before* being formatted into the
  command string — this is the project's no-shell-interpolation rule, and
  it's the one place in the Docker adapter where getting it wrong would be
  a real vulnerability, not just a bug. See
  `internal/docker/validate.go`'s `validateContainerRef` and
  `toolerr.CategoryInvalidInput`.
