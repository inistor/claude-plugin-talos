# Upgrading Talos

`talos_upgrade` collapses the whole talosctl upgrade flow into **one call per node**:
**cordon → drain → install → reboot → wait → uncordon**, returning only when the node is back in
service. Do not reimplement those phases with separate tools.

- **Drain** uses the kubectl drain library, so it is PDB-aware, skips DaemonSet, mirror and static pods, and honours each pod's `terminationGracePeriodSeconds`.
- **Install** auto-detects `ImageService.Pull` + `LifecycleService.Upgrade` on v1.13+ servers, or single-shot `MachineService.Upgrade` on older ones.
- **Reboot** is an explicit RPC on v1.13+ (LifecycleService is install-only); on older servers the legacy upgrade RPC reboots by itself.
- **Uncordon** runs at the end, and in a defer when an earlier phase failed, so a partial failure never leaves a node cordoned.

If Kubernetes cannot be reached the upgrade **aborts** rather than rebooting a node whose workloads
were never evicted. `skip_drain=true` overrides that deliberately.

## Before starting

**Upgrade path.** v1.14 is reachable **directly from 1.12 or 1.13** — no intermediate hop. 1.11
and older must hop through intermediate minors. Upstream still recommends the latest patch of each
minor as the conservative route. `references/v1.14-changes.md` has the detail, plus the behaviour
changes to check before and after.

**Choosing the installer image.** `ghcr.io/siderolabs/installer` no longer exists in v1.14; the
image comes from the Image Factory or a self-built installer. A **stock image carries no
extensions**, so upgrading an extension-using cluster to one strips them on reboot. Find the
schematic each node was installed from and pass a matching image — see
`references/images/boot-assets.md`.

## Procedure

1. Pre-flight: `talos_version`, `talos_health`, `talos_extensions` on each node.
2. `talos_etcd_snapshot` before touching any control-plane node.
3. Control-plane nodes **one at a time**: `talos_upgrade(node, image)`, then `talos_health` and `talos_etcd_members` before moving to the next.
4. Then workers — in parallel only if the workloads tolerate simultaneous reboots.
5. Post-upgrade: `talos_health`, `talos_version`, and `talos_extensions` again to confirm nothing was lost.

**Control-plane serialization.** Talos refuses an upgrade that would lose etcd quorum, so it
serializes control-plane upgrades itself. Sequence them anyway, to allow verification between
nodes. A two-node control plane loses quorum whenever one member is down, regardless.

**Rollback.** If the upgraded system fails to boot, the A/B bootloader reverts on its own.
`talos_rollback` reverts a *successful* but unwanted upgrade, and needs the previous installer
image still in the `system` image store — see `references/images/image-cache.md`.

## Success response (`auto_reboot=true`)

```json
{
  "status": "ok",
  "api": "lifecycle",
  "server_tag": "v1.13.10",
  "pulled_image": "factory.talos.dev/metal-installer/3765679...@sha256:...",
  "rebooted": true,
  "talos_back": true,
  "k8s_ready": true,
  "uncordoned": true,
  "k8s_node_name": "worker-01",
  "stages": ["cordoned node worker-01", "evicting pod app/web-0", "node worker-01 drained", "..."]
}
```

| Field | Meaning |
|---|---|
| `status` | `"ok"` only on the full success path. Any other value, or its absence, marks the result as an error. |
| `api` | `"lifecycle"` on v1.13+ servers, `"legacy"` on older ones |
| `server_tag` | Talos tag detected **before** the upgrade |
| `pulled_image` | Digest-pinned image actually installed (v1.13+ only) |
| `rebooted` / `talos_back` | Reboot was issued / the node answered the API again |
| `k8s_ready` | Node reported Ready to the Kubernetes API |
| `uncordoned` | Node was returned to service |
| `k8s_node_name` | Resolved Kubernetes node name; absent when the node is not a cluster member |
| `stages` | Ordered progress log — the most useful field when something stalls |

## Failure statuses

The response always carries whatever was observed before the failure, so `stages` and the boolean
fields still show how far it got.

| `status` | Meaning | Remediation |
|---|---|---|
| `k8s_discovery_failed` | Could not resolve or reach the Kubernetes node, so cordon and drain were impossible. **Nothing was installed.** | Fix Kubernetes access, or pass `skip_drain=true` to upgrade without evicting workloads. |
| `cordon_failed` | The node could not be cordoned. Nothing was installed. | Check Kubernetes API reachability and RBAC. |
| `drain_failed` | Eviction did not complete within the drain timeout, usually a PDB with `maxUnavailable: 0`. Nothing was installed; the node is uncordoned again. | Relax the PDB, or scale the blocking workload. |
| `install_failed` | The install itself failed — image pull failure, unreachable registry, or a rejected upgrade RPC. | Read `install_error`. A 404 on `ghcr.io/siderolabs/installer` means the pre-1.14 image path is still in use. |
| `installed_no_reboot` | Install succeeded but no reboot was performed. | Expected with `auto_reboot=false`. Otherwise call `talos_reboot`. |
| `rebooted_no_wait` | The node rebooted but did not answer within the wait window; see `wait_error`. | Check the console or BMC. Slow bare-metal POST can exceed the wait — confirm with `talos_version` before treating it as a failure. |
| `internal_error` | The install returned no result at all. | Retry; report if it persists. |
| `failed` | The LifecycleService upgrade stream completed but the installer returned a non-zero `exit_code`, which is reported alongside. The image was pulled successfully; the install itself rejected it. | Read `messages` for the installer output — a schematic that does not match the node's platform or secure-boot state is the usual cause. |

Additional fields can appear alongside any status. Each is a signal on its own — **a key ending in
`_error` marks the whole result as an error even when `status` is `"ok"`**:

- `install_error` — the install's own message when it was not JSON-encoded
- `wait_error` — Talos did not become reachable again in time
- `k8s_ready_error` — the node came back but never reported Ready
- `uncordon_error` — the node is still cordoned and needs manual attention
- `k8s_discovery_error` — why Kubernetes could not be reached
- `hint` — the concrete next step, when there is an obvious one

## `auto_reboot=false`

Installs into the alternate A/B partition and updates META, then stops. No drain, no reboot, no
wait, no uncordon — the node keeps running the current version until `talos_reboot` is called, or
`talos_upgrade` runs again with `auto_reboot=true`. `installed_no_reboot` is the expected status
here, not an error.

Use it to install during business hours and activate in a maintenance window. Prefer it over the
legacy `stage: true`, which exists only on the pre-v1.13 path.

## `skip_drain=true`

Proceeds when the Kubernetes node cannot be discovered or reached, installing and rebooting
**without evicting workloads**.

Appropriate for a node that is not a cluster member, or one whose workloads have already been
moved. **Not** appropriate as a way past `drain_failed` — that status means a PDB is protecting
something, and skipping the drain destroys the availability the PDB was written to keep.

## Timeouts

Fixed, not exposed as parameters:

| Phase | Timeout |
|---|---|
| Drain | 5 minutes |
| Reboot wait | 10 minutes |
| Node Ready wait | 5 minutes |

A slow bare-metal node with a long POST can exceed the reboot wait and return `rebooted_no_wait`
even though the upgrade succeeded. Confirm with `talos_version` before assuming a failure.
