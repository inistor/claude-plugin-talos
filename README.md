# claude-plugin-talos

A Claude Code plugin for managing Talos Linux clusters. Provides MCP tools, skills, commands, and an agent for cluster lifecycle management, troubleshooting, image creation, and more.

Targets **Talos Linux v1.12**.

## Prerequisites

- `yq` — for YAML parsing in hook scripts
- `docker` — for building custom images with imager

**MCP server** (one of):
- `go` 1.26+ — to install via `go install` (recommended)
- `docker` — to run the pre-built Docker image

## Installation

### 1. Install the MCP server

**Option A: go install (recommended)**

```bash
go install github.com/ionmudreac/claude-plugin-talos/talos-mcp@latest
```

Ensure `~/go/bin` is in your `PATH`:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

**Option B: Docker**

No installation needed — the plugin can use the Docker image directly. Override `.mcp.json` in your project or user settings:

```json
{
  "mcpServers": {
    "talos": {
      "command": "docker",
      "args": ["run", "--rm", "-i", "-v", "${HOME}/.talos:/root/.talos:ro", "ionnistor/talos-mcp:main"]
    }
  }
}
```

> **Note**: The Docker option mounts `~/.talos/config` read-only. If you need to use a local talosconfig from your project directory, use Option A instead — the `talos_set_config` tool can point to any path on the host.

### 2. Install the plugin

```bash
# Add the marketplace (one-time)
/plugin marketplace add inistor/claude-plugins

# Install the plugin
/plugin install talos@inistor-plugins
```

### Upgrade

```bash
# MCP server
go install github.com/ionmudreac/claude-plugin-talos/talos-mcp@latest

# Plugin (reinstall picks up latest)
/plugin install talos@inistor-plugins
```

### Uninstall

```bash
# Remove the binary
rm $(go env GOPATH)/bin/talos-mcp

# Remove the plugin
/plugin uninstall talos
```

## Components

### MCP Server (Go)
A custom Go MCP server wrapping the Talos gRPC API via the official SDK (`github.com/siderolabs/talos/pkg/machinery/client`). Provides 29 tools mapping 1:1 to talosctl operations:

- **Config**: set-config (custom talosconfig path), contexts, config-info
- **Cluster**: bootstrap, health, version, get (resources)
- **Node**: apply-config, reboot, shutdown, reset, upgrade
- **Diagnostics**: logs, dmesg, services, containers, processes
- **System**: disks, mounts, memory, cpu, netstat
- **etcd**: members, snapshot, defrag, status
- **Generation**: gen-config, patch, get-config

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
