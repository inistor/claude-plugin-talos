# claude-plugin-talos

A Claude Code plugin for managing Talos Linux clusters. Provides MCP tools, a skill, commands, and an agent for cluster lifecycle management, troubleshooting, image creation, and more.

Targets **Talos Linux v1.14**. Differences against v1.13 are called out inline in the skill where they matter.

## Prerequisites

- `docker` — required for the MCP server and for building custom images with imager
- `talosctl` v1.14+ — for the handful of operations with no MCP equivalent (config generation, Kubernetes upgrade, support bundle, CA rotation, etcd restore)

## Installation

### 1. Install the plugin

```bash
# Add the marketplace (one-time)
/plugin marketplace add inistor/claude-plugins

# Install the plugin
/plugin install talos@inistor-plugins
```

The MCP server runs as a Docker container (`ionnistor/talos-mcp`) — no separate install needed. It mounts `~/.talos/config` read-only. For project-specific talosconfig files, Claude reads the file and passes its content to `talos_set_config`.

To use `talos_etcd_snapshot`, also mount a writable directory at `/out`; the handler refuses to write anywhere that would be discarded when the container exits:

```json
{"mcpServers": {"talos": {"command": "sh", "args": ["-c",
  "docker run --rm -i -v $HOME/.talos:/root/.talos:ro -v $HOME/.talos/backups:/out ionnistor/talos-mcp:0.1.19"]}}}
```

**Alternative: go install** (if you prefer a local binary over Docker)

```bash
go install github.com/ionmudreac/claude-plugin-talos/talos-mcp@latest
```

Then override `.mcp.json` in your settings: `{"mcpServers": {"talos": {"command": "talos-mcp"}}}`

## Components

### MCP Server (Go)
A Go MCP server wrapping the Talos gRPC API via the official SDK (`github.com/siderolabs/talos/pkg/machinery/client`), built against machinery v1.14.0. Distributed as a Docker image (`ionnistor/talos-mcp`). Pure API — no talosctl dependency.

52 tools, all annotated read-only or destructive so a host can gate them:

- **Config**: `talos_set_config`, `talos_config_info`, `talos_machine_config`
- **Cluster**: `talos_bootstrap`, `talos_health`, `talos_version`, `talos_members`, `talos_kubeconfig`, `talos_get`
- **Node**: `talos_apply_config`, `talos_patch`, `talos_reboot`, `talos_shutdown`, `talos_reset`, `talos_upgrade`, `talos_rollback`, `talos_wipe`
- **Services & images**: `talos_services`, `talos_service_restart`, `talos_containers`, `talos_stats`, `talos_image_list`, `talos_image_remove`, `talos_image_prune`
- **Diagnostics**: `talos_logs`, `talos_dmesg`, `talos_processes`
- **System**: `talos_disks`, `talos_mounts`, `talos_memory`, `talos_cpu`, `talos_disk_usage`, `talos_time`
- **Network**: `talos_interfaces`, `talos_addresses`, `talos_routes`, `talos_netstat`, `talos_resolvers`, `talos_hostname`
- **Storage**: `talos_volumes`, `talos_discovered_volumes`
- **etcd**: `talos_etcd_members`, `talos_etcd_status`, `talos_etcd_snapshot`, `talos_etcd_defrag`, `talos_etcd_remove_member`, `talos_etcd_forfeit_leadership`, `talos_etcd_leave`, `talos_etcd_alarm`
- **Filesystem**: `talos_ls`, `talos_read`
- **Extensions**: `talos_extensions`

Every node-aware tool takes a single `node`, plus optional `context` and `endpoint` (to bypass the talosconfig endpoints when they are unreachable).

### Skill
Talos Linux reference covering machine configuration, cluster lifecycle, boot assets, extensions, networking, storage, security, and troubleshooting. Depth lives in five reference files loaded on demand: `machine-config.md`, `networking.md`, `boot-assets.md`, `troubleshooting.md`, `upgrade.md`.

### Commands
- `/talos-bootstrap` — Bootstrap a new cluster from scratch
- `/talos-upgrade` — Upgrade Talos and/or Kubernetes
- `/talos-image` — Build custom images with extensions using the local imager
- `/talos-health` — Comprehensive cluster health check

### Agent
- **talos-operator** — Agent for multi-step cluster operations and troubleshooting.

## Design Philosophy

- **MCP-first**: Talos operations go through the MCP server (gRPC API), not the talosctl CLI. The handful of genuine exceptions are listed in the skill.
- **Kubernetes MCP**: K8s operations use the Kubernetes MCP tools, not kubectl
- **Structured data**: Use `yq`/`jq` for parsing, never grep on YAML/JSON
- **Thin wrapper**: each handler maps closely onto the Talos client API
- **No session-start hook**: the plugin does nothing until you invoke it. Cluster context is available on demand from `talos_config_info`.

## Development

```bash
cd talos-mcp
go build ./... && go vet ./... && go test ./...
```

Releases are tagged `vX.Y.Z`; CI publishes `ionnistor/talos-mcp:X.Y.Z`. The image tag in `.mcp.json` must match the `version` in `.claude-plugin/plugin.json` — CI enforces this, since the two drifted apart more than once.
