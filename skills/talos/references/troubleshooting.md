# Troubleshooting Reference

Targets Talos **v1.14.0** (default Kubernetes 1.37.0).

Docs:
- Troubleshooting — https://docs.siderolabs.com/talos/v1.14/troubleshooting/troubleshooting
- Support bundle — https://docs.siderolabs.com/talos/v1.14/troubleshooting/support-bundle
- `talosctl debug` — https://docs.siderolabs.com/talos/v1.14/troubleshooting/talosctl-debug
- FAQs — https://docs.siderolabs.com/talos/v1.14/troubleshooting/faqs

Use the MCP tools first; SKILL.md lists the few operations that legitimately shell out to
`talosctl`. `talosctl debug` and `talosctl support` are the two that matter here.

## Diagnostic Commands (MCP tools)

| Problem Area | Tool | What to Check |
|---|---|---|
| Known problems | `talos_get(resource_type="diagnostics")` | Talos' own warnings — check this **first** |
| Cluster health | `talos_health` | Overall cluster status |
| Service state | `talos_services` | All services running |
| Service logs | `talos_logs(service)` | kubelet, etcd, apid, machined, sandboxd |
| Kernel logs | `talos_dmesg` | Hardware, driver issues |
| etcd | `talos_etcd_members`, `talos_etcd_status` | Member count, leader |
| Disk space | `talos_discovered_volumes`, `talos_get(mountstatus)`, `talos_disk_usage` | Full disks, mount state (upstream-preferred over the legacy `talos_disks`/`talos_mounts` wrappers) |
| Memory | `talos_memory` | OOM pressure |
| Network | `talos_netstat` | Connectivity |
| Containers | `talos_containers` | Stuck containers |
| Cached images | `talos_image_list`, `talos_image_remove`, `talos_image_prune` | Image cache growth |
| Processes | `talos_processes` | Runaway processes |
| Extensions | `talos_extensions` | Which extensions actually loaded |
| Resources | `talos_get` | Any Talos resource |

## Start Here: the Diagnostics Resource

Talos runs its own diagnostic controllers and publishes findings as first-class COSI resources
(`Diagnostics.runtime.talos.dev`). Each entry is a known-problem warning with a message and a docs
link — address-range overlaps, kubelet CSRs not approved, and similar.

```
talos_get(resource_type="diagnostics", node="<node>")
```

An empty result means Talos itself sees no known problems; it does not mean the node is healthy, so
continue with the checks below. Diagnostics also surface in `talosctl dashboard` and in the support
bundle.

## Common Issues

### Node Not Joining Cluster
1. Check discovery: `talos_members` (registered members) and `talos_get(resource_type="affiliates")`
   (raw discovered peers, before merging)
2. Verify endpoints in config match the actual control plane endpoint
3. Check network connectivity between nodes
4. Verify machine token matches cluster token
5. `talos_logs(service="machined")` for join errors

### etcd Unhealthy
1. `talos_etcd_members` — check member count (should be odd: 1, 3, 5)
2. `talos_etcd_status` — check leader election, DB size
3. `talos_logs(service="etcd")` — look for election timeouts, disk latency
4. If DB too large: `talos_etcd_defrag` (one node at a time)
5. If member lost: `talos_etcd_remove_member`, then reset and rejoin the node

### Kubelet Not Starting
1. `talos_services` — check kubelet state
2. `talos_logs(service="kubelet")` — check for errors
3. Common causes: invalid kubelet config, certificate issues, missing CNI
4. Check K8s API reachability from the node (`talos_netstat`, KubePrism on 7445)
5. On 1.14 with workload isolation on, also check `talos_logs(service="sandboxd")` — the kubelet lives
   inside that namespace and will not start if `sandboxd` is unhealthy

### Pod Stuck in Pending/CrashLoop
1. Kubernetes MCP: `mcp__kubernetes-mcp-server__events_list` for the namespace
2. Node resources: `mcp__kubernetes-mcp-server__nodes_top`
3. Pod logs: `mcp__kubernetes-mcp-server__pods_log`
4. Common causes: resource limits, image pull failures, PV issues

### Upgrade Failures
1. Check upgrade status: `talos_services` on the upgraded node
2. If the node doesn't come back: it may have rolled back to the previous version
3. `talos_dmesg` — check for boot errors
4. `talos_version` — verify the version on the node
5. If an etcd member is missing after upgrade: `talos_etcd_members`
6. 1.14 specifically: an upgrade that pulls `ghcr.io/siderolabs/installer:v1.14.0` fails — that image is
   no longer published. Use `factory.talos.dev/metal-installer/<schematic-id>:v1.14.0` (see
   `boot-assets.md`).

### Network Issues
1. `talos_addresses` — check assigned IPs
2. `talos_routes` — verify routing table
3. `talos_interfaces` — check interface status
4. `talos_netstat` — verify listening ports
5. `talos_dmesg` — check for NIC driver issues
6. If BGP is configured (new in 1.14): `talos_get(resource_type="bgppeerstatuses")`

### Disk Full
1. `talos_disk_usage` for filesystem usage; `talos_get(mountstatus)` for current mounts;
   `talos_discovered_volumes` for block devices
2. Common culprits: etcd DB, container images, logs
3. Defrag etcd if the DB is large
4. Pull-through image cache may fill `/var`
5. Reclaim image cache with the MCP: `talos_image_list` to inspect, `talos_image_prune`
   (`dry_run=true` by default — re-run with `dry_run=false` to actually delete), `talos_image_remove`
   for a specific ref

### Certificate Issues
There is **no `certificates` resource type** — `talos_get(resource_type="certificates")` fails to
resolve. Use the real types:

| Resource | What it holds |
|---|---|
| `ApiCertificates.secrets.talos.dev` | apid server certificate |
| `TrustdCertificates.secrets.talos.dev` | trustd server certificate |
| `KubernetesDynamicCerts.secrets.talos.dev` | dynamically issued Kubernetes certs |
| `KubernetesRootSecrets.secrets.talos.dev` / `OSRootSecrets.secrets.talos.dev` | CA material (root secrets) |

```
talos_get(resource_type="apicertificates", node="<node>")
talos_get(resource_type="trustdcertificates", node="<node>")
```

Leaf certificates auto-rotate. CAs must be rotated deliberately with `talosctl rotate-ca` via Bash
(no MCP equivalent); see https://docs.siderolabs.com/talos/v1.14/security/ca-rotation.

### Storage / RAID / LVM (1.14)
- `talos_get(resource_type="mdarraystatuses")` — software RAID array state (`storage` namespace)
- `talos_get(resource_type="lvmvolumegroupstatuses")`, `…="lvmlogicalvolumestatuses"` — LVM state
- `talos_get(resource_type="fsscrubstatuses")` — filesystem scrub results (`block` namespace)
- `talos_volumes` / `talos_discovered_volumes` for the volume and block-device view

### Kernel Modules
`talos_get(resource_type="kernelmodulestatuses")` reports both dynamically loaded and built-in modules.
This supersedes `LoadedKernelModules`, which is deprecated in 1.14 and should not be used in new
diagnostics.

## Talos 1.14 gotchas — symptom first

Mechanism and remedies for all three are in `references/v1.14-changes.md`; this section is the
symptom-to-cause mapping.

| Symptom | Cause |
|---|---|
| iSCSI volumes that mounted before an upgrade now fail to attach; kubelet cannot find `iscsid` | Workload isolation (`sandboxd`) puts kubelet in its own PID namespace, so the in-tree `iscsi` volume plugin cannot reach host `iscsid`. Migrate to a CSI driver. |
| A burst of pod restarts with **no** reboot in `talos_dmesg` | `sandboxd` died; the kernel tore down the namespace and Talos recreated it, relaunching CRI, kubelet and pods without rebooting. Check `talos_logs(service="sandboxd")`. |
| Prometheus etcd targets go down after upgrading, but the cluster is healthy | etcd HTTP endpoints moved to port 2383; 2379 is gRPC-only. gRPC clients and `talos_health` are unaffected, which is why only monitoring notices. |
| `multipathd` stuck in `Waiting`/`Preparing` in `talos_services`, multipath devices never appear | The extension now reads `/etc/multipath.conf` from the host and ships no default, so it waits forever. Needs an `EtcFileConfig` — apply it **before** upgrading. |

Workload isolation is **on for new 1.14 clusters and off for upgraded ones**, so a cluster can be on
1.14 and still show pre-1.14 behaviour. Confirm with
`talos_get(resource_type="securityprofileconfig")` rather than inferring it from the Talos version.

## Support Bundle

The standard "collect everything" escalation artifact. No MCP equivalent — use Bash:

```bash
talosctl -n <node1>,<node2> support -O support.zip.age
```

It collects, per node: kernel logs, all Talos service logs, kube-system pod logs, COSI resources
(secrets excluded), the COSI runtime state graph, process/IO-pressure snapshots, mounts, PCI devices and
version; plus cluster-level Kubernetes node and kube-system pod manifests.

**1.14 change: the bundle is age-encrypted by default**, to a default recipient set of `siderolabs`
GitHub organization members — meaning the bundle is unreadable locally unless recipients are set explicitly:

```bash
# readable locally, old behaviour
talosctl -n <node> support --no-encryption -O support.zip

# keep Sidero Labs recipients and add your own key
talosctl -n <node> support --encryption-recipients age1... -O support.zip.age

# encrypt only to local keys
talosctl -n <node> support --encryption-recipients age1... --encryption-no-default-recipients
```

`--no-encryption` cannot be combined with `--encryption-recipients` /
`--encryption-no-default-recipients`. When no `-O` is given the output name gets an `.age` suffix
automatically unless `--no-encryption` is set.

## Interactive Debugging

`talosctl debug` runs and attaches to a privileged debug container with a user-provided image. No MCP
equivalent — use Bash:

```bash
talosctl -n <node> debug --image <debug-image> -- <command>
```

Use it sparingly: the debug container has elevated privileges. For routine diagnostics prefer the MCP
tools above. Note `talosctl debug` does not support the `taloscontainers` namespace.

## Resource Types for `talos_get`

Names are resolved case-insensitively against the full type (`Affiliates.cluster.talos.dev`), so the
short forms below work.

Cluster / discovery:
- `members` — merged cluster members
- `affiliates` — discovered peers (`Affiliates.cluster.talos.dev`); there is **no `discoveredmembers`**
- `identities` — node identity

Runtime:
- `diagnostics` — known-problem warnings (`Diagnostics.runtime.talos.dev`)
- `services` — service status
- `extensions` — installed extensions
- `mc` — machine config (alias for machineconfig)
- `kernelmodulestatuses` — loaded + built-in modules (supersedes `loadedkernelmodules`)
- `unattendedinstallstatuses` — unattended install progress (new in 1.14)
- `mountstatus` — current mounts
- `machinestatus` — machine stage / readiness

Network:
- `addresses`, `routes`, `links`, `resolvers`, `hostname`
- `bgppeerstatuses` — BGP session state (new in 1.14)

Storage / block:
- `discoveredvolumes` — block devices and partitions
- `volumestatuses` — Talos-managed volumes
- `mdarraystatuses` — software RAID (new in 1.14)
- `lvmvolumegroupstatuses`, `lvmlogicalvolumestatuses` — LVM (new in 1.14)
- `fsscrubstatuses` — filesystem scrub results (new in 1.14)
- `systemdisks` — system disk info

Secrets:
- `apicertificates`, `trustdcertificates`, `kubernetesdynamiccerts`

Hardware:
- `cpustat` — CPU statistics
- `memorymodules` — memory info

Full list on a live node: `talosctl -n <node> get rd` (resource definitions).
