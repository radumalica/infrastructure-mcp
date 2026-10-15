# Contributing

Thanks for helping. This project is early-stage and there is a lot of roadmap
surface left (see the [Roadmap](README.md#roadmap)).

## Quick start

```bash
git clone https://github.com/radumalica/infrastructure-mcp.git
cd infrastructure-mcp
go build ./... && go vet ./... && test -z "$(gofmt -l .)" && go test ./... -race -cover
```

These are the same checks CI runs, and they are required on every pull request
into `main`. `golangci-lint run ./...` and `pre-commit run --all-files` are
recommended locally.

## Ground rules

- **Named, validated tools only.** Expose meaningful actions (`disk_usage`,
  `docker_restart`), never a generic "run this string" tool. Anything that runs
  arbitrary commands must be opt-in behind `-enable-run-command` (ADR 0009).
- **Read-only by default.** Mutating tools require `confirm: true` and support
  `dry_run`.
- **Inventory-driven.** No hostnames, addresses, or credentials in tool code.
- **Structured errors and logs** via `internal/toolerr` and `withLogging`; never
  log secrets.
- **Tests:** unit tests for adapters and parsing, plus at least one test that
  exercises new tools through the real MCP protocol.

## Adding an integration

Follow [`docs/ADDING_AN_ADAPTER.md`](docs/ADDING_AN_ADAPTER.md): one package per
integration under `internal/`, tools under `mcp/tools/`, wiring in
`cmd/server/main.go`, docs in `README.md` and `docs/TOOL_REFERENCE.md`.

## Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org) (`feat:`,
  `fix:`, `docs:`, `chore:`, ...). Versioning, `CHANGELOG.md`, and releases are
  generated from them. Mark breaking changes with `!` and a `BREAKING CHANGE:`
  footer.
- Describe what changed and why. For an architectural decision, add an ADR under
  [`docs/adr/`](docs/adr/).
- For larger changes, open an issue or discussion first.

## Security issues

Do not file them publicly; see [SECURITY.md](SECURITY.md).

## Conduct

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).
