---
name: talos-health
description: Check the health status of a Talos Linux cluster
allowed-tools: ["Read", "Bash", "Grep", "mcp__talos__*", "mcp__plugin_talos_talos__*", "mcp__kubernetes-mcp-server__*"]
---

Perform a comprehensive health check on the Talos cluster:

1. **Cluster health** — Run `talos_health` to check overall cluster status.

2. **Node versions** — Run `talos_version` on each node to verify consistent versions.

3. **Services** — Run `talos_services` to check all Talos services are running.

4. **etcd** — Run `talos_etcd_members` and `talos_etcd_status` to verify etcd cluster health.

5. **System resources** — Check `talos_memory` and `talos_cpu` for resource pressure.

6. **Storage** — Use the COSI-resource MCP tools (upstream-preferred over the legacy `talos_disks`/`talos_mounts` wrappers):
   - `talos_discovered_volumes` — block devices and partitions
   - `talos_get` with `resource_type: systemdisk` — identify the system disk
   - `talos_get` with `resource_type: mountstatus` — current mount points
   - `talos_disk_usage` — filesystem usage on key paths (/, /var, /system/state)

7. **Kubernetes** — Use Kubernetes MCP tools:
   - `mcp__kubernetes-mcp-server__nodes_top` — Node resource usage
   - `mcp__kubernetes-mcp-server__pods_list` — Check for unhealthy pods
   - `mcp__kubernetes-mcp-server__events_list` — Recent warning events

8. **Report** — Present a summary:
   - Overall status (healthy/degraded/unhealthy)
   - Per-node status table
   - Any issues found with recommended actions
   - Resource utilization overview

**Important:**
- Use `yq` or `jq` for parsing output, not grep
- If issues are found, suggest specific remediation steps
- Reference the Talos skill's troubleshooting guide for known issues
