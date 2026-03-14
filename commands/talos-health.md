---
name: talos-health
description: Check the health status of a Talos Linux cluster
allowed-tools: ["Read", "Bash", "Grep", "mcp__talos__*", "mcp__kubernetes-mcp-server__*"]
---

Perform a comprehensive health check on the Talos cluster:

1. **Cluster health** — Run `mcp__talos__talos_health` to check overall cluster status.

2. **Node versions** — Run `mcp__talos__talos_version` on each node to verify consistent versions.

3. **Services** — Run `mcp__talos__talos_services` to check all Talos services are running.

4. **etcd** — Run `mcp__talos__talos_etcd_members` and `mcp__talos__talos_etcd_status` to verify etcd cluster health.

5. **System resources** — Check `mcp__talos__talos_memory` and `mcp__talos__talos_cpu` for resource pressure.

6. **Disk usage** — Check `mcp__talos__talos_disks` and `mcp__talos__talos_mounts` for storage issues.

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
