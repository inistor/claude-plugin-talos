---
name: talos-linux
description: |
  This skill should be used when working with Talos Linux clusters, talosctl, or the Talos API.
  Covers machine configuration (v1alpha1), cluster bootstrap, Talos upgrades, Kubernetes version
  upgrades, boot asset building with imager, system extensions, networking (bonds, VLANs, VIPs,
  WireGuard, KubeSpan), etcd maintenance, troubleshooting, and disaster recovery.
  Triggers for queries like "upgrade my Talos cluster", "build a custom Talos ISO with extensions",
  "etcd is unhealthy", "node won't join the cluster", "configure bonding on Talos",
  "bootstrap a new Talos cluster", "reset a Talos node", "add a worker node",
  "restore etcd from snapshot", or "recover a failed control plane node".
version: 1.13.2
---

This skill covers Talos Linux v1.13. All documentation references point to https://docs.siderolabs.com/talos/v1.13/.

## Tool Usage Rules

Use Talos MCP tools — `mcp__talos__*` when the MCP is configured via user config, or `mcp__plugin_talos_talos__*` when installed via the plugin marketplace — for all Talos operations. Never shell out to `talosctl` unless the MCP tool is unavailable.

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
- `talosctl upgrade-k8s --to <version>` — Kubernetes version upgrade (complex orchestration: patches configs, pre-pulls images, monitors rollout across all nodes)

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
**Node**: `talos_apply_config`, `talos_patch`, `talos_reboot`, `talos_shutdown`, `talos_reset`, `talos_upgrade`, `talos_rollback`, `talos_wipe`
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

Apply configs: `talos_apply_config(config, mode)` — requires the FULL machine configuration YAML. For partial changes, use `talos_patch` instead. Modes: `auto` (default), `no-reboot`, `reboot`, `staged`, `try`. Use `insecure: true` for nodes in maintenance mode.

Modify running configs with `talos_patch(patch, node)` — applies a strategic merge patch to the node's live config. Use `$patch: delete` to remove fields. Use `dry_run: true` to preview changes. See `references/machine-config.md` for full v1alpha1 structure.

## Cluster Lifecycle

### Bootstrap
1. `talosctl gen secrets -o secrets.yaml` (via Bash)
2. `talosctl gen config <cluster-name> <endpoint> --with-secrets secrets.yaml` (via Bash)
3. Apply CP config to each CP node: `talos_apply_config(config, node, insecure=true)` — fresh nodes are in maintenance mode, requires `insecure: true`
4. Apply worker config to each worker: `talos_apply_config(config, node, insecure=true)`
5. `talos_bootstrap(node)` on ONE CP node only
6. `talos_kubeconfig` — retrieve kubeconfig (needed for health check)
7. `talos_health` — verify cluster is up

**Note**: `talos_version`, `talos_disks`, `talos_get`, and `talos_apply_config` support `insecure: true` for nodes in maintenance mode. Stop using `insecure` once the machine config is applied.

### Upgrade Talos

`talos_upgrade` is a single tool call that does the full talosctl-equivalent flow per node: **cordon → drain → install → reboot → wait → uncordon**. It returns when the node is fully back in service.

Internally, with `auto_reboot=true` (default):
- **Drain** uses the kubectl drain library (`k8s.io/kubectl/pkg/drain`) — same code path as `kubectl drain` and `talosctl upgrade --drain=true`. PDB-aware (retries blocked evictions), skips DaemonSet/mirror/static pods, evicts emptyDir-using pods (node is rebooting anyway), uses each pod's own `terminationGracePeriodSeconds`.
- **Install** auto-detects: `ImageService.Pull` + `LifecycleService.Upgrade` on v1.13+ servers, or single-shot `MachineService.Upgrade` on older ones.
- **Reboot** is the explicit `Reboot` RPC on v1.13+ (LifecycleService is install-only), or implied by the legacy upgrade RPC on <v1.13.
- **Wait** polls `c.Version` with a saw-down-then-saw-up rule to detect the reboot, then polls the Kubernetes API for the node's Ready condition.
- **Uncordon** runs at the end (or in defer on early failure) so the node never stays cordoned because of a partial failure.

K8s steps are **skipped gracefully** if the target isn't a Kubernetes member (e.g. fresh node not yet joined) — the upgrade still runs.

**Steps:**
1. Pre-flight: `talos_version`, `talos_health`, list installed extensions (`talos_extensions`)
2. Etcd snapshot: `talos_etcd_snapshot` (recommended before any CP upgrade)
3. For each control plane node (one at a time):
   a. `talos_upgrade(node, image)` — does the full cycle, returns when the node is back and uncordoned.
   b. Verify (`talos_health`, `talos_etcd_members`) before moving on.
4. Repeat for workers (may run in parallel only if workloads tolerate simultaneous reboots).
5. Post-upgrade verification: `talos_health`, `talos_version`, `talos_extensions` (catches the "upgraded to stock image, lost extensions" mistake).

**Response shape** (on `auto_reboot=true` success path):
- `api`: `"lifecycle"` (v1.13+) or `"legacy"` (<v1.13)
- `server_tag`: detected pre-upgrade Talos tag
- `pulled_image`: canonical digest-pinned image (v1.13+ only)
- `rebooted: true`, `talos_back: true`, `k8s_ready: true`, `uncordoned: true`
- `k8s_node_name`: resolved Kubernetes node name (only when the target is a K8s member)
- `stages`: ordered list of progress messages (`"cordoning node X"`, `"evicting pod Y/Z"`, `"talos reachable again"`, etc.)
- `status`: `"ok"`

If something fails mid-flow, the response still includes everything observed (`rebooted`, `talos_back`, etc. as far as they got) plus an error field (`wait_error`, `k8s_ready_error`, `uncordon_error`) and a status like `"rebooted_no_wait"` or `"installed_no_reboot"`. The uncordon is always attempted on a best-effort basis.

**`auto_reboot=false` — install without activating:**

Pass `auto_reboot=false` when you want to stage the install but defer activation (install during business hours, reboot during a maintenance window). This skips drain, reboot, wait, and uncordon — just the install runs. The new version is written to the alternate A/B partition with META updated; the node keeps running the current version until you trigger `talos_reboot` (or another `talos_upgrade` call with `auto_reboot=true`) manually.

**Important upgrade rules:**
- **Version path**: Must upgrade through all intermediate minor releases (e.g., 1.11 → 1.12 → 1.13, not 1.11 → 1.13 directly)
- **Image is pre-pulled on v1.13+** — `talos_upgrade` first invokes `ImageService.Pull` into the system containerd, then `LifecycleService.Upgrade` with the resolved (digest-pinned) image. Response includes `pulled_image` with the canonical reference.
- **Custom installer for extensions**: Stock installer images contain no extensions — upgrading a cluster that uses extensions to a stock image will strip them on reboot. Build a matching custom installer via `/talos-image` first and pass that image to `talos_upgrade`.
- **CP serialization**: Talos automatically serializes CP upgrades and refuses if etcd quorum would be lost — no need to manually enforce one-at-a-time.
- **Automatic rollback**: If the upgraded system fails to boot, the A/B bootloader automatically reverts. Manual `talos_rollback` is for reverting a successful but unwanted upgrade.
- **Staged upgrades**: `stage: true` (legacy <v1.13 primitive) defers the install to next reboot. Prefer the cross-version `auto_reboot: false` for the same effect on either path.

### Upgrade Kubernetes
Use `talosctl upgrade-k8s --to <version>` via Bash. This is a complex client-side orchestration that patches all nodes' configs, pre-pulls images, and monitors rollout. As of v1.13 this remains client-side — the new `LifecycleService` API covers Talos OS install/upgrade only, not the Kubernetes control-plane upgrade flow (which interleaves Talos API and Kubernetes API calls). Do NOT attempt to replicate this manually with `talos_patch` — use the talosctl command directly. Use `--dry-run` first to preview the plan. The command is resumable if interrupted.

### Scale Up
Generate worker config for the cluster, apply to new node. It joins automatically via discovery.

### Scale Down
For workers: `talos_reset(node)`, then `kubectl delete node <name>` via Bash. For CP nodes: `talos_reset(node, graceful=true)` handles etcd departure automatically, then `kubectl delete node <name>`. Manual etcd leave/remove (`talos_etcd_forfeit_leadership`, `talos_etcd_leave`) is only needed for non-graceful resets or edge cases.

### Reset
`talos_reset(node, graceful=true)` — cordon/drain, leave etcd if CP node, wipe disks, power down. After reset, delete the Kubernetes node object: `kubectl delete node <name>` via Bash.

## Boot Assets & Images

Use the local **imager** container to build custom Talos images. As of v1.13 the imager runs rootless, so `--privileged` and `-v /dev:/dev` are only needed for bootable-media profiles (iso, metal, disk-image, cloud) that use loop devices. The `installer` profile does not need them. Always bind-mount a host directory to `/out` for output.

```bash
mkdir -p _out
docker run --rm -t \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.13.2 \
  <output-type> \
  --system-extension-image ghcr.io/siderolabs/<extension>:<tag>
```

Extension tags must match the Talos version — never use `:latest`. Look up matching tags at https://github.com/siderolabs/extensions/releases.

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
- **KubeSpan**: Talos mesh networking across sites — configured via the `KubeSpanConfig` document as of v1.13 (`.machine.network.kubespan` is deprecated but still works); `excludeAdvertisedNetworks` filters which CIDRs are advertised
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
3. `talos_logs(service, filter)` — service-specific logs, use `filter` to search for specific text
4. `talos_dmesg(filter)` — kernel logs, use `filter` to search for specific drivers/errors
5. `talos_etcd_members` + `talos_etcd_status` + `talos_etcd_alarm` — etcd health
6. Kubernetes MCP: pods, events, node status
7. `talos_memory`, `talos_cpu`, `talos_disk_usage` — resource pressure
8. `talos_interfaces`, `talos_addresses`, `talos_routes` — network state
9. `talos_volumes`, `talos_discovered_volumes` — storage state
10. `talos_time` — NTP sync status
11. `talos_read`, `talos_ls` — inspect files on node

See `references/troubleshooting.md` for common issues and solutions.

## Disaster Recovery

1. **Before full DR**: Check if etcd quorum can be restored first (simpler than full recovery)
2. **etcd snapshot restore**:
   - Wipe EPHEMERAL partition: `talos_reset(node, graceful=false, reboot=true, system_labels_to_wipe="EPHEMERAL")`
   - Wait for etcd to reach "Preparing" state
   - `talosctl bootstrap --recover-from=<snapshot>` (via Bash)
   - If snapshot was copied from disk (not via `talos_etcd_snapshot`), add `--recover-skip-hash-check`
3. **Alternative snapshot method** (when quorum is lost and normal snapshot fails): `talosctl cp /var/lib/etcd/member/snap/db .` via Bash
4. **Config backup**: always keep generated secrets/configs in a safe location
5. **Single CP node recovery**: reset and re-apply config, bootstrap if etcd is lost
6. **Multi-CP recovery**: restore from etcd snapshot on one node, other nodes rejoin
