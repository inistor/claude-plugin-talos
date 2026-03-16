---
name: talos-upgrade
description: Upgrade Talos Linux and/or Kubernetes on a cluster
allowed-tools: ["Read", "Bash", "Grep", "mcp__talos__*", "mcp__kubernetes-mcp-server__*"]
argument-hint: "[talos-version|k8s-version]"
---

Upgrade Talos Linux or Kubernetes on the cluster. Determine what to upgrade from the argument:

- If version starts with `v1.` — Talos upgrade
- If version starts with `v1.3` or similar K8s pattern — Kubernetes upgrade
- If no argument, ask the user what to upgrade

## Talos Upgrade

1. **Pre-flight checks:**
   - Get current version on all nodes: `mcp__talos__talos_version`
   - Check cluster health: `mcp__talos__talos_health`
   - Create etcd snapshot: `mcp__talos__talos_etcd_snapshot`

2. **Upgrade control plane nodes** (one at a time):
   - Use `mcp__talos__talos_upgrade` with the target image
   - Wait for the node to come back and rejoin the cluster
   - Verify health before proceeding to the next node

3. **Upgrade worker nodes:**
   - Use `mcp__talos__talos_upgrade` on each worker
   - Can upgrade workers in parallel if user confirms

4. **Post-upgrade verification:**
   - Check cluster health: `mcp__talos__talos_health`
   - Verify all nodes report new version: `mcp__talos__talos_version`
   - Check Kubernetes workloads via `mcp__kubernetes-mcp-server__pods_list`

## Kubernetes Upgrade

Kubernetes upgrades use `talosctl upgrade-k8s` — a complex client-side orchestration that patches configs, pre-pulls images, and monitors rollout across all nodes.

1. **Pre-flight:** Create etcd snapshot via `mcp__talos__talos_etcd_snapshot`
2. **Run:** `talosctl upgrade-k8s --to <version>` via Bash
3. **Verify:** Check node versions via Bash (`kubectl get nodes`), check cluster health via `mcp__talos__talos_health`

**Important:**
- Always create an etcd snapshot before starting
- Do NOT attempt to replicate the K8s upgrade manually with `talos_patch` — use `talosctl upgrade-k8s`
- Add `--dry-run` first to preview the upgrade plan
