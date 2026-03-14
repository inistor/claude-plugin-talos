# claude-plugin-talos

A Claude Code plugin for managing Talos Linux clusters. Provides MCP tools, skills, commands, and an agent for cluster lifecycle management, troubleshooting, image creation, and more.

Targets **Talos Linux v1.12**.

## Prerequisites

- `talosctl` v1.12+ installed and configured (`~/.talos/config`)
- `yq` — for YAML parsing in hook scripts
- `docker` — for building custom images with imager
- `go` 1.22+ — only needed to build the MCP server from source

## Installation

```bash
# Clone the plugin
git clone https://github.com/ionmudreac/claude-plugin-talos.git

# Build the MCP server
cd claude-plugin-talos/mcp-server
go build -o talos-mcp .

# Use with Claude Code
claude --plugin-dir /path/to/claude-plugin-talos
```

## Components

### MCP Server (Go)
A custom Go MCP server wrapping the Talos gRPC API via the official SDK (`github.com/siderolabs/talos/pkg/machinery/client`). Provides 28 tools mapping 1:1 to talosctl operations:

- **Cluster**: bootstrap, health, version, get (resources)
- **Node**: apply-config, reboot, shutdown, reset, upgrade
- **Diagnostics**: logs, dmesg, services, containers, processes
- **System**: disks, mounts, memory, cpu, netstat
- **etcd**: members, snapshot, defrag, status
- **Config**: gen-config, patch, get-config, contexts, config-info

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
