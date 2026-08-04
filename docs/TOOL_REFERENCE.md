# Tool Reference

Every tool below is registered in `cmd/server/main.go` and implemented under
`mcp/tools/`. This reference lists each tool's input parameters and a sample
output shape. All example values are synthetic — copy-paste the shape, not
the data.

Conventions that apply across the whole tool surface (see
[Tool Design Philosophy](../README.md#tool-design-philosophy) in the README):

- Every successful response includes a `timestamp` (RFC 3339, UTC).
- Percentage-based Linux tools (`disk_usage`, `memory_usage`, `cpu_usage`)
  include `status` (`ok`/`warning`/`critical`) and, on warning/critical, a
  `recommendation`.
- Every tool call that fails returns the same structured error envelope,
  never a raw error — see [Error Handling](../README.md#error-handling).
- Tools whose name ends in a mutating verb (`docker_restart`,
  `proxmox_start_vm`, `proxmox_stop_vm`, `proxmox_snapshot`) require
  `confirm: true`; without it they return `status: "confirmation_required"`
  and take no action. See [ADR 0007](adr/0007-structured-errors-and-confirm-gated-mutations.md).
- Those same mutating tools also accept `dry_run: true`, which returns
  `status: "dry_run"` and a `message` describing the exact command/API call
  that would be made, taking no action — even if `confirm: true` is also
  set. `dry_run` always wins over `confirm`.

---

## Core

### `list_servers`

| Param | Required | Description |
|---|---|---|
| `tag` | no | only return servers carrying this tag |

```json
{
  "servers": [
    { "name": "archive", "hostname": "10.0.0.5", "tags": ["linux", "ethereum"] },
    { "name": "pve01", "hostname": "10.0.0.2", "tags": ["proxmox"] }
  ]
}
```

### `run_command`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory target name (server, router, or switch) |
| `command` | yes | shell command to execute |

```json
{
  "server": "archive",
  "command": "df -h /",
  "stdout": "Filesystem      Size  Used Avail Use% Mounted on\n/dev/sda1        49G   21G   26G  45% /\n",
  "stderr": "",
  "exit_code": 0,
  "duration_ms": 118,
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `uptime`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |

```json
{
  "server": "archive",
  "uptime_seconds": 1922400,
  "load1": 0.34,
  "load5": 0.41,
  "load15": 0.38,
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `disk_usage`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |

```json
{
  "server": "archive",
  "filesystems": [
    {
      "filesystem": "/dev/sda1",
      "mount_point": "/",
      "total_kb": 51475068,
      "used_kb": 46853200,
      "available_kb": 2003212,
      "used_percent": 91,
      "status": "critical",
      "recommendation": "Free disk space immediately or expand capacity."
    }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `memory_usage`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |

```json
{
  "server": "archive",
  "total_kb": 16589312,
  "used_kb": 14201880,
  "available_kb": 2387432,
  "used_percent": 85.6,
  "swap_total_kb": 2097148,
  "swap_free_kb": 2097148,
  "status": "warning",
  "recommendation": "Free memory or plan a capacity increase.",
  "timestamp": "2026-07-30T09:00:00Z"
}
```

## Linux

### `failed_services`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |

```json
{
  "server": "archive",
  "services": [
    { "unit": "backup-sync.service", "load": "loaded", "active": "failed", "sub": "failed", "description": "Nightly backup sync" }
  ],
  "status": "warning",
  "recommendation": "Investigate and restart or fix the failed service(s).",
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `cpu_usage`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |

```json
{ "server": "archive", "used_percent": 12.4, "status": "ok", "timestamp": "2026-07-30T09:00:00Z" }
```

### `reboot_required`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |

```json
{
  "server": "archive",
  "required": true,
  "reason": "/var/run/reboot-required present",
  "running_kernel": "5.15.0-181-generic",
  "newest_kernel": "5.15.0-186-generic",
  "status": "warning",
  "recommendation": "Schedule a reboot during a maintenance window.",
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `running_processes`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |
| `limit` | no | max processes returned, sorted by CPU% descending (default 20) |

```json
{
  "server": "archive",
  "processes": [
    { "pid": 4821, "ppid": 1, "user": "root", "cpu_percent": 34.2, "mem_percent": 8.1, "command": "geth --syncmode snap" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `journal_errors`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |
| `limit` | no | max entries returned, most recent first (default 20) |

```json
{
  "server": "archive",
  "entries": [
    { "timestamp": "2026-07-30T08:58:11Z", "unit": "docker.service", "priority": "err", "message": "failed to start container: port already in use" }
  ],
  "status": "warning",
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `kernel_version`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |

```json
{ "server": "archive", "kernel_release": "5.15.0-181-generic", "timestamp": "2026-07-30T09:00:00Z" }
```

## Docker

### `docker_ps`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |
| `all` | no | include stopped containers too (default: running only) |

```json
{
  "server": "archive",
  "containers": [
    { "id": "a1b2c3d4e5f6", "image": "grafana/grafana:11.2.0", "command": "/run.sh", "created_at": "2026-06-01 10:00:00 +0000 UTC", "status": "Up 3 weeks", "state": "running", "ports": "0.0.0.0:3000->3000/tcp", "names": "grafana" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `docker_images`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |

```json
{
  "server": "archive",
  "images": [
    { "id": "sha256:1a2b3c", "repository": "grafana/grafana", "tag": "11.2.0", "created_at": "2026-05-20 12:00:00 +0000 UTC", "size": "412MB" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `docker_stats`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |
| `container` | no | restrict to one container (default: all) |

```json
{
  "server": "archive",
  "stats": [
    { "id": "a1b2c3d4e5f6", "name": "grafana", "cpu_percent": "1.24%", "mem_usage": "128MiB / 4GiB", "mem_percent": "3.12%", "net_io": "1.2MB / 800kB", "block_io": "0B / 4.1MB", "pids": "18" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `docker_logs`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |
| `container` | yes | container name or ID |
| `tail` | no | most recent N lines (default 100) |

```json
{
  "server": "archive",
  "container": "grafana",
  "logs": "logger=settings ... msg=\"Config loaded\"\n",
  "line_count": 1,
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `docker_restart` — mutating, requires `confirm: true`

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |
| `container` | yes | container name or ID |
| `confirm` | yes to act | must be `true` to actually restart |
| `dry_run` | no | see `dry_run` above — shows the command, takes no action |

Unconfirmed call:

```json
{ "server": "archive", "container": "grafana", "status": "confirmation_required", "message": "Set confirm: true to restart this container. No action was taken.", "timestamp": "2026-07-30T09:00:00Z" }
```

Confirmed call:

```json
{ "server": "archive", "container": "grafana", "status": "restarted", "message": "Container restarted.", "timestamp": "2026-07-30T09:00:00Z" }
```

### `docker_exec`

Container-scoped equivalent of `run_command` — not confirm-gated, following `run_command`'s own convention rather than `docker_restart`'s. A non-zero `exit_code` is reported as data, not a tool error.

| Param | Required | Description |
|---|---|---|
| `server` | yes | inventory server name |
| `container` | yes | container name or ID |
| `command` | yes | shell command to run inside the container |

```json
{ "server": "archive", "container": "grafana", "command": "cat /etc/grafana/grafana.ini | grep http_port", "stdout": "http_port = 3000\n", "stderr": "", "exit_code": 0, "timestamp": "2026-07-30T09:00:00Z" }
```

## Kubernetes

### `kubectl_get_pods`

| Param | Required | Description |
|---|---|---|
| `cluster` | yes | inventory kubernetes cluster name |
| `namespace` | no | scope to one namespace (default: all) |

```json
{
  "cluster": "home",
  "pods": [
    { "name": "grafana-7f9c9b6c8-abcde", "namespace": "monitoring", "phase": "Running", "ready": "1/1", "restarts": 0, "node": "node-a", "start_time": "2026-06-01T10:00:00Z" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `kubectl_logs`

| Param | Required | Description |
|---|---|---|
| `cluster` | yes | inventory kubernetes cluster name |
| `namespace` | yes | pod's namespace |
| `pod` | yes | pod name |
| `container` | no | container name (default: pod's only/first) |
| `tail` | no | most recent N lines (default: all) |

```json
{
  "cluster": "home",
  "namespace": "monitoring",
  "pod": "grafana-7f9c9b6c8-abcde",
  "logs": "level=info msg=\"HTTP Server Listen\"\n",
  "line_count": 1,
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `kubectl_events`

| Param | Required | Description |
|---|---|---|
| `cluster` | yes | inventory kubernetes cluster name |
| `namespace` | no | scope to one namespace (default: all) |

```json
{
  "cluster": "home",
  "events": [
    { "type": "Warning", "reason": "BackOff", "object": "pod/grafana-7f9c9b6c8-abcde", "message": "Back-off restarting failed container", "count": 3, "first_seen": "2026-07-30T08:40:00Z", "last_seen": "2026-07-30T08:59:00Z" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `kubectl_describe`

| Param | Required | Description |
|---|---|---|
| `cluster` | yes | inventory kubernetes cluster name |
| `namespace` | yes | pod's namespace |
| `pod` | yes | pod name |

```json
{
  "cluster": "home",
  "namespace": "monitoring",
  "pod": "grafana-7f9c9b6c8-abcde",
  "phase": "Running",
  "node": "node-a",
  "pod_ip": "10.42.0.14",
  "start_time": "2026-06-01T10:00:00Z",
  "containers": [
    { "name": "grafana", "ready": true, "restart_count": 0, "image": "grafana/grafana:11.2.0", "state": "running" }
  ],
  "events": [],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `kubectl_nodes`

| Param | Required | Description |
|---|---|---|
| `cluster` | yes | inventory kubernetes cluster name |

```json
{
  "cluster": "home",
  "nodes": [
    { "name": "node-a", "ready": true, "roles": ["control-plane"], "unschedulable": false, "kubelet_version": "v1.30.2", "os_image": "Ubuntu 22.04.4 LTS", "kernel_version": "5.15.0-181-generic", "container_runtime": "containerd://1.7.18", "cpu_capacity": "8", "memory_capacity": "32720316Ki", "internal_ip": "10.42.0.1" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `kubectl_exec`

Pod-scoped equivalent of `run_command`/`docker_exec` — not confirm-gated. A non-zero `exit_code` is reported as data, not a tool error.

| Param | Required | Description |
|---|---|---|
| `cluster` | yes | inventory kubernetes cluster name |
| `namespace` | yes | namespace the pod is in |
| `pod` | yes | pod name |
| `container` | no | container name (default: the pod's only/first container) |
| `command` | yes | command and args, e.g. `["cat", "/etc/resolv.conf"]` |

```json
{ "cluster": "home", "namespace": "default", "pod": "app-1", "command": ["cat", "/etc/resolv.conf"], "stdout": "nameserver 10.43.0.10\n", "stderr": "", "exit_code": 0, "timestamp": "2026-07-30T09:00:00Z" }
```

## Grafana

### `grafana_alerts`

| Param | Required | Description |
|---|---|---|
| `instance` | yes | inventory grafana instance name |

```json
{
  "instance": "main",
  "alerts": [
    { "status": "firing", "labels": { "alertname": "HighDiskUsage", "instance": "archive" }, "starts_at": "2026-07-30T08:00:00Z", "fingerprint": "9f2a1c" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `grafana_dashboards`

| Param | Required | Description |
|---|---|---|
| `instance` | yes | inventory grafana instance name |
| `query` | no | free-text title filter |
| `tag` | no | filter by tag |

```json
{
  "instance": "main",
  "dashboards": [
    { "uid": "abc123", "title": "Node Exporter Full", "tags": ["linux", "prometheus"], "url": "/d/abc123/node-exporter-full" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `grafana_annotations`

| Param | Required | Description |
|---|---|---|
| `instance` | yes | inventory grafana instance name |
| `from_ms` / `to_ms` | no | time range, epoch ms |
| `tags` | no | only annotations with all of these tags |

```json
{
  "instance": "main",
  "annotations": [
    { "id": 42, "dashboard_uid": "abc123", "time": 1785400000000, "text": "Deployed v0.7.0", "tags": ["deploy"] }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `grafana_query`

| Param | Required | Description |
|---|---|---|
| `instance` | yes | inventory grafana instance name |
| `datasource_uid` | yes | target datasource UID |
| `query` | yes | raw PromQL/LogQL/SQL/... expression |
| `from` / `to` | no | time range (relative like `now-1h`, or epoch ms) |

`result` is a datasource-specific passthrough (Prometheus/Loki/SQL shape),
not normalized — read it according to the datasource you queried.

```json
{
  "instance": "main",
  "datasource_uid": "prometheus-uid",
  "result": { "status": "success", "data": { "resultType": "vector", "result": [] } },
  "timestamp": "2026-07-30T09:00:00Z"
}
```

## Proxmox

### `proxmox_nodes`

| Param | Required | Description |
|---|---|---|
| `instance` | yes | inventory proxmox instance name |

```json
{
  "instance": "lab",
  "nodes": [
    { "node": "pve01", "status": "online", "cpu": 0.08, "max_cpu": 16, "mem": 17179869184, "max_mem": 68719476736, "uptime": 5184000 }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `proxmox_vms`

| Param | Required | Description |
|---|---|---|
| `instance` | yes | inventory proxmox instance name |
| `node` | yes | cluster node to list guests on |

```json
{
  "instance": "lab",
  "node": "pve01",
  "vms": [
    { "vmid": 101, "name": "grafana-vm", "type": "qemu", "status": "running", "cpu": 0.03, "max_mem": 4294967296, "mem": 1717986918, "uptime": 2592000 }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `proxmox_tasks`

| Param | Required | Description |
|---|---|---|
| `instance` | yes | inventory proxmox instance name |
| `node` | yes | cluster node to list tasks on |
| `limit` | no | max tasks returned, most recent first |

```json
{
  "instance": "lab",
  "node": "pve01",
  "tasks": [
    { "upid": "UPID:pve01:00001A2B:...", "type": "vzdump", "status": "OK", "user": "root@pam", "start_time": 1785396000, "end_time": 1785396600 }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `proxmox_start_vm` — state-changing, requires `confirm: true`

| Param | Required | Description |
|---|---|---|
| `instance` | yes | inventory proxmox instance name |
| `node` | yes | cluster node the guest lives on |
| `vmid` | yes | numeric VM/container ID |
| `type` | no | `qemu` (default) or `lxc` |
| `confirm` | yes to act | must be `true` |
| `dry_run` | no | see `dry_run` above — shows the API call, takes no action |

Confirmed call — starting is asynchronous, check the returned `upid` with `proxmox_tasks`:

```json
{ "instance": "lab", "node": "pve01", "vmid": 101, "type": "qemu", "status": "start_requested", "upid": "UPID:pve01:00001A2C:...", "message": "Start requested; check proxmox_tasks with this UPID for the outcome.", "timestamp": "2026-07-30T09:00:00Z" }
```

### `proxmox_stop_vm` — destructive, requires `confirm: true`

Same params as `proxmox_start_vm`. This is a **hard power-off**, not a
graceful shutdown.

### `proxmox_snapshot` — state-changing, requires `confirm: true`

Same params as `proxmox_start_vm` plus `name` (snapshot name, required).

## Networking — Cisco

### `cisco_version`

| Param | Required | Description |
|---|---|---|
| `device` | yes | inventory router/switch name |

```json
{
  "device": "sw01",
  "hostname": "sw01",
  "version_line": "Cisco IOS Software, Catalyst 4500 L3 Switch Software (cat4500-ENTSERVICESK9-M), Version 15.0(2)SG11, RELEASE SOFTWARE (fc2)",
  "uptime": "47 weeks, 15 hours, 58 minutes",
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `cisco_interfaces`

| Param | Required | Description |
|---|---|---|
| `device` | yes | inventory router/switch name |

```json
{
  "device": "sw01",
  "interfaces": [
    { "interface": "Vlan100", "ip_address": "10.0.0.20", "ok": "YES", "method": "NVRAM", "status": "up", "protocol": "up" },
    { "interface": "GigabitEthernet1/0/1", "ip_address": "unassigned", "ok": "YES", "method": "unset", "status": "up", "protocol": "up" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `cisco_inventory`

| Param | Required | Description |
|---|---|---|
| `device` | yes | inventory router/switch name |

```json
{
  "device": "sw01",
  "entries": [
    { "name": "1", "description": "WS-C3650-48P chassis", "pid": "WS-C3650-48PS-E", "vid": "V07", "serial_number": "FDO2000A1BC" }
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `cisco_logs`

| Param | Required | Description |
|---|---|---|
| `device` | yes | inventory router/switch name |
| `limit` | no | most recent N lines (default: all buffered) |

```json
{
  "device": "sw01",
  "lines": [
    "*Jul 30 08:59:11.204: %LINK-3-UPDOWN: Interface GigabitEthernet1/0/12, changed state to down"
  ],
  "timestamp": "2026-07-30T09:00:00Z"
}
```

### `cisco_backup`

| Param | Required | Description |
|---|---|---|
| `device` | yes | inventory router/switch name |

Returns the running-config text as-is (`config` field). Not shown here for
brevity; see `cisco_backup.go`.

Pagination: Telnet-connected devices get `terminal length 0` sent
automatically first (Telnet sessions are persistent, so this actually takes
effect); SSH-connected devices do not, since SSH runs one command per
ephemeral exec channel and pagination settings don't carry over between
calls — if an SSH-connected device's default page length isn't already
unlimited, only the first `--More--` page comes back. Configure
`terminal length 0` on the device itself for SSH-reached targets.

### `cisco_backup_diff`

| Param | Required | Description |
|---|---|---|
| `device` | yes | inventory router/switch name |

Fetches the current running-config (same call as `cisco_backup`) and diffs it against the last snapshot this tool took for the same device, persisted under `-backup-dir` (default `configs/backups`, one file per device, one snapshot — not a rotating history). The first call for a device has nothing to diff against yet.

First call for a device:

```json
{ "device": "core-sw", "changed": false, "first_snapshot": true, "timestamp": "2026-07-30T09:00:00Z" }
```

A later call after the config changed:

```json
{
  "device": "core-sw",
  "changed": true,
  "first_snapshot": false,
  "diff": "--- previous\n+++ current\n@@ -1,3 +1,3 @@\n hostname router1\n-interface Gi0/1\n+interface Gi0/2\n !\n",
  "timestamp": "2026-07-30T09:05:00Z"
}
```

---

## Resources

Resources are read-only, URI-addressed data an agent can fetch directly,
distinct from tools (which are invoked with arguments). Registered in
`mcp/resources/`.

### `audit://recent`

The most recent mutating tool invocations (`docker_restart`,
`proxmox_start_vm`, `proxmox_stop_vm`, `proxmox_snapshot`) this server has
handled, newest first — including blocked attempts (`confirmation_required`,
`dry_run`), not just ones that actually mutated something. In-memory only
(`-audit-history-size`, default 200 entries); cleared on restart. Not a
durable audit trail — see `internal/audit`'s package doc for why.

```json
[
  { "timestamp": "2026-08-03T10:05:00Z", "tool": "docker_restart", "target": "archive", "user": "claude-code", "status": "restarted" },
  { "timestamp": "2026-08-03T10:04:12Z", "tool": "proxmox_snapshot", "target": "lab", "user": "claude-code", "status": "confirmation_required" }
]
```

---

See [Examples](../README.md#examples) in the README for full request/response
walkthroughs, and [`examples/`](../examples/) for runnable inventory
snippets and MCP client configs.
