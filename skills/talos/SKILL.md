---
name: talos
description: |
  This skill should be used when working with Talos Linux clusters, talosctl, the Talos API, or the
  Talos MCP tools (talos_*). Covers machine configuration (v1alpha1 and the multi-document model),
  cluster bootstrap, Talos upgrades, Kubernetes version upgrades, boot asset building with imager,
  system extensions, networking (bonds, VLANs, VIPs, WireGuard, KubeSpan, BGP), storage (LVM, RAID,
  volumes), etcd maintenance, troubleshooting, and disaster recovery.
  Triggers for queries like "upgrade my Talos cluster", "build a custom Talos ISO with extensions",
  "etcd is unhealthy", "node won't join the cluster", "configure bonding on Talos",
  "bootstrap a new Talos cluster", "reset a Talos node", "add a worker node",
  "restore etcd from snapshot", "recover a failed control plane node", or
  "which talos MCP tools are available".
---

Targets **Talos Linux v1.14**. Documentation references point to https://docs.siderolabs.com/talos/v1.14/.
Differences against v1.13 are called out inline where they matter; anything not marked applies to both.

## v1.14 at a glance

| | v1.14 | v1.13 |
|---|---|---|
| Default Kubernetes | 1.37.0 | 1.36.0 |
| Supported Kubernetes | 1.32.0 – 1.37.99 | 1.31.0 – 1.36.99 |
| etcd | 3.7.1 (3.6 is the minimum) | 3.6.x |
| Installer image | Image Factory only | `ghcr.io/siderolabs/installer` |
| Config documents | 89 kinds | 44 kinds |

**The installer image moved.** `ghcr.io/siderolabs/installer` is no longer published as of v1.14 — the
tag simply 404s. Installer images now come from the Image Factory:

```
factory.talos.dev/metal-installer/<schematic-id>:v1.14.0
```

Use the schematic the node was installed from. The empty/default schematic is
`376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba`. `ghcr.io/siderolabs/imager` and
`ghcr.io/siderolabs/installer-base` are unaffected and still published.

**Upgrading to v1.14 is direct from 1.12 or 1.13** — no intermediate hop. See the upgrade section.

## Tool Usage Rules

Use Talos MCP tools — `mcp__talos__*` when the MCP is configured via user config, or
`mcp__plugin_talos_talos__*` when installed via the plugin marketplace — for all Talos operations.
Use Kubernetes MCP tools (`mcp__kubernetes-mcp-server__*`) for all Kubernetes operations. The tool
schemas are the authoritative list of what exists and what each parameter accepts; read them rather
than assuming.

Shell out only for the operations that genuinely have no MCP equivalent:

| Operation | Why |
|---|---|
| `talosctl gen secrets` / `gen config` | Config generation is client-side |
| `talosctl upgrade-k8s --to <version>` | Client-side orchestration interleaving Talos and Kubernetes APIs |
| `talosctl support` | Aggregates a full support bundle into an archive |
| `talosctl rotate-ca` | Multi-phase CA rotation across Talos and Kubernetes |
| `talosctl bootstrap --recover-from` | etcd restore from a snapshot |
| `talosctl validate` | Offline config validation |

Use `yq` or `jq` for parsing YAML/JSON. Avoid `grep` on structured data.

**Avoid large results.** MCP results that exceed the context window get dumped to temp files and
become unusable. Scope every query: filter by namespace, use label selectors, pass `tail_lines` on
logs, pass `filter` on `talos_logs` / `talos_dmesg`, and give `talos_get` a `resource_id` when you
want one object. If a result did land in a temp file, extract what you need from it with `jq` rather
than re-running the same broad query.

## Talosconfig

The client config lives at `~/.talos/config` (or `$TALOSCONFIG`) and holds contexts with endpoints
and TLS credentials.

Every node-aware tool takes an optional `node` (a **single string**) plus `context` and `endpoint`.
**There is no `nodes` array** — each call targets exactly one node, and passing one is rejected with
an explanatory error rather than silently ignored. To cover several nodes, issue one call per node.

Omitting `node` lets the request run on whichever endpoint apid picks (typically a control plane).
That is fine for cluster-wide reads like `talos_health` or `talos_etcd_members`, but per-node tools
(`talos_addresses`, `talos_disks`, `talos_read`, `talos_ls`, …) will then return the *endpoint's*
data, not the node you meant.

`endpoint` overrides the talosconfig's endpoints and connects directly to an address. Reach for it
when the configured control-plane endpoints are down — exactly when the talosconfig is least useful.

**Before any Talos operation**, check for a local `talosconfig` in the working directory or project
root. If found, base64-encode it (`base64 < talosconfig`) and pass that to `talos_set_config`.
Encoding preserves formatting; long base64 cert lines must not be wrapped.

**`x509: certificate signed by unknown authority` or `Ed25519 verification failure`** does not mean
the certificates are incompatible. It means the talosconfig does not match the cluster — wrong
config, stale config from a rebuilt cluster, or `talos_set_config` was never called.

**One cluster per session.** `talos_set_config` sets the config for all subsequent calls. To switch
clusters, call it again; to switch contexts within one talosconfig, use the `context` parameter.

## Talos Overview

Talos Linux is an immutable, API-driven, minimal Linux OS for Kubernetes. No SSH, no shell, no
package manager. Management is via the Talos API (port 50000) over mutual TLS. The OS is read-only
with an A/B partition scheme for atomic upgrades and rollback.

Key components: `machined` (init), `apid` (API gateway), `trustd` (certificate authority), `etcd`
(control plane only). v1.14 adds `sandboxd`, which runs CRI containerd, kubelet and all pods in a
dedicated PID and mount namespace — see "v1.14 behaviour changes" below.

## Machine Configuration

v1.14 completes a move from one monolithic v1alpha1 file to a **multi-document** model: 89 registered
document kinds, including 24 `Kube*Config` documents that replace the whole v1alpha1 `cluster:`
section, plus new storage (LVM, RAID), runtime (`SysctlConfig`, `EtcFileConfig`, `SecurityProfileConfig`)
and network (`BGPInstanceConfig`, `VethConfig`) documents.

v1alpha1 still works, but a config containing **both** a deprecated field and its replacement
document is **rejected**. (The `.machine.sysctls` / `.machine.sysfs` / `.machine.kernel.modules` /
`.machine.udev.rules` family is the exception — there the new document simply wins.)

Apply a full config with `talos_apply_config(config, mode)`; modes are `auto` (default),
`no-reboot`, `staged`, `try`, and the deprecated `reboot`. Use `insecure: true` for nodes in
maintenance mode. For partial changes use `talos_patch(patch, node)` — a strategic merge patch
against the live config, with `$patch: delete` to remove fields and `dry_run: true` to preview.

`talos_version`, `talos_disks` and `talos_apply_config` accept `insecure: true`. Other tools —
including `talos_get` — do not.

See `references/machine-config.md` for the full document map, the v1alpha1 → document migration
tables, registries, storage, and worked examples.

## Cluster Lifecycle

### Bootstrap
1. `talosctl gen secrets -o secrets.yaml` (Bash)
2. `talosctl gen config <cluster-name> <endpoint> --with-secrets secrets.yaml` (Bash)
3. Apply the control-plane config to each CP node: `talos_apply_config(config, node, insecure=true)` — fresh nodes are in maintenance mode
4. Apply the worker config to each worker the same way
5. `talos_bootstrap(node)` on **one** CP node only
6. `talos_kubeconfig` — retrieve the kubeconfig
7. `talos_health` — verify

Stop passing `insecure` once the machine config is applied.

### Upgrade Talos

`talos_upgrade` performs the whole talosctl-equivalent flow in one call per node:
**cordon → drain → install → reboot → wait → uncordon**, returning when the node is back in service.

- **Drain** uses the kubectl drain library, so it is PDB-aware, skips DaemonSet/mirror/static pods, and honours each pod's `terminationGracePeriodSeconds`.
- **Install** auto-detects `ImageService.Pull` + `LifecycleService.Upgrade` on v1.13+, or single-shot `MachineService.Upgrade` on older servers.
- **Reboot** is an explicit RPC on v1.13+ (LifecycleService is install-only); on older servers the legacy upgrade RPC reboots by itself.
- **Uncordon** runs at the end, and in a defer if an earlier phase failed, so a partial failure never leaves the node cordoned.

If Kubernetes cannot be reached the upgrade **aborts** rather than rebooting a node whose workloads
were never evicted. Pass `skip_drain=true` to override that deliberately.

**Procedure:**
1. Pre-flight: `talos_version`, `talos_health`, `talos_extensions`
2. `talos_etcd_snapshot` before any control-plane upgrade
3. Control-plane nodes one at a time: `talos_upgrade(node, image)`, then confirm with `talos_health` and `talos_etcd_members` before the next
4. Then workers (in parallel only if the workloads tolerate simultaneous reboots)
5. Post-upgrade: `talos_health`, `talos_version`, and `talos_extensions` — this last one catches the "upgraded to a stock image, lost the extensions" mistake

**Rules that bite:**
- **Version path**: upgrade through each intermediate minor (1.12 → 1.13 → 1.14). Upstream allows 1.12 and 1.13 to go directly to 1.14, but the conservative path is the latest patch of each minor.
- **Use a matching installer**: a stock image contains no extensions, so upgrading an extension-using cluster to one strips them on reboot. Build a matching schematic with `/talos-image` and pass that image.
- **Control-plane serialization**: Talos refuses an upgrade that would lose etcd quorum, so it serializes control-plane upgrades itself. Sequence them anyway so you can verify between nodes — and note a 2-node control plane loses quorum whenever one member is down, regardless.
- **Automatic rollback**: if the new system fails to boot, the A/B bootloader reverts on its own. `talos_rollback` is for reverting a *successful* but unwanted upgrade.
- **Install without activating**: `auto_reboot=false` installs into the alternate partition and updates META, leaving the node on the current version until you call `talos_reboot`. Prefer this over the legacy `stage: true`.

See `references/upgrade.md` for the full response shape and failure statuses.

### Upgrade Kubernetes

`talosctl upgrade-k8s --to <version>` via Bash. This stays client-side in v1.14 — LifecycleService
covers the Talos OS install only, not the Kubernetes control-plane rollout, which interleaves Talos
and Kubernetes API calls. Do not try to reproduce it with `talos_patch`. Preview with `--dry-run`;
the command is resumable if interrupted.

### Scale up
Generate a worker config and apply it to the new node. It joins automatically via discovery.

### Scale down / reset
`talos_reset(node, graceful=true)` cordons and drains, leaves etcd if the node is a control plane,
wipes disks and powers down. Then remove the Kubernetes node object with
`mcp__kubernetes-mcp-server__resources_delete`.

Manual etcd departure (`talos_etcd_forfeit_leadership`, `talos_etcd_leave`,
`talos_etcd_remove_member`) is only needed for **non-graceful** resets or when a node is already
gone — a graceful reset handles it.

## Boot Assets & Images

Build custom images with the local **imager** container. It runs rootless, so `--privileged` and
`-v /dev:/dev` are only needed for bootable-media profiles that use loop devices; the `installer`
profile does not need them. Always bind-mount a host directory to `/out`.

```bash
mkdir -p _out
docker run --rm -t \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 \
  <profile> \
  --system-extension-image ghcr.io/siderolabs/<extension>:<tag>
```

Common profiles: `iso`, `metal`, `metal-uki`, `installer`, their `secureboot-*` variants, and the
cloud targets (`aws`, `azure`, `gcp`, `nocloud`, `vmware`, …). There is **no** `disk-image` profile.

Only kernel-module extensions embed the Talos version in their tag; userspace ones are versioned
independently. Look up the exact tag rather than guessing:

```bash
crane export ghcr.io/siderolabs/extensions:v1.14.0 | tar x -O image-digests | grep <extension-name>
```

See `references/boot-assets.md` for the full profile list, output formats, extension tables,
SecureBoot, SBC overlays, and the installer load/tag/push flow.

### On-node image cache

Each node runs two containerd instances with separate image stores:

- **`cri`** (default) — kubelet's containerd, namespace `k8s.io`. Holds Kubernetes workload images; old ones accumulate because kubelet GC is watermark-driven. This is the usual prune target.
- **`system`** — Talos's own containerd. Holds the installer image and system extensions. Do **not** remove the installer image for the running version — `talos_rollback` needs it.

Use `talos_image_list`, `talos_image_remove`, and `talos_image_prune` (which defaults to
`dry_run=true` — always preview first).

containerd indexes each image under **three** refs: the tag, the digest, and the raw content ID.
Removing only the tag frees no space, because the other two still pin the blob. `talos_image_prune`
handles all three, which is why it is the right tool for reclaiming space; `talos_image_remove` is
the right tool for forcing a re-pull of a specific tag.

## System Extensions

Extensions add drivers, tools and services. They are baked into the boot image, not installed at
runtime. Three tiers: **core** (official), **extra** (community, tested), **contrib** (best-effort).

Common: `iscsi-tools`, `qemu-guest-agent`, `intel-ucode`, `amd-ucode`, `tailscale`, `drbd`,
`nvidia-container-toolkit-lts` / `-production` (the unsuffixed name no longer exists).

Check what a node has with `talos_extensions`.

## Networking

Configured through standalone documents — `LinkConfig`, `BondConfig`, `BridgeConfig`, `VLANConfig`,
`WireguardConfig`, `DHCPv4Config`, `Layer2VIPConfig`, `HostnameConfig`, `ResolverConfig`,
`KubeSpanConfig`, `NetworkRuleConfig`, and in v1.14 `BGPInstanceConfig`, `VethConfig` and
`HTTPProbeConfig`. The entire `machine.network` tree is deprecated.

Two traps worth knowing before you touch a running cluster:

- Adding **any** link or DHCP document disables the default "DHCP on all physical links" behaviour **globally**, not just for the link you configured. One `LinkConfig` can silently drop DHCP on every other NIC.
- With `NetworkDefaultActionConfig` set to `ingress: block`, `NetworkRuleConfig` documents are an allowlist; with `ingress: accept` the same documents invert into a blocklist.

See `references/networking.md` for CEL link selectors, the migration table, the firewall model, and
worked examples.

## etcd Operations

| Task | Tool |
|---|---|
| List members | `talos_etcd_members` |
| Status / alarms | `talos_etcd_status`, `talos_etcd_alarm` |
| Snapshot | `talos_etcd_snapshot` — do this before upgrades and resets |
| Defragment | `talos_etcd_defrag` — one node at a time, resource-heavy |
| Forfeit leadership | `talos_etcd_forfeit_leadership` — before maintenance on the leader |
| Leave / remove | `talos_etcd_leave`, `talos_etcd_remove_member` — non-graceful resets only |
| Restore | `talosctl bootstrap --recover-from=<snapshot>` (Bash) |

`talos_etcd_remove_member` takes the member ID **as a decimal string** — IDs are uint64 and exceed
what a JSON number represents exactly. Take the value from `talos_etcd_members`.

The MCP server usually runs in an ephemeral container, so `talos_etcd_snapshot` writes to `/out` and
refuses any path that would be discarded on exit. Bind-mount a host directory there.

## Security

- All API access over mTLS; certificates are managed and rotated by `trustd`.
- CA rotation: `talosctl rotate-ca` (Bash) — it handles both the Talos and Kubernetes CAs. Do not hand-roll this with `gen secrets`.
- RBAC roles: `os:admin`, `os:operator`, `os:reader`, `os:etcd:backup`, `os:impersonator`.
- SELinux is enabled and enforcing by default.

## v1.14 behaviour changes

Worth checking before and after an upgrade:

- **Workload isolation (`sandboxd`)** is on by default for **new** clusters and off for **upgraded** ones (the document is simply absent). With it on, the in-tree Kubernetes `iscsi` volume plugin cannot work — kubelet cannot reach host `iscsid` across the sandbox PID namespace. Use `csi-driver-iscsi` or `democratic-csi`.
- **Filesystem trim** follows the same new-on / upgraded-off asymmetry.
- **etcd metrics and health moved from port 2379 to 2383.** Prometheus scrapes and firewall rules pointing at 2379 break silently.
- **`multipath-tools` needs a migration applied *before* upgrading**, or `multipathd` hangs forever waiting for a config file that no longer ships. See `references/troubleshooting.md`.
- **`talosctl support` output is age-encrypted by default**; pass `--no-encryption` for the old behaviour.
- **NRI is no longer disabled** for CRI containerd.
- **`net.ipv4.conf.{all,default}.send_redirects = 0`** by default — breaks nodes acting deliberately as L3 gateways.
- **TLS 1.3 minimum** on etcd and kube-apiserver; custom cipher-suite settings are ignored.

## Troubleshooting

Work in this order:

1. `talos_get(resource_type="diagnostics")` — Talos's own known-problem warnings, the cheapest first look
2. `talos_health` — overall cluster health
3. `talos_services` — service states
4. `talos_logs(service, filter)` and `talos_dmesg(filter)` — always pass `filter`
5. `talos_etcd_members`, `talos_etcd_status`, `talos_etcd_alarm`
6. Kubernetes MCP: pods, events, node status
7. `talos_memory`, `talos_cpu`, `talos_disk_usage` — resource pressure
8. `talos_interfaces`, `talos_addresses`, `talos_routes` — network state
9. `talos_volumes`, `talos_discovered_volumes` — storage state
10. `talos_time` — NTP sync
11. `talos_read`, `talos_ls` — inspect files on the node

For a full bundle to attach to a bug report: `talosctl support` (Bash).

See `references/troubleshooting.md` for specific failures and their fixes.

## Disaster Recovery

1. **Try to restore quorum first** — far simpler than a full recovery.
2. **etcd snapshot restore**:
   - Wipe the EPHEMERAL partition: `talos_reset(node, graceful=false, reboot=true, system_labels_to_wipe="EPHEMERAL")`
   - Wait for etcd to reach "Preparing"
   - `talosctl bootstrap --recover-from=<snapshot>` (Bash)
   - Add `--recover-skip-hash-check` if the snapshot was copied off disk rather than taken with `talos_etcd_snapshot`
3. **When quorum is lost and a normal snapshot fails**: `talosctl cp /var/lib/etcd/member/snap/db .` (Bash)
4. **Keep generated secrets and configs somewhere safe** — they cannot be regenerated.
5. **Single control plane**: reset, re-apply config, bootstrap if etcd is gone.
6. **Multiple control planes**: restore from a snapshot on one node; the others rejoin.
