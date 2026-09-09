package main

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTools(s *server.MCPServer) {
	// Common parameters used across tools
	// node: optional, target node IP/hostname
	// context: optional, talosconfig context name

	// --- Configuration management ---

	s.AddTool(mcp.NewTool("talos_set_config",
		mcp.WithDescription("Set the talosconfig for this session. All subsequent tools will use this config instead of ~/.talos/config. Pass the file content base64-encoded to preserve formatting. Use: base64 < talosconfig via Bash, then pass the output."),
		mcp.WithString("content", mcp.Required(), mcp.Description("Talosconfig content, base64-encoded (preferred) or raw YAML")),
		mutating(),
	), handleSetConfig)

	s.AddTool(mcp.NewTool("talos_config_info",
		mcp.WithDescription("Show current talosconfig content (contexts, endpoints, nodes)."),
		readOnly(),
	), handleConfigInfo)

	s.AddTool(mcp.NewTool("talos_get",
		mcp.WithDescription("Get Talos resources by type. Supports aliases (e.g. 'mc', 'addresses', 'volumes', 'members', 'extensions', 'links', 'routes'). Like 'talosctl get <type> [id]'."),
		mcp.WithString("resource_type", mcp.Required(), mcp.Description("Resource type or alias: addresses, routes, links, members, mc, volumes, extensions, discoveredvolumes, cpustat, etc.")),
		mcp.WithString("resource_id", mcp.Description("Optional resource ID to get a specific resource")),
		mcp.WithString("namespace", mcp.Description("Resource namespace (auto-detected if omitted)")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleGet)

	// --- Cluster operations ---

	s.AddTool(mcp.NewTool("talos_bootstrap",
		mcp.WithDescription("Bootstrap etcd on a control plane node. Only run on ONE node per cluster. For etcd recovery, use talosctl bootstrap --recover-from via Bash."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleBootstrap)

	s.AddTool(mcp.NewTool("talos_health",
		mcp.WithDescription("Check cluster health: etcd, API server, kubelet, connectivity. Note: 'finish boot sequence' check on control plane nodes may timeout — this is normal for long-running CPs."),
		mcp.WithNumber("wait_timeout", mcp.Description("Timeout in seconds to wait for cluster to be ready (default: 300)")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleHealth)

	s.AddTool(mcp.NewTool("talos_version",
		mcp.WithDescription("Get Talos and Kubernetes version info from a node."),
		mcp.WithBoolean("short", mcp.Description("Print short version string only")),
		mcp.WithBoolean("insecure", mcp.Description("Use insecure mode for maintenance/bootstrap (no TLS auth)")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleVersion)

	// --- Node operations ---

	s.AddTool(mcp.NewTool("talos_apply_config",
		mcp.WithDescription("Apply FULL machine configuration to a node. Requires the complete config YAML (not a patch). For partial changes, use talos_patch instead."),
		mcp.WithString("config", mcp.Required(), mcp.Description("Complete machine configuration YAML")),
		mcp.WithString("mode", mcp.Description("Apply mode (default: auto). 'reboot' is deprecated upstream — prefer auto or no-reboot."), mcp.Enum("auto", "no-reboot", "reboot", "staged", "try")),
		mcp.WithBoolean("dry_run", mcp.Description("Check how the config change will be applied without actually applying")),
		mcp.WithBoolean("insecure", mcp.Description("Use insecure mode for maintenance/bootstrap (no TLS auth)")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleApplyConfig)

	s.AddTool(mcp.NewTool("talos_reboot",
		mcp.WithDescription("Reboot a Talos node."),
		mcp.WithString("mode", mcp.Description("Reboot mode (default: default)"), mcp.Enum("default", "powercycle", "force")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleReboot)

	s.AddTool(mcp.NewTool("talos_shutdown",
		mcp.WithDescription("Shutdown a Talos node."),
		mcp.WithBoolean("force", mcp.Description("Force shutdown without cordon/drain")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleShutdown)

	s.AddTool(mcp.NewTool("talos_reset",
		mcp.WithDescription("Reset a Talos node (wipe and return to maintenance mode)."),
		mcp.WithBoolean("graceful", mcp.Description("Graceful reset with cordon/drain and etcd leave (default: true)")),
		mcp.WithBoolean("reboot", mcp.Description("Reboot after reset instead of shutting down (default: false)")),
		mcp.WithString("wipe_mode", mcp.Description("Wipe mode (default: all)"), mcp.Enum("all", "system-disk", "user-disks")),
		mcp.WithString("system_labels_to_wipe", mcp.Description("Comma-separated partition labels to wipe selectively (e.g. 'u-nvme,EPHEMERAL')")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleReset)

	s.AddTool(mcp.NewTool("talos_upgrade",
		mcp.WithDescription("Upgrade Talos on a node — full talosctl-equivalent flow in a single call. With auto_reboot=true (default): cordons + drains the Kubernetes node (using the kubectl drain library, with PDB-aware retries, DaemonSet/mirror-pod skipping, and emptyDir handling), then installs the new version (auto-detecting LifecycleService on v1.13+ or legacy MachineService.Upgrade on older servers), reboots the node, waits for both Talos and Kubernetes to come back, and uncordons. K8s steps are skipped gracefully if the node isn't registered as a Kubernetes member. Returns when the node is fully back in service. Set auto_reboot=false for install-only (no drain, no reboot, no wait, no uncordon) — useful when staging for a maintenance window. Response includes \"api\" (\"lifecycle\"|\"legacy\"), \"server_tag\", \"rebooted\", \"talos_back\", \"k8s_ready\", \"uncordoned\", \"k8s_node_name\", \"stages\", and on v1.13+ the resolved \"pulled_image\". Pass skip_drain=true to proceed when Kubernetes is unreachable (workloads will not be evicted)."),
		mcp.WithString("image", mcp.Required(), mcp.Description("Talos installer image reference. As of v1.14 ghcr.io/siderolabs/installer is no longer published — use the Image Factory, e.g. factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.14.0 (that schematic id is the empty/default one; use your own if the cluster was installed from a custom schematic). If the cluster uses extensions, point to the matching schematic — a stock image strips extensions on reboot.")),
		mcp.WithBoolean("auto_reboot", mcp.Description("Reboot the node into the new version after a successful install (default: true). On v1.13+ this issues an explicit Reboot RPC after LifecycleService.Upgrade completes; on <v1.13 it leaves the legacy upgrade RPC's auto-reboot in place. Set false to install-only: the new version is staged in the alternate A/B partition and META is updated, but the node keeps running the current version until you trigger talos_reboot yourself.")),
		mcp.WithString("reboot_mode", mcp.Description("Reboot mode: \"default\" (graceful, may use kexec) or \"powercycle\" (skip kexec, full hardware reboot). Applies to both the LifecycleService Reboot RPC and the legacy upgrade's built-in reboot. Ignored if auto_reboot=false."), mcp.Enum("default", "powercycle")),
		mcp.WithBoolean("skip_drain", mcp.Description("Proceed even if the Kubernetes node cannot be discovered or reached (default: false). Without this the upgrade aborts rather than rebooting a node whose workloads were never evicted.")),
		mcp.WithBoolean("force", mcp.Description("(<v1.13 only) Force upgrade, skip etcd health checks (may cause data loss). Ignored on v1.13+ servers — LifecycleService does not expose this option.")),
		mcp.WithBoolean("stage", mcp.Description("(<v1.13 only) Stage the upgrade to perform after next reboot. Equivalent to auto_reboot=false but legacy-specific. Prefer auto_reboot for cross-version code. Ignored on v1.13+ servers.")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleUpgrade)

	// --- Diagnostics ---

	s.AddTool(mcp.NewTool("talos_logs",
		mcp.WithDescription("Get service logs from a node."),
		mcp.WithString("service", mcp.Required(), mcp.Description("Service name (e.g. kubelet, etcd, apid, machined)")),
		mcp.WithNumber("tail_lines", mcp.Description("Number of lines from the end (default: 100)")),
		mcp.WithString("filter", mcp.Description("Filter string — only return log lines containing this text")),
		mcp.WithBoolean("kubernetes", mcp.Description("Use the k8s.io containerd namespace")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleLogs)

	s.AddTool(mcp.NewTool("talos_dmesg",
		mcp.WithDescription("Get kernel logs (dmesg) from a node."),
		mcp.WithBoolean("tail", mcp.Description("Only return recent messages (useful for large dmesg output). Without this, returns all messages since boot.")),
		mcp.WithString("filter", mcp.Description("Filter string — only return lines containing this text")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleDmesg)

	s.AddTool(mcp.NewTool("talos_services",
		mcp.WithDescription("List all services and their status on a node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleServices)

	s.AddTool(mcp.NewTool("talos_containers",
		mcp.WithDescription("List running containers on a node."),
		mcp.WithString("namespace", mcp.Description("Containerd namespace (default: cri = Kubernetes workloads; system = etcd, kubelet)"), mcp.Enum("cri", "system")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleContainers)

	s.AddTool(mcp.NewTool("talos_processes",
		mcp.WithDescription("List running processes on a node."),
		mcp.WithString("sort", mcp.Description("Sort by (default: rss)"), mcp.Enum("rss", "cpu")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleProcesses)

	// --- System info ---

	s.AddTool(mcp.NewTool("talos_disks",
		mcp.WithDescription("List disks on a node."),
		mcp.WithBoolean("insecure", mcp.Description("Use insecure mode for maintenance/bootstrap (no TLS auth)")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleDisks)

	s.AddTool(mcp.NewTool("talos_mounts",
		mcp.WithDescription("List mount points on a node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleMounts)

	s.AddTool(mcp.NewTool("talos_memory",
		mcp.WithDescription("Get memory usage info from a node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleMemory)

	s.AddTool(mcp.NewTool("talos_netstat",
		mcp.WithDescription("List network connections on a node."),
		mcp.WithString("filter", mcp.Description("Filter (default: all)"), mcp.Enum("all", "connected", "listening")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleNetstat)

	// --- etcd operations ---

	s.AddTool(mcp.NewTool("talos_etcd_members",
		mcp.WithDescription("List etcd cluster members."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleEtcdMembers)

	s.AddTool(mcp.NewTool("talos_etcd_snapshot",
		mcp.WithDescription("Create an etcd snapshot and save to a local file."),
		mcp.WithString("output_path", mcp.Required(), mcp.Description("Local file path to save the snapshot")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		mutating(),
	), handleEtcdSnapshot)

	s.AddTool(mcp.NewTool("talos_etcd_defrag",
		mcp.WithDescription("Defragment etcd on a node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleEtcdDefrag)

	s.AddTool(mcp.NewTool("talos_etcd_status",
		mcp.WithDescription("Get etcd status from a node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleEtcdStatus)

	s.AddTool(mcp.NewTool("talos_etcd_remove_member",
		mcp.WithDescription("Remove an etcd member by ID. Get member IDs from talos_etcd_members first. Required before resetting a control plane node."),
		mcp.WithString("member_id", mcp.Required(), mcp.Description("Etcd member ID to remove, as a decimal string (IDs are uint64 and exceed the range a JSON number represents exactly). Take it from talos_etcd_members.")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleEtcdRemoveMember)

	s.AddTool(mcp.NewTool("talos_etcd_forfeit_leadership",
		mcp.WithDescription("Make the current etcd leader forfeit its leadership. Useful before maintenance on the leader node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		mutating(),
	), handleEtcdForfeitLeadership)

	s.AddTool(mcp.NewTool("talos_etcd_leave",
		mcp.WithDescription("Make a node leave the etcd cluster gracefully."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleEtcdLeave)

	s.AddTool(mcp.NewTool("talos_etcd_alarm",
		mcp.WithDescription("List etcd alarms (e.g. NOSPACE when DB is full)."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleEtcdAlarm)

	s.AddTool(mcp.NewTool("talos_patch",
		mcp.WithDescription("Patch the running machine configuration on a node. Fetches the current config, applies a strategic merge patch, and sends it back. Like 'talosctl patch machineconfig'."),
		mcp.WithString("patch", mcp.Required(), mcp.Description("Strategic merge patch YAML to apply to the machine config")),
		mcp.WithString("mode", mcp.Description("Apply mode (default: auto). 'reboot' is deprecated upstream — prefer auto or no-reboot."), mcp.Enum("auto", "no-reboot", "reboot", "staged", "try")),
		mcp.WithBoolean("dry_run", mcp.Description("Preview the change without applying")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handlePatch)

	s.AddTool(mcp.NewTool("talos_kubeconfig",
		mcp.WithDescription("Retrieve the admin kubeconfig for the cluster. Returns the kubeconfig YAML content."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleKubeconfig)

	// --- Additional operations ---

	s.AddTool(mcp.NewTool("talos_rollback",
		mcp.WithDescription("Rollback a node to the previous Talos version (reverts a failed upgrade using the A/B partition scheme)."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleRollback)

	s.AddTool(mcp.NewTool("talos_service_restart",
		mcp.WithDescription("Restart a specific service on a node."),
		mcp.WithString("service", mcp.Required(), mcp.Description("Service name to restart (e.g. kubelet, etcd, containerd)")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleServiceRestart)

	s.AddTool(mcp.NewTool("talos_image_list",
		mcp.WithDescription("List cached container images on a node."),
		mcp.WithString("namespace", mcp.Description("Containerd namespace (default: cri = Kubernetes workloads; system = etcd, kubelet)"), mcp.Enum("cri", "system")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleImageList)

	s.AddTool(mcp.NewTool("talos_image_remove",
		mcp.WithDescription("Remove a cached container image from a node. Equivalent to `talosctl image remove`. Safe — running containers reference images by digest; removal only affects future pulls, which will re-fetch from the registry."),
		mcp.WithString("image", mcp.Required(), mcp.Description("Image reference: name (registry/repo:tag) or digest (sha256:...). Get exact refs from talos_image_list.")),
		mcp.WithString("namespace", mcp.Description("Containerd namespace (default: cri = Kubernetes workloads; system = etcd, kubelet)"), mcp.Enum("cri", "system")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleImageRemove)

	s.AddTool(mcp.NewTool("talos_image_prune",
		mcp.WithDescription("Remove cached images in a namespace that are not currently used by any running container. Plugin-level helper — talosctl has no built-in prune subcommand. Defaults to dry_run=true: returns the list of candidates with sizes and total reclaimable bytes; re-run with dry_run=false to actually delete. In-use detection matches on exact name, exact digest, or substring (a container's image ref often embeds `name@sha256:digest`)."),
		mcp.WithBoolean("dry_run", mcp.Description("Preview-only. Default true so the first call is always safe. Set false to actually remove.")),
		mcp.WithString("namespace", mcp.Description("Containerd namespace (default: cri = Kubernetes workloads; system = etcd, kubelet)"), mcp.Enum("cri", "system")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleImagePrune)

	s.AddTool(mcp.NewTool("talos_stats",
		mcp.WithDescription("Get container runtime stats (CPU, memory usage per container)."),
		mcp.WithString("namespace", mcp.Description("Containerd namespace (default: cri = Kubernetes workloads; system = etcd, kubelet)"), mcp.Enum("cri", "system")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleStats)

	s.AddTool(mcp.NewTool("talos_ls",
		mcp.WithDescription("List files and directories on a node's filesystem."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory path to list")),
		mcp.WithBoolean("recurse", mcp.Description("List one level of subdirectories")),
		mcp.WithString("pattern", mcp.Description("Filter — only return entries whose name contains this string")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleLS)

	s.AddTool(mcp.NewTool("talos_read",
		mcp.WithDescription("Read a file from a node's filesystem."),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path to read")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleRead)

	s.AddTool(mcp.NewTool("talos_disk_usage",
		mcp.WithDescription("Get disk usage for a path on a node (like df/du)."),
		mcp.WithString("path", mcp.Description("Path to check (default: /)")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleDiskUsage)

	s.AddTool(mcp.NewTool("talos_time",
		mcp.WithDescription("Get current time and NTP sync status from a node."),
		mcp.WithString("server", mcp.Description("Optional NTP server to check against")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), handleTime)

	s.AddTool(mcp.NewTool("talos_wipe",
		mcp.WithDescription("Wipe a block device on a node. DESTRUCTIVE — use with caution."),
		mcp.WithString("device", mcp.Required(), mcp.Description("Device name without /dev/ prefix (e.g. sdb, nvme0n1)")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		destructive(),
	), handleWipe)

	// --- COSI resource tools (semantic wrappers around talos_get) ---

	s.AddTool(mcp.NewTool("talos_volumes",
		mcp.WithDescription("List volume statuses on a node (mount points, sizes, labels, provisioning state)."),
		mcp.WithString("id", mcp.Description("Optional volume ID")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("volumestatuses"))

	s.AddTool(mcp.NewTool("talos_discovered_volumes",
		mcp.WithDescription("List discovered block devices and partitions (dev path, size, filesystem, label, bus path)."),
		mcp.WithString("id", mcp.Description("Optional device ID (e.g. sda, sda1)")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("discoveredvolumes"))

	s.AddTool(mcp.NewTool("talos_addresses",
		mcp.WithDescription("List IP addresses assigned to network interfaces on a node."),
		mcp.WithString("id", mcp.Description("Optional address ID")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("addresses"))

	s.AddTool(mcp.NewTool("talos_routes",
		mcp.WithDescription("List routing table entries on a node."),
		mcp.WithString("id", mcp.Description("Optional route ID")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("routes"))

	s.AddTool(mcp.NewTool("talos_interfaces",
		mcp.WithDescription("List network interfaces (links) on a node — status, MTU, speed, hardware addr."),
		mcp.WithString("id", mcp.Description("Optional interface name")),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("links"))

	s.AddTool(mcp.NewTool("talos_cpu",
		mcp.WithDescription("Get CPU usage statistics from a node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("cpustat"))

	s.AddTool(mcp.NewTool("talos_extensions",
		mcp.WithDescription("List installed system extensions on a node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("extensions"))

	s.AddTool(mcp.NewTool("talos_machine_config",
		mcp.WithDescription("Get the running machine configuration from a node (v1alpha1 YAML)."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("mc"))

	s.AddTool(mcp.NewTool("talos_members",
		mcp.WithDescription("List cluster members (discovered via Talos discovery service)."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("members"))

	s.AddTool(mcp.NewTool("talos_resolvers",
		mcp.WithDescription("List configured DNS resolvers on a node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("resolvers"))

	s.AddTool(mcp.NewTool("talos_hostname",
		mcp.WithDescription("Get the hostname of a node."),
		nodeParam(),
		contextParam(),
		endpointParam(),
		readOnly(),
	), resourceHandler("hostname"))
}

// Shared parameter definitions. These appear on nearly every tool, so they are
// defined once rather than repeated in each schema.

// nodeParam is the single-node target. Multi-node fan-out is deliberately not
// supported: setupClient rejects a "nodes" array so callers get a clear error
// instead of silently reading the endpoint's own data.
func nodeParam() mcp.ToolOption {
	return mcp.WithString("node",
		mcp.Description("Target node IP or hostname. Exactly one node per call; issue one call per node to fan out."))
}

func contextParam() mcp.ToolOption {
	return mcp.WithString("context", mcp.Description("Talosconfig context name"))
}

// endpointParam overrides the endpoint from the talosconfig. Needed when the
// configured endpoints are unreachable and a node must be addressed directly —
// the recovery case, which is exactly when the talosconfig is least reliable.
func endpointParam() mcp.ToolOption {
	return mcp.WithString("endpoint",
		mcp.Description("Override the talosconfig endpoint and connect to this address directly. Use when the configured endpoints are down."))
}

// readOnly marks a tool that only observes cluster state.
func readOnly() mcp.ToolOption {
	return mcp.WithToolAnnotation(mcp.ToolAnnotation{
		ReadOnlyHint:  mcp.ToBoolPtr(true),
		OpenWorldHint: mcp.ToBoolPtr(true),
	})
}

// destructive marks a tool that can disrupt or destroy cluster state, so a host
// can gate or badge it rather than treating it like a read.
func destructive() mcp.ToolOption {
	return mcp.WithToolAnnotation(mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(true),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	})
}

// mutating marks a tool that changes state without being destructive.
func mutating() mcp.ToolOption {
	return mcp.WithToolAnnotation(mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(false),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	})
}
