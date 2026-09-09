# Bootstrapping a cluster

Brings a set of freshly booted, unconfigured nodes up as a working cluster. Upstream:
https://docs.siderolabs.com/talos/v1.14/

## Maintenance mode

A node booted from an ISO or a fresh disk image, with no machine config, sits in **maintenance
mode**: the Talos API answers on port 50000 but serves only a self-signed certificate, because no
cluster CA exists yet. Calls must therefore be made with `insecure: true`.

Only three tools accept `insecure`: **`talos_version`**, **`talos_apply_config`** and
**`talos_disks`**. `talos_get` does **not** — a maintenance-mode node cannot be inspected through
it, so use `talos_disks` to find the install disk and `talos_version` to confirm reachability.

Stop passing `insecure` the moment the machine config is applied; from then on the node presents a
cluster-signed certificate and an insecure call is both wrong and less safe.

## Procedure

1. `talosctl gen secrets -o secrets.yaml` (Bash). Store this file safely — it cannot be regenerated, and losing it means losing the ability to recover the cluster.
2. `talosctl gen config <cluster-name> https://<endpoint>:6443 --with-secrets secrets.yaml` (Bash), producing `controlplane.yaml`, `worker.yaml` and `talosconfig`.
3. Load the talosconfig: base64-encode it and pass the result to `talos_set_config`.
4. Confirm each target is reachable and in maintenance mode: `talos_version(node=..., insecure=true)`, and `talos_disks(node=..., insecure=true)` to pick the install disk.
5. Apply the control-plane config to every control-plane node: `talos_apply_config(config, node=..., insecure=true)`. Each node installs to disk and reboots into the configured system.
6. Apply the worker config to every worker the same way.
7. `talos_bootstrap(node=...)` on **exactly one** control-plane node.
8. `talos_kubeconfig` once etcd is up.
9. `talos_health` to verify.

## Bootstrap exactly one node

`talos_bootstrap` initialises a new etcd cluster. Running it on a second node creates a **second,
separate** etcd cluster rather than joining the first, and the result is a split cluster that has
to be reset and redone. The other control-plane nodes join the initialised etcd automatically once
they see it. Bootstrap is a one-time operation for the life of the cluster: it is not part of
recovery, except through `talosctl bootstrap --recover-from` (see
`references/operations/etcd-and-recovery.md`).

## Verifying

Use `talos_health`, then list nodes through the Kubernetes MCP
(`mcp__kubernetes-mcp-server__resources_list` for `v1/Node`) to confirm each node registered and
went Ready.

**Do not verify with `nodes_top`.** It reads the metrics API, which needs metrics-server, and a
freshly bootstrapped cluster has no metrics-server installed. The resulting error says nothing
about cluster health and reads as a failure that is not one.

Nodes stuck out of Ready immediately after bootstrap are usually waiting on a CNI: Talos installs
Flannel by default, but a config generated with `--with-cni=none` leaves the cluster deliberately
Ready-less until a CNI is applied.
