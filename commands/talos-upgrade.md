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

1. Upgrade is done via machine config patch — update the `cluster.apiServer.image`, `cluster.controllerManager.image`, `cluster.scheduler.image` fields
2. Apply config to control plane nodes using `mcp__talos__talos_apply_config`
3. Monitor Kubernetes component rollout via Kubernetes MCP

**Important:**
- Always upgrade control plane nodes before workers
- Always create an etcd snapshot before starting
- Never upgrade all control plane nodes simultaneously
- Use `yq` for YAML manipulation, not sed/grep
