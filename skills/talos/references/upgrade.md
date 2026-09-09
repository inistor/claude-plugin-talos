# talos_upgrade — response shape and failure modes

Docs: https://docs.siderolabs.com/talos/v1.14/

`talos_upgrade` collapses the whole talosctl upgrade flow into one call per node. This file documents
what it returns, so a failure can be read without guessing.

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
fields still tell you how far it got.

| `status` | Meaning | What to do |
|---|---|---|
| `k8s_discovery_failed` | Could not resolve or reach the Kubernetes node, so cordon and drain were impossible. **Nothing was installed.** | Fix Kubernetes access, or pass `skip_drain=true` to upgrade without evicting workloads. |
| `cordon_failed` | The node could not be cordoned. Nothing was installed. | Check Kubernetes API reachability and RBAC. |
| `drain_failed` | Eviction did not complete within the drain timeout, usually a PDB with `maxUnavailable: 0`. Nothing was installed; the node is uncordoned again. | Relax the PDB, or scale the blocking workload. |
| `install_failed` | The install itself failed — see `install_error` for the underlying message (image pull failure, unreachable registry, rejected upgrade RPC). | Read `install_error`. A 404 on `ghcr.io/siderolabs/installer` means you are still using the pre-1.14 image path. |
| `installed_no_reboot` | Install succeeded but the reboot was not performed. | Expected with `auto_reboot=false`. Otherwise call `talos_reboot`. |
| `rebooted_no_wait` | The node rebooted but did not come back within the wait window; see `wait_error`. | Check console/BMC. Slow bare-metal POST can exceed the wait. |
| `internal_error` | The install returned no result at all. | Retry; report if it persists. |

Additional fields that can appear alongside any status. Each is a signal on its own — a key ending
in `_error` marks the whole result as an error even when `status` is `"ok"`:

- `install_error` — the install's own message when it was not JSON-encoded
- `wait_error` — Talos did not become reachable again in time
- `k8s_ready_error` — the node came back but never reported Ready
- `uncordon_error` — the node is still cordoned and needs manual attention
- `k8s_discovery_error` — why Kubernetes could not be reached
- `hint` — the concrete next step, when there is an obvious one

## `auto_reboot=false`

Installs into the alternate A/B partition and updates META, then stops. No drain, no reboot, no
wait, no uncordon — the node keeps running the current version until you call `talos_reboot` (or
`talos_upgrade` again with `auto_reboot=true`).

Use it to install during business hours and activate in a maintenance window. Prefer it over the
legacy `stage: true`, which only exists on the pre-v1.13 path.

## `skip_drain=true`

Proceeds when the Kubernetes node cannot be discovered or reached. The node is installed and
rebooted **without evicting its workloads**. Appropriate for a node that is not a cluster member, or
one whose workloads you have already moved. Not appropriate as a way to get past a `drain_failed` —
that means a PDB is protecting something.

## Timeouts

Currently fixed, not exposed as parameters:

| Phase | Timeout |
|---|---|
| Drain | 5 minutes |
| Reboot wait | 10 minutes |
| Node Ready wait | 5 minutes |

A slow bare-metal node with a long POST can exceed the reboot wait and return `rebooted_no_wait`
even though the upgrade succeeded. Confirm with `talos_version` before assuming a failure.
