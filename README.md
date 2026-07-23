# Infrastructure MCP Server - Technical Architecture & Development Plan

## Vision

Build a production-grade, self-hosted Infrastructure MCP Server that enables AI agents (Hermes, Claude Code, Claude Desktop, Cursor, OpenAI Agents, etc.) to safely interact with infrastructure through high-level tools instead of raw shell access.

The MCP Server should become the single interface between AI agents and infrastructure.

---

# Goals

- Self-hosted
- Production-ready
- Secure by default
- Extensible
- Vendor agnostic
- Infrastructure as Code friendly
- Stateless where possible
- Structured outputs only
- Human-auditable
- Open Source

---

# Non Goals

The project is NOT intended to become:

- another SSH wrapper
- another Ansible replacement
- another monitoring system
- another Kubernetes operator

Instead, it provides AI-friendly infrastructure primitives.

---

# High Level Architecture

```
                 AI Agent
      (Hermes / Claude / Cursor)

                    │
                    │ MCP
                    ▼

        Infrastructure MCP Server

                    │

        ┌───────────┴────────────┐

        │                        │

    Infrastructure APIs      SSH Layer

        │                        │

        ▼                        ▼

 Proxmox                  Linux Servers

 Grafana                  Cisco

 Prometheus               MikroTik

 Docker                   UniFi

 Kubernetes               Home Assistant

 VMware                   Storage

 AWS                      Bare Metal
```

---

# Technology Stack

Language

- Go 1.24+

SDK

- Official Model Context Protocol Go SDK

Configuration

- YAML

Logging

- slog

Configuration validation

- go-playground/validator

SSH

- golang.org/x/crypto/ssh

HTTP

- net/http

JSON

- encoding/json

Testing

- Go testing
- Testcontainers
- Docker Compose

CI

- GitHub Actions

---

# Project Structure

```
infrastructure-mcp/

├── cmd/
│   └── server/
│
├── internal/
│
│   ├── inventory/
│   ├── ssh/
│   ├── linux/
│   ├── docker/
│   ├── kubernetes/
│   ├── proxmox/
│   ├── grafana/
│   ├── prometheus/
│   ├── loki/
│   ├── cisco/
│   ├── mikrotik/
│   ├── unifi/
│   ├── vmware/
│   ├── homeassistant/
│   ├── weather/
│   ├── email/
│   ├── security/
│   ├── backup/
│   └── utils/
│
├── mcp/
│
│   ├── tools/
│   ├── prompts/
│   └── resources/
│
├── configs/
│
├── docs/
│
├── examples/
│
├── scripts/
│
├── tests/
│
└── CLAUDE.md
```

---

# Inventory

Everything should be driven from inventory.

No IP addresses should exist inside tool implementations.

Example:

```yaml
servers:

  archive:
    hostname: 10.0.0.5
    user: hermes
    key: ~/.ssh/archive
    tags:
      - linux
      - ethereum

  pve01:
    hostname: 10.0.0.2
    user: hermes
    proxyjump: jumpbox
    tags:
      - proxmox

routers:

  core:
    hostname: 10.0.0.10
    vendor: cisco

switches:

  sw01:
    hostname: 10.0.0.20
    vendor: cisco

grafana:

  url: https://grafana.lab.local

  token: ${GRAFANA_TOKEN}

proxmox:

  url: https://pve.lab.local:8006

  token: ${PROXMOX_TOKEN}
```

---

# Authentication

Infrastructure MCP should never expose credentials to AI agents.

Authentication belongs inside adapters.

Supported authentication methods:

- SSH Keys
- API Tokens
- OAuth
- Username/Password
- Kubernetes kubeconfig

---

# Adapter Layer

Each integration should be isolated.

```
linux/

docker/

grafana/

proxmox/

cisco/

mikrotik/

unifi/

homeassistant/
```

Every adapter exposes a common interface.

---

# MCP Tool Philosophy

Never expose raw infrastructure.

Instead expose meaningful actions.

Avoid

```
execute(command)
```

Prefer

```
check_disk()

check_memory()

docker_list()

grafana_alerts()

backup_switch()
```

---

# Version 0.1

Core infrastructure

Tools

- list_servers
- run_command
- uptime
- disk_usage
- memory_usage

---

# Version 0.2

Linux

Tools

- failed_services
- cpu_usage
- reboot_required
- running_processes
- journal_errors
- kernel_version

---

# Version 0.3

Docker

Tools

- docker_ps
- docker_logs
- docker_stats
- docker_restart
- docker_images

---

# Version 0.4

Kubernetes

Tools

- kubectl_get_pods
- kubectl_logs
- kubectl_events
- kubectl_describe
- kubectl_nodes

---

# Version 0.5

Grafana

Tools

- grafana_alerts
- grafana_dashboards
- grafana_query
- grafana_annotations

---

# Version 0.6

Proxmox

Tools

- proxmox_nodes
- proxmox_vms
- proxmox_tasks
- proxmox_start_vm
- proxmox_stop_vm
- proxmox_snapshot

---

# Version 0.7

Networking

Cisco

- backup
- show version
- interfaces
- inventory
- logs

MikroTik

- backup
- interfaces
- routes
- firewall
- system resources

UniFi

- devices
- clients
- AP status
- firmware
- statistics

---

# Version 0.8

Monitoring

Prometheus

Loki

Tempo

Alertmanager

---

# Version 0.9

Home Assistant

Tools

- entities
- devices
- automations
- sensors
- cameras

---

# Version 1.0

AI-oriented tools

These become the primary interface.

Instead of calling 15 low-level tools, the AI calls one expert tool.

Examples

diagnose_linux()

diagnose_docker()

diagnose_kubernetes()

diagnose_network()

diagnose_storage()

morning_brief()

incident_summary()

root_cause_analysis()

backup_everything()

check_infrastructure()

---

# Tool Design

Each tool should

- validate inputs
- never panic
- return structured JSON
- include severity
- include recommendations
- include timestamp

Example

```json
{
  "status": "warning",
  "server": "archive",
  "disk_usage": 91,
  "recommendation": "Free disk space or expand storage.",
  "timestamp": "2026-07-23T19:10:00Z"
}
```

---

# Logging

Every tool execution must produce structured logs.

Fields

- tool
- user
- duration
- target
- result
- error

---

# Error Handling

Never return raw Go errors.

Always return

- message
- recommendation
- retryable
- category

---

# Security

- No secrets in logs
- No secrets in prompts
- No shell interpolation
- Validate all inputs
- Whitelist inventory
- Least privilege
- Read-only by default
- Dangerous actions require confirmation

---

# Testing

Unit Tests

100% coverage for

- inventory
- adapters
- parsing
- validation

Integration Tests

Docker Compose

- Ubuntu
- Alpine
- Grafana
- Prometheus
- Loki

---

# Performance Goals

SSH connection pooling

Parallel execution

Context cancellation

Streaming responses

Timeout support

Retry support

---

# Future Features

- AWS
- Azure
- GCP
- VMware
- TrueNAS
- Ceph
- Synology
- IPMI
- Redfish
- OpenStack
- Nomad
- Consul

---

# Claude Code Development Rules

Claude Code should always:

- keep packages small
- use dependency injection
- avoid globals
- prefer interfaces
- write unit tests
- write integration tests
- use context.Context everywhere
- use slog for logging
- avoid reflection when possible
- keep functions under ~50 lines when practical
- return structured errors
- document exported functions
- never expose credentials
- never hardcode infrastructure
- always use inventory

---

# Long-Term Vision

The Infrastructure MCP Server should become a universal infrastructure abstraction layer for AI agents.

Instead of teaching every AI model how to manage Linux, Kubernetes, Proxmox, Cisco, MikroTik, Docker, Grafana, or Home Assistant individually, the MCP Server encapsulates operational expertise behind safe, reusable, high-level tools.

AI agents focus on reasoning.

Infrastructure MCP focuses on execution.