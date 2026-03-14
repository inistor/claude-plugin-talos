# claude-plugin-talos

A Claude Code plugin for managing Talos Linux clusters. Provides MCP tools, skills, commands, and an agent for cluster lifecycle management, troubleshooting, image creation, and more.

Targets **Talos Linux v1.12**.

## Prerequisites

- `docker` — required for the MCP server and for building custom images with imager
- `talosctl` v1.12+ — for operations not covered by the MCP (resource listing, config generation, patching)
- `yq` — for YAML parsing in hook scripts

## Installation

### 1. Install the plugin

```bash
# Add the marketplace (one-time)
/plugin marketplace add inistor/claude-plugins

# Install the plugin
/plugin install talos@inistor-plugins
```

The MCP server runs as a Docker container (`ionnistor/talos-mcp`) — no separate install needed. It mounts `~/.talos/config` by default. For project-specific talosconfig files, Claude reads the file and passes its content to `talos_set_config`.

**Alternative: go install** (if you prefer a local binary over Docker)

```bash
go install github.com/ionmudreac/claude-plugin-talos/talos-mcp@latest
```

Then override `.mcp.json` in your settings: `{"mcpServers": {"talos": {"command": "talos-mcp"}}}`

## Components

### MCP Server (Go)
A custom Go MCP server wrapping the Talos gRPC API via the official SDK (`github.com/siderolabs/talos/pkg/machinery/client`). Distributed as a Docker image (`ionnistor/talos-mcp`). Pure API — no talosctl dependency.

- **Config**: set-config (custom talosconfig via content), config-info
- **Cluster**: bootstrap, health, version
- **Node**: apply-config, reboot, shutdown, reset, upgrade
- **Diagnostics**: logs, dmesg, services, containers, processes
- **System**: disks, mounts, memory, netstat
- **etcd**: members, snapshot, defrag, status

### Skill
Comprehensive Talos Linux reference covering machine configuration, cluster lifecycle, boot assets, extensions, networking, security, and troubleshooting. Includes 4 reference files with detailed YAML examples.

### Commands
- `/talos-bootstrap` — Bootstrap a new cluster from scratch
- `/talos-upgrade` — Upgrade Talos and/or Kubernetes
- `/talos-image` — Build custom images with extensions using local imager
- `/talos-health` — Comprehensive cluster health check

### Agent
- **talos-operator** — Autonomous agent for cluster operations and troubleshooting. Triggers on Talos-related tasks like upgrades, diagnostics, node management, and image creation.

### Hook
- **SessionStart** — Detects talosconfig context and exposes cluster info to Claude at session start.

## Design Philosophy

- **MCP-first**: All Talos operations go through the MCP server (gRPC API), not talosctl CLI
- **Kubernetes MCP**: K8s operations use the existing Kubernetes MCP tools, not kubectl
- **Structured data**: Use `yq`/`jq` for parsing, never grep on YAML/JSON
- **Thin wrapper**: The Go MCP server is minimal — each handler is a direct mapping to the Talos client API
