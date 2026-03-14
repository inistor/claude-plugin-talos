---
name: talos-bootstrap
description: Bootstrap a new Talos Linux cluster from scratch
allowed-tools: ["Read", "Write", "Bash", "Grep", "Glob", "mcp__talos__*", "mcp__kubernetes-mcp-server__*"]
argument-hint: "[cluster-name] [endpoint]"
---

Bootstrap a new Talos Linux cluster. Follow these steps:

1. **Gather information** — Ask the user for:
   - Cluster name (use argument if provided)
   - Control plane endpoint (VIP or load balancer address)
   - Node IPs and roles (control plane vs worker)
   - Any config patches (extensions, network config, etc.)

2. **Generate configuration** — Use `mcp__talos__talos_gen_config` with the cluster name and endpoint. Apply any user-specified patches.

3. **Apply configs** — For each node:
   - Apply the appropriate config (controlplane or worker) using `mcp__talos__talos_apply_config`
   - Target the specific node using the `node` parameter

4. **Bootstrap etcd** — Use `mcp__talos__talos_bootstrap` on ONE control plane node only.

5. **Verify** — Use `mcp__talos__talos_health` to check cluster health. Then use `mcp__kubernetes-mcp-server__nodes_top` to verify Kubernetes is running.

6. **Get kubeconfig** — Inform the user how to retrieve the kubeconfig: `talosctl kubeconfig`

**Important:**
- Never bootstrap more than one node
- Wait for each step to complete before proceeding
- Use `yq` or `jq` for parsing any YAML/JSON output, not grep
- Report progress at each step
