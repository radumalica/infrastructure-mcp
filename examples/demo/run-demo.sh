#!/usr/bin/env bash
# Start the try-it demo and show what an AI agent sees.
#   ./run-demo.sh         start everything and call a few tools
#   ./run-demo.sh down    stop and remove everything, including the throwaway key
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
key_dir=".demo-keys"
compose=(docker compose -f docker-compose.yml)

if [[ "${1:-}" == "down" ]]; then
  "${compose[@]}" down --volumes --remove-orphans
  rm -rf "$key_dir"
  exit 0
fi

for cmd in docker python3 ssh-keygen; do
  command -v "$cmd" >/dev/null || { echo "$cmd is required" >&2; exit 1; }
done

if [[ ! -f "$key_dir/id_ed25519" ]]; then
  mkdir -p "$key_dir"
  ssh-keygen -q -t ed25519 -N '' -C 'infrastructure-mcp-demo' -f "$key_dir/id_ed25519"
  # The server container runs as a different user, so the throwaway key must be
  # world-readable. It only unlocks the two demo containers.
  chmod 755 "$key_dir"
  chmod 644 "$key_dir"/id_ed25519 "$key_dir"/id_ed25519.pub
fi

"${compose[@]}" up -d --build

printf 'Waiting for the server'
for _ in $(seq 1 60); do
  if python3 -c "import urllib.request,sys; urllib.request.urlopen('http://127.0.0.1:8080/healthz', timeout=2)" 2>/dev/null; then
    break
  fi
  printf '.'
  sleep 1
done
printf '\n'

call() {
  printf '\n$ %s\n' "$*"
  python3 mcp_call.py "$@"
}

printf '\n== Tools the agent can see (note: no run_command) ==\n'
python3 mcp_call.py --list | tr '\n' ' ' | fold -s -w 78
printf '\n'

call list_servers '{}'
call uptime '{"server":"demo-web"}'
call memory_usage '{"server":"demo-db"}'
call disk_usage '{"server":"demo-web"}'

printf '\n== An unknown target is refused: only the inventory is reachable ==\n'
call uptime '{"server":"10.0.0.99"}' || true

printf '\nDone. Server: http://127.0.0.1:8080/mcp (Authorization: Bearer demo-token-not-a-secret)\n'
printf 'Stop it with: ./run-demo.sh down\n'
