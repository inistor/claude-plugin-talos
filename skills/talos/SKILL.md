---
name: Talos Linux
description: |
  This skill should be used when working with Talos Linux clusters, talosctl, or the Talos API.
  Covers machine configuration (v1alpha1), cluster bootstrap, Talos upgrades, Kubernetes version
  upgrades, boot asset building with imager, system extensions, networking (bonds, VLANs, VIPs,
  WireGuard, KubeSpan), etcd maintenance, troubleshooting, and disaster recovery.
  Triggers for queries like "upgrade my Talos cluster", "build a custom Talos ISO with extensions",
  "etcd is unhealthy", "node won't join the cluster", "configure bonding on Talos",
  "bootstrap a new Talos cluster", "reset a Talos node", "add a worker node",
  "restore etcd from snapshot", or "recover a failed control plane node".
version: 1.12.0
---

This skill covers Talos Linux v1.12. All documentation references point to https://docs.siderolabs.com/talos/v1.12/.

## Tool Usage Rules

Use Talos MCP tools (`mcp__talos__*`) for all Talos operations. Never shell out to `talosctl` unless the MCP tool is unavailable.

Use Kubernetes MCP tools (`mcp__kubernetes-mcp-server__*`) for all Kubernetes operations. Avoid `kubectl` unless the MCP tool is impractical (e.g., events — see below).

Use `yq` or `jq` for parsing YAML/JSON output. Avoid `grep` on structured data.

**Avoid large results**: MCP tool results that exceed the context window get dumped to temp files and become unusable. Always scope queries narrowly:
- For Kubernetes events: the MCP events tool often returns too much data even with namespace filtering. Instead, use `kubectl get events -n <namespace> --sort-by='.lastTimestamp' | tail -50` via Bash, or if the result was saved to a temp file, extract recent events with `jq '[.[] | .text | fromjson] | sort_by(.lastTimestamp) | last(20)' <file>`
- For pod lists: filter by namespace, never list all pods across all namespaces
- For resource lists: specify the namespace and use label selectors when possible
- For logs: always use `tail_lines` to limit output
- If a result is saved to a temp file, read it with `jq` or `yq` via Bash to extract only what's needed, then retry with a narrower query

**Operations requiring `talosctl`** (no MCP equivalent — use via Bash):
- `talosctl gen secrets` / `talosctl gen config` — generate cluster configuration
- `talosctl machineconfig patch` — apply strategic merge patches to configs

## Talosconfig

The Talos client config lives at `~/.talos/config` (or `$TALOSCONFIG`). It contains contexts with endpoints and TLS credentials. Each MCP tool accepts an optional `node` parameter to target a specific node and `context` to select a talosconfig context.

**Before any Talos operation**, check if a local `talosconfig` file exists in the current working directory or project root. If found, base64-encode it via Bash (`base64 < talosconfig`) and call `talos_set_config(content)` with the base64 output. This preserves the exact file formatting (long base64 cert lines must not be wrapped). This is critical when working in project directories that have their own cluster configs.

**Common TLS error**: If you see `x509: certificate signed by unknown authority` or `Ed25519 verification failure`, this does NOT mean the certificates are incompatible. It means the **talosconfig does not match the cluster** — wrong config for the target cluster, stale config from a rebuilt cluster, or `talos_set_config` was not called. Fix: verify the correct talosconfig is loaded, re-run `talos_set_config` with the right file.

**Single-cluster per session**: The MCP server is stateful — `talos_set_config` sets the config for ALL subsequent calls in the session. It cannot operate on multiple clusters in parallel. To switch clusters, call `talos_set_config` again with the new config content. Use the `context` parameter on individual tools to switch between contexts within the same talosconfig.

## Talos Overview

Talos Linux is an immutable, API-driven, minimal Linux OS designed for Kubernetes. There is no SSH, no shell, no package manager. All management is via the Talos API (port 50000) using mutual TLS. The OS is read-only with an A/B partition scheme for atomic upgrades and rollback.

Key components: `machined` (init), `apid` (API gateway), `trustd` (certificate authority), `etcd` (on control plane nodes only).

## Available MCP Tools

**Config**: `talos_set_config`, `talos_config_info`, `talos_machine_config`
**Cluster**: `talos_bootstrap`, `talos_health`, `talos_version`, `talos_members`, `talos_kubeconfig`, `talos_get`
**Node**: `talos_apply_config`, `talos_reboot`, `talos_shutdown`, `talos_reset`, `talos_upgrade`, `talos_rollback`, `talos_wipe`
**Services**: `talos_services`, `talos_service_restart`, `talos_containers`, `talos_stats`, `talos_image_list`
**Diagnostics**: `talos_logs`, `talos_dmesg`, `talos_processes`
**System**: `talos_disks`, `talos_mounts`, `talos_memory`, `talos_cpu`, `talos_disk_usage`, `talos_time`
**Network**: `talos_interfaces`, `talos_addresses`, `talos_routes`, `talos_netstat`, `talos_resolvers`, `talos_hostname`
**Storage**: `talos_volumes`, `talos_discovered_volumes`
**etcd**: `talos_etcd_members`, `talos_etcd_status`, `talos_etcd_snapshot`, `talos_etcd_defrag`, `talos_etcd_remove_member`, `talos_etcd_forfeit_leadership`, `talos_etcd_leave`, `talos_etcd_alarm`
**Filesystem**: `talos_ls`, `talos_read`
**Extensions**: `talos_extensions`

## Machine Configuration

Talos uses a single YAML configuration file (v1alpha1) with two top-level sections:

- `machine` — node-specific: type (controlplane/worker), network, install disk, kubelet, files, kernel args, sysctls, extensions
- `cluster` — cluster-wide: control plane endpoint, cluster name, API server, etcd, discovery, CNI, inline manifests

Generate configs: `talosctl gen config <cluster-name> <endpoint>` (via Bash)

Apply configs: `talos_apply_config(config, mode)` — modes: `auto` (default), `no-reboot`, `staged`, `try`

Modify configs with **strategic merge patches**. Use `$patch: delete` to remove fields. Multi-document YAML for multiple patches. See `references/machine-config.md` for full v1alpha1 structure.

## Cluster Lifecycle

### Bootstrap
1. Generate configs (talosctl) → 2. Apply controlplane config (`talos_apply_config`) → 3. Apply worker config → 4. `talos_bootstrap` on ONE CP node → 5. `talos_health` → 6. `talos_kubeconfig`

### Upgrade Talos
1. Check current versions (`talos_version`) → 2. Verify health (`talos_health`) → 3. Snapshot etcd (`talos_etcd_snapshot`) → 4. Upgrade CP nodes one at a time (`talos_upgrade`) → 5. Upgrade workers → 6. Verify health

If an upgrade fails, use `talos_rollback` to revert to the previous version (A/B partition scheme).

### Upgrade Kubernetes
Done via machine config patch — update these image tags to the target K8s version:
- `cluster.apiServer.image` (kube-apiserver)
- `cluster.controllerManager.image` (kube-controller-manager)
- `cluster.scheduler.image` (kube-scheduler)
- `cluster.proxy.image` (kube-proxy)
- `machine.kubelet.image` (kubelet)

Apply the patch to all control plane nodes via `talos_apply_config`. Workers pick up kubelet changes when their config is updated. Monitor rollout via Kubernetes MCP.

### Scale Up
Generate worker config for the cluster, apply to new node. It joins automatically via discovery.

### Scale Down
For workers: `talos_reset(node)`. For CP nodes: first `talos_etcd_forfeit_leadership` (if leader), then `talos_etcd_leave` or `talos_etcd_remove_member(member_id)`, then `talos_reset(node)`.

### Reset
`talos_reset(node, graceful=true)` — wipes the node back to maintenance mode. Always remove from etcd first for CP nodes.

## Boot Assets & Images

Use the local **imager** container to build custom Talos images:

```bash
docker run --rm -t -v /dev:/dev --privileged \
  ghcr.io/siderolabs/imager:v1.12.0 \
  <output-type> \
  --system-extension-image ghcr.io/siderolabs/<extension>:latest
```

Output types: `iso`, `metal`, `disk-image`, `installer`, `aws`, `azure`, `gcp`, etc.

See `references/boot-assets.md` for extension list, overlay options, SecureBoot, and profiles.

## System Extensions

Extensions add functionality (drivers, tools, services) to Talos. They are baked into the boot image — not installed at runtime.

Three tiers: **core** (official, tested), **extra** (community, tested), **contrib** (community, best-effort).

Common extensions: `iscsi-tools`, `qemu-guest-agent`, `intel-ucode`, `amd-ucode`, `nvidia-container-toolkit`, `tailscale`, `drbd`.

Check installed: `talos_extensions`

## Networking

Talos networking is configured in `machine.network`. Key concepts:

- **Interfaces**: configured by `deviceSelector` (preferred) or name
- **Addressing**: static (`addresses` + `routes`) or DHCP
- **Bonds/Bridges/VLANs**: logical interfaces with `bond`, `bridge`, `vlans` config
- **VIPs**: shared virtual IPs for HA control plane (`vip.ip`)
- **WireGuard**: built-in support via `wireguard` interface config
- **KubeSpan**: Talos mesh networking across sites
- **Firewall**: ingress rules via `networkRuleConfig` resources

See `references/networking.md` for configuration patterns.

## etcd Operations

- List members: `talos_etcd_members`
- Status: `talos_etcd_status`
- Snapshot: `talos_etcd_snapshot(output_path)` — always do before upgrades/resets
- Defragment: `talos_etcd_defrag` — run on one node at a time, resource-heavy
- Remove member: `talos_etcd_remove_member(member_id)` — required before resetting a CP node
- Forfeit leadership: `talos_etcd_forfeit_leadership` — before maintenance on leader node
- Leave cluster: `talos_etcd_leave` — graceful removal
- Alarms: `talos_etcd_alarm` — check for NOSPACE or other alarms
- Recovery: bootstrap with `--recover-from=<snapshot-path>` (talosctl via Bash)

## Security

- All API access via mTLS (mutual TLS)
- Certificates managed by `trustd`, auto-rotated
- CA rotation: `talosctl gen secrets` (CLI required) → apply new config
- RBAC: `os:admin`, `os:reader`, `os:etcd:backup`, `os:impersonator` roles
- SELinux: enabled by default in enforcing mode

## Troubleshooting

When diagnosing issues, follow this order:

1. `talos_health` — overall cluster health
2. `talos_services` — check service states
3. `talos_logs(service)` — service-specific logs (kubelet, etcd, apid, machined)
4. `talos_dmesg` — kernel logs
5. `talos_etcd_members` + `talos_etcd_status` + `talos_etcd_alarm` — etcd health
6. Kubernetes MCP: pods, events, node status
7. `talos_memory`, `talos_cpu`, `talos_disk_usage` — resource pressure
8. `talos_interfaces`, `talos_addresses`, `talos_routes` — network state
9. `talos_volumes`, `talos_discovered_volumes` — storage state
10. `talos_time` — NTP sync status
11. `talos_read`, `talos_ls` — inspect files on node

See `references/troubleshooting.md` for common issues and solutions.

## Disaster Recovery

1. **etcd snapshot restore**: `talosctl bootstrap --recover-from=<snapshot>`
2. **Config backup**: always keep generated secrets/configs in a safe location
3. **Single CP node recovery**: reset and re-apply config, bootstrap if etcd is lost
4. **Multi-CP recovery**: restore from etcd snapshot on one node, other nodes rejoin
