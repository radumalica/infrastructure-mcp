# Glama listing

[Glama](https://glama.ai) builds listed servers itself: it ignores this
repository's `Dockerfile`, clones the repo into `/app`, runs the configured build
steps there, and starts the container with the configured command so it can
inspect the server. The values below are what the server's admin page needs.
They were checked by running the same steps in a clean `debian:trixie-slim`
container, but Glama's form formats come from maintainer reports, not official
documentation, so re-verify if the form changes.

## Build steps (JSON array of shell commands)

```json
[
  "apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*",
  "curl -fsSL https://go.dev/dl/go1.26.9.linux-$(dpkg --print-architecture).tar.gz | tar -C /usr/local -xz",
  "GOCACHE=/tmp/go-cache GOPATH=/tmp/go /usr/local/go/bin/go build -buildvcs=false -trimpath -ldflags='-s -w' -o /usr/local/bin/infrastructure-mcp ./cmd/server"
]
```

Keep the Go version in step with `go` in `go.mod`.

## CMD arguments (JSON array)

```json
[
  "/usr/local/bin/infrastructure-mcp",
  "-inventory",
  "/app/examples/demo/inventory.demo.yaml"
]
```

Glama wraps stdio servers with `mcp-proxy` automatically, so the command must
invoke the binary directly; adding `mcp-proxy` here would stack a second proxy
layer and can prevent startup or tool discovery. The inventory is the secret-free
demo one, so the server starts and lists its tools without any real
infrastructure. It must not be `configs/inventory.example.yaml`, which fails to
load unless its `${VAR}` secrets are set.

## Environment variables JSON schema

With the secret-free demo inventory the server needs no environment variables, so
declare none. This is a valid JSON Schema (draft 7) that requires nothing:

```json
{
  "type": "object",
  "properties": {},
  "required": []
}
```

If Glama pre-fills this field with something else, replace it with the schema
above. Do not mark `MCP_HTTP_TOKEN`, `GRAFANA_TOKEN` or other secrets as required:
they apply to HTTP mode and to real inventories, and a required variable would
stop Glama's inspector from starting the server. `glama.json` only carries
`$schema` and `maintainers`; it does not feed this field.

## Notes

- Arbitrary command execution (`run_command`, `docker_exec`, `kubectl_exec`) is
  off by default, so Glama inspects only the named, read-mostly tools.
- `glama.json` at the repository root lets the owner claim the listing.
