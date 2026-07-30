## v0.5.0 - 2026-07-30
#### Features
- add banner SVG for project branding in README - (cd6ee91) - tox
#### Documentation
- (**changelog**) update for v0.4.0 [skip ci] - (ff27a8f) - repo-gha[bot]
- add ADRs, tool reference, adapter guide, and examples (#17) - (323aaef) - Radu

- - -

## v0.4.0 - 2026-07-30
#### Features
- support SSH public-key auth for routers/switches (#16) - (c53f85b) - Radu
#### Documentation
- (**changelog**) update for v0.3.0 [skip ci] - (d9fd16d) - repo-gha[bot]

- - -

## v0.3.0 - 2026-07-30
#### Features
- v0.7 (partial) Networking — Cisco read-only diagnostics (#11) - (2b7fc21) - Radu
- v0.6 Proxmox (nodes, VMs, tasks, start/stop/snapshot) (#9) - (cd01ab4) - Radu
- enhance release workflow to support protected main branch and automate changelog updates - (ab84da0) - tox
#### Bug Fixes
- remove unnecessary persist-credentials option in GitHub Actions workflow - (140da03) - tox
- write CHANGELOG.md to main via Contents API instead of git push (#15) - (a955a20) - Radu
- disable credential persistence in checkout step and update GitHub App token action - (ba610fa) - tox
- clear checkout's persisted GITHUB_TOKEN before App-token push to main (#14) - (ef04b90) - Radu
- update GitHub App token step to use client-id instead of app-id - (b871d72) - tox
- push CHANGELOG.md to main via GitHub App token instead of PR (#13) - (c43fcfb) - Radu
- release pipeline corrupting GITHUB_OUTPUT with cocogitto stderr (#12) - (80992ee) - Radu
- quote release workflow strings containing an unescaped colon (#10) - (4728c93) - Radu
- Dockerfile go version + release pipeline never pushing version bump (#8) - (6aab622) - Radu
#### Testing
- app token contents api bypass check [skip ci] - (ba46034) - repo-gha[bot]
- temporary workflow to isolate App-token bypass behavior [skip ci] - (34a38f9) - Radu
- contents api signature check [skip ci] - (411ad9b) - Radu

- - -

## v0.2.0 - 2026-07-30
#### Features
- v0.4 Kubernetes + v0.5 Grafana tools (#7) - (42c546d) - Radu
#### Miscellaneous
- changelog.md format fix (#5) - (a8f6ec4) - Radu
- changelog.md format change - (c315bc8) - tox

- - -

## v0.1.1 - 2026-07-30
#### Bug Fixes
- require a bearer token on -transport=http (#4) - (cf61e73) - Radu
- require a bearer token on -transport=http (security) - (a76e553) - tox

- - -

## v0.1.0 - 2026-07-30
#### Features
- implement GitHub workflows for release management, conventional commit checks, and security scanning (#2) - (3f753c4) - Radu
- implement GitHub workflows for release management, conventional commit checks, and security scanning - (0b59ab9) - tox
- add discussion and issue templates for better community engagement (#1) - (9fe07d6) - Radu
- add discussion and issue templates for better community engagement - (65be5c6) - tox
#### Bug Fixes
- format CHANGELOG.md for clarity (#3) - (e51b492) - Radu
- format CHANGELOG.md for clarity on automatic entry generation - (ef16e7e) - tox
- update dependencies for modelcontextprotocol/go-sdk and jsonschema-go - (bc2ddb2) - tox

- - -

## v0.0.0 - 2026-07-30
#### Features
- CI/CD pipelines, Docker packaging, and remote MCP transport - (cddbd3e) - tox
- v0.3 Docker tools (docker_ps, docker_logs, docker_stats, docker_restart, docker_images) - (c3bb722) - tox
- v0.2 Linux extended tools (failed_services, cpu_usage, reboot_required, running_processes, journal_errors, kernel_version) - (fa24eab) - tox
- Telnet transport + transparent SSH/Telnet dispatch for legacy network devices - (1b90a76) - tox
- unify inventory targets, add SSH legacy crypto support for old devices - (e69bd5c) - tox
- add structured error contract, per-execution logging, host-key CLI flags - (1d9fee4) - tox
- complete v0.1 tool surface (run_command, uptime, disk_usage, memory_usage) - (29fed8e) - tox
- add internal/ssh pooled client with proxyjump and host-key verification - (eaf4cff) - tox
- wire MCP server skeleton with list_servers tool - (01bb71e) - tox
- add inventory package with YAML loading, env-secret expansion, validation - (38b8e40) - tox
#### Bug Fixes
- support multiple Grafana/Proxmox instances in inventory schema - (7d5e309) - tox
#### Documentation
- rewrite README as a professional open-source README, add MIT LICENSE - (60d5e88) - tox
#### Miscellaneous
- ignore .claude/ (agent session/worktree state) - (d261c99) - tox
- add GitHub Actions CI, relax go.mod version to README's 1.24+ floor - (ba4a944) - tox
- initial project scaffold (go.mod, docs) - (6cacbd9) - tox


