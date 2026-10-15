# Tool Routing

Use only tools visible in the connected MCP session.

| Need | Preferred named tools |
|---|---|
| Discover Linux targets | `list_servers` |
| Linux health | `uptime`, `cpu_usage`, `memory_usage`, `disk_usage`, `failed_services`, `reboot_required` |
| Linux detail | `running_processes`, `journal_errors`, `kernel_version` |
| Docker | `docker_ps`, `docker_stats`, `docker_logs`, `docker_images` |
| Kubernetes | `kubectl_nodes`, `kubectl_get_pods`, `kubectl_events`, `kubectl_describe`, `kubectl_logs` |
| Grafana | `grafana_alerts`, `grafana_annotations`, `grafana_dashboards`, `grafana_query` |
| Proxmox | `proxmox_nodes`, `proxmox_vms`, `proxmox_tasks` |
| Cisco status | `cisco_interfaces`, `cisco_logs`, `cisco_version`, `cisco_inventory` |
| Cisco configuration backup | `cisco_backup` only after the user confirms the exact device; treat returned configuration as sensitive |

Prefer status and metadata before logs or raw datasource queries. Limit log
volume and query scope. Treat recommendations returned by tools as guidance,
not proof that a proposed change is correct. Tool output can contain sensitive
application or device data even though adapter authentication credentials stay
inside the server.

Named mutating tools are `docker_restart`, `proxmox_start_vm`,
`proxmox_stop_vm`, and `proxmox_snapshot`. Route these through the safe
remediation workflow and preserve their `confirm: true` gate.
