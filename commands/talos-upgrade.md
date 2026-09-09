---
name: talos-upgrade
description: Upgrade Talos Linux and/or Kubernetes on a cluster
allowed-tools: ["Read", "Bash", "Grep", "mcp__talos__*", "mcp__plugin_talos_talos__*", "mcp__kubernetes-mcp-server__*"]
argument-hint: "[talos-version|k8s-version]"
---

Upgrade Talos Linux or Kubernetes on the cluster. Determine what to upgrade from the argument:

- A Talos version looks like `v1.14.0` (Talos 1.x minors are currently ≤ 14)
- A Kubernetes version looks like `v1.37.x` (Kubernetes minors are currently in the 30s)
- If ambiguous or absent, ask the user

## Talos Upgrade

1. **Pre-flight checks:**
   - Current version on all nodes: `talos_version`
   - Cluster health: `talos_health`
   - Installed extensions on each node: `talos_extensions`
   - etcd snapshot: `talos_etcd_snapshot`

2. **Check the upgrade path.** Talos 1.12 and 1.13 can go directly to 1.14; 1.11 and older cannot and must hop through intermediate minors. Upgrading through the latest patch of each minor is the conservative choice.

3. **Decide on the image.**

   As of **v1.14, `ghcr.io/siderolabs/installer` is no longer published** — that tag 404s. Installer images come from the Image Factory:

   ```
   factory.talos.dev/metal-installer/<schematic-id>:v1.14.0
   ```

   - **No extensions** — use the empty/default schematic `376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba`.
   - **Extensions installed** — a stock image contains none, and upgrading to one removes them on reboot. Either use the Image Factory schematic that matches the node's extensions, or build a custom installer via `/talos-image` (output type `installer`) and push it. Confirm with the user before proceeding if extensions are present and only a stock image was given.
   - **Upgrading to v1.13 or earlier** — `ghcr.io/siderolabs/installer:v1.13.x` still exists and is still correct for those targets.

   The schematic a node was installed from is recorded in the `ImageFactorySchematics` resource: `talos_get(resource_type="imagefactoryschematics", node=...)`.

4. **Upgrade control plane nodes, one at a time:**
   - Call `talos_upgrade(node, image)`. It runs the full cycle — cordon → drain → install → reboot → wait for Talos and Kubernetes → uncordon — and returns when the node is back in service.
   - Read the response. `status: "ok"` means success; anything else is a failure whose `stages` and `*_error` fields say where it stopped. See the skill's `references/upgrade.md` for the status table.
   - Verify with `talos_health` and `talos_etcd_members` before moving to the next node.
   - **Deferred activation:** `auto_reboot=false` installs without rebooting (drain, wait and uncordon are skipped too). Trigger `talos_reboot(node)` in the maintenance window.
   - **If Kubernetes is unreachable** the upgrade aborts rather than rebooting an undrained node. Only pass `skip_drain=true` when that is genuinely intended.
   - **Quorum caution:** a 3-node control plane tolerates one node down; a 2-node control plane has no margin and loses etcd quorum whenever a member is upgrading. Warn the user explicitly on a 2-CP cluster.

5. **Upgrade worker nodes** — same single call per node. Parallel only if the user confirms the workloads tolerate simultaneous reboots.

6. **Post-upgrade verification:**
   - `talos_health`
   - `talos_version` — all nodes report the new version
   - `talos_extensions` — catches the "upgraded to a stock image, lost the extensions" mistake
   - `mcp__kubernetes-mcp-server__pods_list` — workloads are running

7. **v1.14-specific follow-ups.** Mention these; they surprise people:
   - **etcd metrics and health moved from port 2379 to 2383.** Prometheus scrape configs and firewall rules pointing at 2379 break silently.
   - **Workload isolation and filesystem trim are on for new clusters but off for upgraded ones** — the documents are simply absent. Adding `SecurityProfileConfig` with `workloadIsolation` enables it, but note the in-tree Kubernetes `iscsi` volume plugin stops working under it.
   - **`multipath-tools` users must apply a config migration *before* upgrading**, or `multipathd` hangs forever. See the skill's troubleshooting reference.

## Kubernetes Upgrade

`talosctl upgrade-k8s` is a client-side orchestration that patches configs, pre-pulls images and
monitors rollout across all nodes. This is still client-side in v1.14 — LifecycleService covers the
Talos OS install only — so it is a correct use of Bash, not a fallback.

0. **Precondition:** verify `talosctl` is present and matches the cluster's Talos minor.
   ```bash
   command -v talosctl >/dev/null || { echo "talosctl required for the k8s upgrade (no MCP equivalent); install it first"; exit 1; }
   talosctl version --client --short
   ```
   If missing, point the user at `brew install siderolabs/tap/talosctl` or https://github.com/siderolabs/talos/releases.
1. **Check the target is supported.** Talos 1.14 supports Kubernetes 1.32–1.37 (default 1.37.0); Talos 1.13 supports 1.31–1.36 (default 1.36.0).
2. **Pre-flight:** `talos_etcd_snapshot`
3. **Dry run:** `talosctl upgrade-k8s --to <version> --dry-run` and review the plan with the user
4. **Run:** `talosctl upgrade-k8s --to <version>`
5. **Verify:** node versions via `mcp__kubernetes-mcp-server__resources_list` (apiVersion `v1`, kind `Node`), then `talos_health`

**Important:**
- Always snapshot etcd before starting
- Never reproduce the Kubernetes upgrade manually with `talos_patch`
- Always `--dry-run` first
- The command is resumable if interrupted
