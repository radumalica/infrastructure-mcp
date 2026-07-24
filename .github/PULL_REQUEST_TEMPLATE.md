## What

Brief summary of the change and why it's needed.

## How

Key implementation details — what was added, changed, or removed, and why
you chose that approach over alternatives.

## Checklist

- [ ] Commit messages follow [Conventional Commits](https://www.conventionalcommits.org) (`feat:`, `fix:`, `docs:`, `chore:`, ...)
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `gofmt -l .` reports no unformatted files
- [ ] `go test ./... -race` passes
- [ ] New / changed code has tests (unit for adapters/parsing, MCP protocol round-trip for tools)
- [ ] Exported functions are documented
- [ ] No credentials, hostnames, or IPs are hardcoded — everything resolves through inventory
- [ ] Structured errors use `internal/toolerr`, not raw Go errors
- [ ] Tool output includes `status`, `recommendation` (where relevant), and `timestamp`

## Related

Link any related issues, discussions, or PRs:

Closes #
