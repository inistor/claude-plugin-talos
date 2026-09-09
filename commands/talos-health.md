---
name: talos-health
description: Check the health status of a Talos Linux cluster
allowed-tools: ["Read", "Bash", "Grep", "mcp__talos__*", "mcp__plugin_talos_talos__*", "mcp__kubernetes-mcp-server__*"]
---

Perform a comprehensive health check on the Talos cluster:

1. **Diagnostics first** — Run `talos_get` with `resource_type: diagnostics`. Talos surfaces its own known-problem warnings here, and it is the cheapest thing to check.

2. **Cluster health** — Run `talos_health` to check overall cluster status.

3. **Node versions** — Run `talos_version` on each node to verify consistent versions.

4. **Services** — Run `talos_services` to check all Talos services are running.

5. **etcd** — Run `talos_etcd_members` and `talos_etcd_status` to verify etcd cluster health.

6. **System resources** — Check `talos_memory` and `talos_cpu` for resource pressure.

7. **Storage** — Use the COSI-resource MCP tools (upstream-preferred over the legacy `talos_disks`/`talos_mounts` wrappers):
   - `talos_discovered_volumes` — block devices and partitions
   - `talos_get` with `resource_type: systemdisk` — identify the system disk
   - `talos_get` with `resource_type: mountstatus` — current mount points
   - `talos_disk_usage` — filesystem usage on key paths (/, /var, /system/state)

8. **Kubernetes** — Use Kubernetes MCP tools:
   - `mcp__kubernetes-mcp-server__resources_list` (apiVersion `v1`, kind `Node`) — node readiness
   - `mcp__kubernetes-mcp-server__nodes_top` — node resource usage (needs metrics-server; skip if absent)
   - `mcp__kubernetes-mcp-server__pods_list` — Check for unhealthy pods
   - `mcp__kubernetes-mcp-server__events_list` — Recent warning events

9. **Report** — Present a summary:
   - Overall status (healthy/degraded/unhealthy)
   - Per-node status table
   - Any issues found with recommended actions
   - Resource utilization overview

**Important:**
- Use `yq` or `jq` for parsing output, not grep
- If issues are found, suggest specific remediation steps
- Reference the Talos skill's troubleshooting guide for known issues
- On a v1.14 cluster, also check that Prometheus scrapes etcd on port **2383**, not 2379 — the metrics endpoint moved and the old target fails silently
