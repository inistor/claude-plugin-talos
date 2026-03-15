# Troubleshooting Reference

Docs: https://docs.siderolabs.com/talos/v1.12/advanced/troubleshooting/

## Diagnostic Commands (MCP tools)

| Problem Area | Tool | What to Check |
|---|---|---|
| Cluster health | `talos_health` | Overall cluster status |
| Service state | `talos_services` | All services running |
| Service logs | `talos_logs(service)` | kubelet, etcd, apid, machined |
| Kernel logs | `talos_dmesg` | Hardware, driver issues |
| etcd | `talos_etcd_members`, `talos_etcd_status` | Member count, leader |
| Disk space | `talos_disks`, `talos_mounts` | Full disks |
| Memory | `talos_memory` | OOM pressure |
| Network | `talos_netstat` | Connectivity |
| Containers | `talos_containers` | Stuck containers |
| Processes | `talos_processes` | Runaway processes |
| Resources | `talos_get` | Any Talos resource |

## Common Issues

### Node Not Joining Cluster
1. Check discovery: `talos_members`
2. Verify endpoints in config match actual CP endpoint
3. Check network connectivity between nodes
4. Verify machine token matches cluster token
5. Check `talos_logs(service="machined")` for join errors

### etcd Unhealthy
1. `talos_etcd_members` — check member count (should be odd: 1, 3, 5)
2. `talos_etcd_status` — check leader election, DB size
3. `talos_logs(service="etcd")` — look for election timeouts, disk latency
4. If DB too large: `talos_etcd_defrag` (one node at a time)
5. If member lost: remove stale member, reset and rejoin

### Kubelet Not Starting
1. `talos_services` — check kubelet state
2. `talos_logs(service="kubelet")` — check for errors
3. Common causes: invalid kubelet config, certificate issues, missing CNI
4. Check K8s API reachability from node

### Pod Stuck in Pending/CrashLoop
1. Use Kubernetes MCP: `events_list` for the namespace
2. Check node resources: `nodes_top`
3. Check pod logs: `pods_log`
4. Common causes: resource limits, image pull failures, PV issues

### Upgrade Failures
1. Check upgrade status: `talos_services` on the upgraded node
2. If node doesn't come back: it may have rolled back to previous version
3. `talos_dmesg` — check for boot errors
4. `talos_version` — verify version on the node
5. If etcd member missing after upgrade: check `talos_etcd_members`

### Network Issues
1. `talos_addresses` — check assigned IPs
2. `talos_routes` — verify routing table
3. `talos_interfaces` — check interface status
4. `talos_netstat` — verify listening ports
5. `talos_dmesg` — check for NIC driver issues

### Disk Full
1. `talos_mounts` — check available space
2. Common culprits: etcd DB, container images, logs
3. Defrag etcd if DB is large
4. Pull through image cache may fill `/var`

### Certificate Issues
1. `talos_get(resource_type="certificates")` — check cert status (use generic get for this resource)
2. Certificates auto-rotate, but CA must be rotated manually
3. If expired: apply new config with fresh CA

## Resource Types for `talos_get`

Common resource types:
- `members` — cluster members
- `services` — service status
- `addresses` — IP addresses
- `routes` — routing table
- `links` — network interfaces
- `extensions` — installed extensions
- `discoveredmembers` — discovered nodes
- `mc` — machine config (alias for machineconfig)
- `cpustat` — CPU statistics
- `memorymodules` — memory info
- `systemdisks` — system disk info
