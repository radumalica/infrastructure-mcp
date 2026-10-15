# Try it in two minutes

Run Infrastructure MCP against two throwaway Linux containers, with no real
servers, credentials, or cloud accounts involved.

```bash
cd examples/demo
./run-demo.sh        # builds, starts, and calls a few tools
./run-demo.sh down   # removes everything, including the throwaway key
```

Requires Docker with Compose, `python3`, and `ssh-keygen`.

## What it starts

- `target-web` and `target-db`: Debian containers running only `sshd`, reachable
  with a freshly generated key in `.demo-keys/` (gitignored, deleted on `down`).
- `infrastructure-mcp`: the server built from this checkout, listening on
  `http://127.0.0.1:8080/mcp` only, with a demo-only bearer token.

The script then prints the tools an agent can see (there is no `run_command`),
calls `list_servers`, `uptime`, `memory_usage` and `disk_usage`, and shows that a
target outside the inventory is refused.

Disk and memory figures come from the machine running Docker, because the
containers share its kernel and filesystems.

## Connect a real agent

While the demo is up, point any MCP client that supports Streamable HTTP at it:

```bash
claude mcp add --transport http infra-demo http://127.0.0.1:8080/mcp \
  --header "Authorization: Bearer demo-token-not-a-secret"
```

Then ask: "How much memory is free on demo-db?" Remove it afterwards with
`claude mcp remove infra-demo`.

## Notes

- The demo uses `-insecure-ignore-host-key` and a world-readable throwaway key so
  the server container can read it. Both are acceptable only because the targets
  are disposable and bound to localhost. For real infrastructure follow the main
  [README](../../README.md): verified host keys, a private key, and a real token.
- The Docker, Kubernetes, Grafana, Proxmox and Cisco tools need those systems and
  are not part of this demo.
- `render_gif.py` regenerates `assets/demo.gif` from a live run of this demo.
