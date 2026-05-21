---
name: talos-upgrade
description: Upgrade Talos Linux and/or Kubernetes on a cluster
allowed-tools: ["Read", "Bash", "Grep", "mcp__talos__*", "mcp__plugin_talos_talos__*", "mcp__kubernetes-mcp-server__*"]
argument-hint: "[talos-version|k8s-version]"
---

Upgrade Talos Linux or Kubernetes on the cluster. Determine what to upgrade from the argument:

- If version starts with `v1.` — Talos upgrade
- If version starts with `v1.3` or similar K8s pattern — Kubernetes upgrade
- If no argument, ask the user what to upgrade

## Talos Upgrade

1. **Pre-flight checks:**
   - Get current version on all nodes: `talos_version`
   - Check cluster health: `talos_health`
   - List installed extensions on each node: `talos_extensions`
   - Create etcd snapshot: `talos_etcd_snapshot`

2. **Decide on the image:**
   - **No extensions** — use the stock image, e.g. `ghcr.io/siderolabs/installer:v1.13.2`.
   - **Extensions installed** — stock images **do not contain extensions**; upgrading to one will remove them on reboot. Build a custom installer first via `/talos-image` (output type `installer`), push it to a registry, and use that image reference here. Confirm with the user before proceeding if extensions are present and only a stock image was provided.

3. **Upgrade control plane nodes** (one at a time):
   a. Call `talos_upgrade(node, image)`.
   b. Inspect the response's `"api"` field:
      - **`"api": "legacy"`** (server <v1.13) — the node auto-reboots as part of the upgrade. Skip to (d) and wait for it to come back.
      - **`"api": "lifecycle"`** (server v1.13+) — install-only; the node is *not* rebooted yet. Continue to (c).
   c. *(v1.13+ only)* Reboot to activate. For workload safety, drain via the Kubernetes MCP first:
      - `mcp__kubernetes-mcp-server__nodes_cordon` to mark unschedulable
      - Evict / delete the pods on the node (Kubernetes MCP eviction tooling, or `kubectl drain` via Bash if necessary)
      - Then `talos_reboot(node)` — this is the bare Talos reboot, **does not drain on its own**, so the drain step is on you
      - After the node comes back, uncordon via the Kubernetes MCP
   d. Verify health (`talos_health`, `talos_etcd_members`) before proceeding to the next CP node.
   - **Quorum caution:** for 3-node CP this tolerates one node down; for 2-node CP you have zero margin — the cluster will lose etcd quorum while a CP is upgrading. Warn the user explicitly on a 2-CP cluster.

4. **Upgrade worker nodes:**
   - Use `talos_upgrade` on each worker (same legacy-vs-lifecycle dispatch + optional drain pattern as step 3 for v1.13+).
   - Workers can be upgraded in parallel only if the user confirms and workloads tolerate simultaneous reboots.

5. **Post-upgrade verification:**
   - Check cluster health: `talos_health`
   - Verify all nodes report new version: `talos_version`
   - Verify extensions are still present: `talos_extensions` (catches the "upgraded to stock image" mistake)
   - Check Kubernetes workloads via `mcp__kubernetes-mcp-server__pods_list`

## Kubernetes Upgrade

Kubernetes upgrades use `talosctl upgrade-k8s` — a complex client-side orchestration that patches configs, pre-pulls images, and monitors rollout across all nodes. As of Talos v1.13 this stays client-side (the new `LifecycleService` API covers Talos OS upgrades only).

0. **Precondition:** verify `talosctl` is installed and on PATH. Run `command -v talosctl >/dev/null || { echo "talosctl required for k8s upgrade (no MCP equivalent); install it first"; exit 1; }`. If missing, tell the user to install it (`brew install siderolabs/tap/talosctl` or download from https://github.com/siderolabs/talos/releases) before proceeding — there is no MCP equivalent for this path.
1. **Pre-flight:** Create etcd snapshot via `talos_etcd_snapshot`
2. **Dry-run:** `talosctl upgrade-k8s --to <version> --dry-run` via Bash and review the plan with the user
3. **Run:** `talosctl upgrade-k8s --to <version>` via Bash
4. **Verify:** Check node versions via `mcp__kubernetes-mcp-server__nodes_list`, check cluster health via `talos_health`

**Important:**
- Always create an etcd snapshot before starting
- Do NOT attempt to replicate the K8s upgrade manually with `talos_patch` — use `talosctl upgrade-k8s`
- Always run `--dry-run` first to preview the upgrade plan
