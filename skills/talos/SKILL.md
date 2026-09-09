---
name: talos
description: >-
  This skill should be used when working with Talos Linux (v1.14 / v1.13) clusters, talosctl, the
  Talos API, or the Talos MCP tools (talos_*). Use for queries like "upgrade my Talos cluster",
  "what changed in Talos 1.14", "build a custom Talos ISO with extensions", "etcd is unhealthy",
  "node won't join the cluster", "configure bonding / a VLAN / a VIP on Talos", "set up BGP on
  Talos", "add a user volume / LVM / RAID", "configure a registry mirror", "block ingress ports on
  a node", "bootstrap a new Talos cluster", "reset a Talos node", "restore etcd from a snapshot",
  "recover a failed control plane node", or "which talos MCP tools are available".
version: 1.14.0
---

Targets **Talos Linux v1.14**, with v1.13 differences noted where they matter.
Upstream reference: https://docs.siderolabs.com/talos/v1.14/

This file carries only what is needed on every call. Load one topic file from the routing table
below for anything deeper — the topic files hold the worked examples and the traps.

## Routing table

| Task or symptom | Read |
|---|---|
| Bootstrap a new cluster | `references/operations/bootstrap.md` |
| Upgrade Talos on a node; read an upgrade failure | `references/operations/upgrade-talos.md` |
| Upgrade Kubernetes | `references/operations/upgrade-kubernetes.md` |
| Add or remove a node, reset a node | `references/operations/scale-and-reset.md` |
| etcd maintenance, snapshots, quorum loss, disaster recovery | `references/operations/etcd-and-recovery.md` |
| A config apply was rejected; patching; apply modes | `references/config/config-model.md` |
| Move off deprecated v1alpha1 fields | `references/config/config-migration.md` |
| Configure apiserver, kubelet, CoreDNS, kube-proxy, CNI, manifests | `references/config/kubernetes-documents.md` |
| User volumes, LVM, RAID, encryption, trim/scrub | `references/config/storage-volumes.md` |
| Registry mirrors, pull-through cache, registry auth | `references/config/registries.md` |
| A NIC lost DHCP after a config change; link selection | `references/networking/links.md` |
| Bond, bridge, VLAN, VRF, dummy, veth, WireGuard | `references/networking/logical-links.md` |
| BGP, virtual IPs, KubeSpan | `references/networking/bgp-and-vips.md` |
| Ingress firewall rules | `references/networking/firewall.md` |
| DNS, DoT/DoH, hostname, NTP/NTS | `references/networking/dns-and-time.md` |
| Build an ISO, installer or disk image | `references/images/boot-assets.md` |
| Choose or pin a system extension | `references/images/extensions.md` |
| Reclaim disk space from cached images | `references/images/image-cache.md` |
| Diagnose a failure | `references/troubleshooting.md` |
| Behaviour that changed in v1.14 | `references/v1.14-changes.md` |

## v1.14 at a glance

| | v1.14 | v1.13 |
|---|---|---|
| Default Kubernetes | 1.37.0 | 1.36.0 |
| Supported Kubernetes | 1.32.0 – 1.37.99 | 1.31.0 – 1.36.99 |
| etcd | 3.7.1 (3.6 minimum) | 3.6.x |
| Installer image | Image Factory only | `ghcr.io/siderolabs/installer` |
| Config document kinds | 90 | 45 |

**`ghcr.io/siderolabs/installer` is no longer published as of v1.14** — the tag 404s. Installer
images come from `factory.talos.dev/metal-installer/<schematic-id>:v1.14.0`. The default schematic
id and the rules for choosing one live in `references/images/boot-assets.md`. `imager` and
`installer-base` are unaffected.

Upgrading to v1.14 is direct from 1.12 or 1.13. Behaviour that changed under operators' feet —
sandboxd, filesystem trim, the etcd metrics port, multipath — is in `references/v1.14-changes.md`;
check it before and after any upgrade.

## Tool usage rules

Use Talos MCP tools (`mcp__talos__*`, or `mcp__plugin_talos_talos__*` when installed as a plugin)
for Talos operations, and Kubernetes MCP tools (`mcp__kubernetes-mcp-server__*`) for Kubernetes
operations. **The tool schemas are authoritative** for which tools exist and what each parameter
accepts — read them rather than assuming a capability is missing.

Shell out only for these, which have no MCP equivalent. Using them is correct, not a fallback:

| Operation | Command |
|---|---|
| Config generation | `talosctl gen secrets`, `talosctl gen config` |
| Kubernetes upgrade | `talosctl upgrade-k8s --to <version>` |
| Support bundle | `talosctl support` |
| CA rotation | `talosctl rotate-ca` |
| etcd restore | `talosctl bootstrap --recover-from` |
| Offline config validation | `talosctl validate` |

Parse YAML and JSON with `yq` or `jq`, never `grep`.

**Avoid large results.** An MCP result that exceeds the context window is dumped to a temp file and
becomes unusable. Scope every query: filter by namespace, use label selectors, pass `tail_lines` on
logs, pass `filter` on `talos_logs` and `talos_dmesg`, and give `talos_get` a `resource_id` when a
single object is wanted. When a result has already landed in a temp file, extract the needed fields
from it with `jq` instead of re-running a broad query.

## Talosconfig and node targeting

The client config lives at `~/.talos/config` (or `$TALOSCONFIG`) and holds contexts with endpoints
and TLS credentials.

Every node-aware tool takes an optional `node` (a **single string**), plus `context` and `endpoint`.
**There is no `nodes` array.** Passing one is rejected with an explanatory error rather than
silently ignored. To cover several nodes, issue one call per node.

Omitting `node` runs the request on whichever endpoint apid picks, typically a control plane. That
is correct for cluster-wide reads such as `talos_health` or `talos_etcd_members`, but per-node tools
(`talos_addresses`, `talos_disks`, `talos_read`, `talos_ls`, …) then return the *endpoint's* data
rather than the intended node's.

`endpoint` overrides the talosconfig endpoints and dials an address directly. Use it when the
configured control-plane endpoints are down — the recovery case, and exactly when the talosconfig is
least useful.

**Which talosconfig is used is fixed when the server starts** — from `TALOSCONFIG` if set, otherwise
`~/.talos/config`. The server holds no per-session state and there is no tool to change the config
mid-session. Select a context within the configured file per call with `context`.

To target a cluster whose talosconfig is a different file, point `TALOSCONFIG` at it in the server's
launch configuration, or add a second MCP server entry — the same pattern the Kubernetes MCP servers
use with `KUBECONFIG`. Note the server usually runs in a container that mounts only `~/.talos`, so a
talosconfig outside that directory is not visible to it.

Inspect what the server is actually using with `talos_config_info`, which reports the resolved path,
the current context, and each context's endpoints and certificate expiry.

**`x509: certificate signed by unknown authority` or `Ed25519 verification failure` does not mean
the certificates are incompatible.** It means the talosconfig does not match the cluster — wrong
file in `TALOSCONFIG`, a stale config from a rebuilt cluster, or the wrong `context` selected.

## Talos overview

Talos Linux is an immutable, API-driven, minimal Linux OS for Kubernetes. No SSH, no shell, no
package manager. Management is via the Talos API on port 50000 over mutual TLS. The OS is read-only
with an A/B partition scheme for atomic upgrades and rollback.

Components: `machined` (init), `apid` (API gateway), `trustd` (certificate authority), `etcd`
(control plane only), and in v1.14 `sandboxd`, which isolates CRI containerd, kubelet and all pods.

Machine configuration is a stream of YAML documents. v1.14 registers 90 document kinds, and the
v1alpha1 `cluster:` section is deprecated in favour of 24 `Kube*Config` documents. A config carrying
both a deprecated field and its replacement document is **rejected** — see
`references/config/config-model.md`.

## Upgrade rules that bite

Full procedure and failure statuses: `references/operations/upgrade-talos.md`.

- `talos_upgrade` performs cordon → drain → install → reboot → wait → uncordon in one call per node. Do not reimplement those steps.
- A stock image contains no extensions, so upgrading an extension-using cluster to one strips them on reboot. Use the matching schematic.
- Talos refuses an upgrade that would lose etcd quorum, so control-plane upgrades serialize themselves. Sequence them anyway to allow verification between nodes. A 2-node control plane has no margin at all.
- If the upgraded system fails to boot, the A/B bootloader reverts automatically. `talos_rollback` is for reverting a *successful* but unwanted upgrade.
- `auto_reboot=false` installs into the alternate partition without activating it.

## Security

- All API access over mTLS; `trustd` manages and rotates certificates.
- Rotate CAs with `talosctl rotate-ca`, which handles both the Talos and Kubernetes CAs. Do not hand-roll this with `gen secrets`.
- RBAC roles: `os:admin`, `os:operator`, `os:reader`, `os:etcd:backup`, `os:impersonator`.
- SELinux is enabled and enforcing by default.
