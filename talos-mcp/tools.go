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
	), handleSetConfig)

	s.AddTool(mcp.NewTool("talos_config_info",
		mcp.WithDescription("Show current talosconfig content (contexts, endpoints, nodes)."),
	), handleConfigInfo)

	s.AddTool(mcp.NewTool("talos_get",
		mcp.WithDescription("Get Talos resources by type. Supports aliases (e.g. 'mc', 'addresses', 'volumes', 'members', 'extensions', 'links', 'routes'). Like 'talosctl get <type> [id]'."),
		mcp.WithString("resource_type", mcp.Required(), mcp.Description("Resource type or alias: addresses, routes, links, members, mc, volumes, extensions, discoveredvolumes, cpustat, etc.")),
		mcp.WithString("resource_id", mcp.Description("Optional resource ID to get a specific resource")),
		mcp.WithString("namespace", mcp.Description("Resource namespace (auto-detected if omitted)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleGet)

	// --- Cluster operations ---

	s.AddTool(mcp.NewTool("talos_bootstrap",
		mcp.WithDescription("Bootstrap etcd on a control plane node. Only run on ONE node per cluster. For etcd recovery, use talosctl bootstrap --recover-from via Bash."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleBootstrap)

	s.AddTool(mcp.NewTool("talos_health",
		mcp.WithDescription("Check cluster health: etcd, API server, kubelet, connectivity. Note: 'finish boot sequence' check on control plane nodes may timeout — this is normal for long-running CPs."),
		mcp.WithNumber("wait_timeout", mcp.Description("Timeout in seconds to wait for cluster to be ready (default: 300)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleHealth)

	s.AddTool(mcp.NewTool("talos_version",
		mcp.WithDescription("Get Talos and Kubernetes version info from a node."),
		mcp.WithBoolean("short", mcp.Description("Print short version string only")),
		mcp.WithBoolean("insecure", mcp.Description("Use insecure mode for maintenance/bootstrap (no TLS auth)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleVersion)

	// --- Node operations ---

	s.AddTool(mcp.NewTool("talos_apply_config",
		mcp.WithDescription("Apply FULL machine configuration to a node. Requires the complete config YAML (not a patch). For partial changes, use talos_patch instead."),
		mcp.WithString("config", mcp.Required(), mcp.Description("Complete machine configuration YAML")),
		mcp.WithString("mode", mcp.Description("Apply mode: auto, no-reboot, reboot, staged, try (default: auto)")),
		mcp.WithBoolean("dry_run", mcp.Description("Check how the config change will be applied without actually applying")),
		mcp.WithBoolean("insecure", mcp.Description("Use insecure mode for maintenance/bootstrap (no TLS auth)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleApplyConfig)

	s.AddTool(mcp.NewTool("talos_reboot",
		mcp.WithDescription("Reboot a Talos node."),
		mcp.WithString("mode", mcp.Description("Reboot mode: default, powercycle (skip kexec), force (skip graceful teardown)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleReboot)

	s.AddTool(mcp.NewTool("talos_shutdown",
		mcp.WithDescription("Shutdown a Talos node."),
		mcp.WithBoolean("force", mcp.Description("Force shutdown without cordon/drain")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleShutdown)

	s.AddTool(mcp.NewTool("talos_reset",
		mcp.WithDescription("Reset a Talos node (wipe and return to maintenance mode)."),
		mcp.WithBoolean("graceful", mcp.Description("Graceful reset with cordon/drain and etcd leave (default: true)")),
		mcp.WithBoolean("reboot", mcp.Description("Reboot after reset instead of shutting down (default: false)")),
		mcp.WithString("wipe_mode", mcp.Description("Wipe mode: all (default), system-disk, user-disks")),
		mcp.WithString("system_labels_to_wipe", mcp.Description("Comma-separated partition labels to wipe selectively (e.g. 'u-nvme,EPHEMERAL')")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleReset)

	s.AddTool(mcp.NewTool("talos_upgrade",
		mcp.WithDescription("Upgrade Talos on a node to a new version."),
		mcp.WithString("image", mcp.Required(), mcp.Description("Talos image reference (e.g. ghcr.io/siderolabs/installer:v1.12.3)")),
		mcp.WithBoolean("force", mcp.Description("Force upgrade, skip etcd health checks (may cause data loss)")),
		mcp.WithBoolean("stage", mcp.Description("Stage the upgrade to perform after next reboot")),
		mcp.WithString("reboot_mode", mcp.Description("Reboot mode during upgrade: default, powercycle")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleUpgrade)

	// --- Diagnostics ---

	s.AddTool(mcp.NewTool("talos_logs",
		mcp.WithDescription("Get service logs from a node."),
		mcp.WithString("service", mcp.Required(), mcp.Description("Service name (e.g. kubelet, etcd, apid, machined)")),
		mcp.WithNumber("tail_lines", mcp.Description("Number of lines from the end (default: 100)")),
		mcp.WithString("filter", mcp.Description("Filter string — only return log lines containing this text")),
		mcp.WithBoolean("kubernetes", mcp.Description("Use the k8s.io containerd namespace")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleLogs)

	s.AddTool(mcp.NewTool("talos_dmesg",
		mcp.WithDescription("Get kernel logs (dmesg) from a node."),
		mcp.WithBoolean("tail", mcp.Description("Only return recent messages (useful for large dmesg output). Without this, returns all messages since boot.")),
		mcp.WithString("filter", mcp.Description("Filter string — only return lines containing this text")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleDmesg)

	s.AddTool(mcp.NewTool("talos_services",
		mcp.WithDescription("List all services and their status on a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleServices)

	s.AddTool(mcp.NewTool("talos_containers",
		mcp.WithDescription("List running containers on a node."),
		mcp.WithString("namespace", mcp.Description("Container namespace: system, cri (default: cri)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleContainers)

	s.AddTool(mcp.NewTool("talos_processes",
		mcp.WithDescription("List running processes on a node."),
		mcp.WithString("sort", mcp.Description("Sort by: rss (default), cpu")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleProcesses)

	// --- System info ---

	s.AddTool(mcp.NewTool("talos_disks",
		mcp.WithDescription("List disks on a node."),
		mcp.WithBoolean("insecure", mcp.Description("Use insecure mode for maintenance/bootstrap (no TLS auth)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleDisks)

	s.AddTool(mcp.NewTool("talos_mounts",
		mcp.WithDescription("List mount points on a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleMounts)

	s.AddTool(mcp.NewTool("talos_memory",
		mcp.WithDescription("Get memory usage info from a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleMemory)

	s.AddTool(mcp.NewTool("talos_netstat",
		mcp.WithDescription("List network connections on a node."),
		mcp.WithString("filter", mcp.Description("Filter: all (default), connected, listening")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleNetstat)

	// --- etcd operations ---

	s.AddTool(mcp.NewTool("talos_etcd_members",
		mcp.WithDescription("List etcd cluster members."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleEtcdMembers)

	s.AddTool(mcp.NewTool("talos_etcd_snapshot",
		mcp.WithDescription("Create an etcd snapshot and save to a local file."),
		mcp.WithString("output_path", mcp.Required(), mcp.Description("Local file path to save the snapshot")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleEtcdSnapshot)

	s.AddTool(mcp.NewTool("talos_etcd_defrag",
		mcp.WithDescription("Defragment etcd on a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleEtcdDefrag)

	s.AddTool(mcp.NewTool("talos_etcd_status",
		mcp.WithDescription("Get etcd status from a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleEtcdStatus)

	s.AddTool(mcp.NewTool("talos_etcd_remove_member",
		mcp.WithDescription("Remove an etcd member by ID. Get member IDs from talos_etcd_members first. Required before resetting a control plane node."),
		mcp.WithNumber("member_id", mcp.Required(), mcp.Description("Etcd member ID to remove")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleEtcdRemoveMember)

	s.AddTool(mcp.NewTool("talos_etcd_forfeit_leadership",
		mcp.WithDescription("Make the current etcd leader forfeit its leadership. Useful before maintenance on the leader node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleEtcdForfeitLeadership)

	s.AddTool(mcp.NewTool("talos_etcd_leave",
		mcp.WithDescription("Make a node leave the etcd cluster gracefully."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleEtcdLeave)

	s.AddTool(mcp.NewTool("talos_etcd_alarm",
		mcp.WithDescription("List etcd alarms (e.g. NOSPACE when DB is full)."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleEtcdAlarm)

	s.AddTool(mcp.NewTool("talos_patch",
		mcp.WithDescription("Patch the running machine configuration on a node. Fetches the current config, applies a strategic merge patch, and sends it back. Like 'talosctl patch machineconfig'."),
		mcp.WithString("patch", mcp.Required(), mcp.Description("Strategic merge patch YAML to apply to the machine config")),
		mcp.WithString("mode", mcp.Description("Apply mode: auto (default), no-reboot, reboot, staged, try")),
		mcp.WithBoolean("dry_run", mcp.Description("Preview the change without applying")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handlePatch)

	s.AddTool(mcp.NewTool("talos_kubeconfig",
		mcp.WithDescription("Retrieve the admin kubeconfig for the cluster. Returns the kubeconfig YAML content."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleKubeconfig)

	// --- Additional operations ---

	s.AddTool(mcp.NewTool("talos_rollback",
		mcp.WithDescription("Rollback a node to the previous Talos version (reverts a failed upgrade using the A/B partition scheme)."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleRollback)

	s.AddTool(mcp.NewTool("talos_service_restart",
		mcp.WithDescription("Restart a specific service on a node."),
		mcp.WithString("service", mcp.Required(), mcp.Description("Service name to restart (e.g. kubelet, etcd, containerd)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleServiceRestart)

	s.AddTool(mcp.NewTool("talos_image_list",
		mcp.WithDescription("List cached container images on a node."),
		mcp.WithString("namespace", mcp.Description("Containerd namespace: cri (default), system")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleImageList)

	s.AddTool(mcp.NewTool("talos_stats",
		mcp.WithDescription("Get container runtime stats (CPU, memory usage per container)."),
		mcp.WithString("namespace", mcp.Description("Container namespace: cri (default), system")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleStats)

	s.AddTool(mcp.NewTool("talos_ls",
		mcp.WithDescription("List files and directories on a node's filesystem."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory path to list")),
		mcp.WithBoolean("recurse", mcp.Description("List one level of subdirectories")),
		mcp.WithString("pattern", mcp.Description("Filter — only return entries whose name contains this string")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleLS)

	s.AddTool(mcp.NewTool("talos_read",
		mcp.WithDescription("Read a file from a node's filesystem."),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path to read")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleRead)

	s.AddTool(mcp.NewTool("talos_disk_usage",
		mcp.WithDescription("Get disk usage for a path on a node (like df/du)."),
		mcp.WithString("path", mcp.Description("Path to check (default: /)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleDiskUsage)

	s.AddTool(mcp.NewTool("talos_time",
		mcp.WithDescription("Get current time and NTP sync status from a node."),
		mcp.WithString("server", mcp.Description("Optional NTP server to check against")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleTime)

	s.AddTool(mcp.NewTool("talos_wipe",
		mcp.WithDescription("Wipe a block device on a node. DESTRUCTIVE — use with caution."),
		mcp.WithString("device", mcp.Required(), mcp.Description("Device name without /dev/ prefix (e.g. sdb, nvme0n1)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleWipe)

	// --- COSI resource tools (semantic wrappers around talos_get) ---

	s.AddTool(mcp.NewTool("talos_volumes",
		mcp.WithDescription("List volume statuses on a node (mount points, sizes, labels, provisioning state)."),
		mcp.WithString("id", mcp.Description("Optional volume ID")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("volumestatuses"))

	s.AddTool(mcp.NewTool("talos_discovered_volumes",
		mcp.WithDescription("List discovered block devices and partitions (dev path, size, filesystem, label, bus path)."),
		mcp.WithString("id", mcp.Description("Optional device ID (e.g. sda, sda1)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("discoveredvolumes"))

	s.AddTool(mcp.NewTool("talos_addresses",
		mcp.WithDescription("List IP addresses assigned to network interfaces on a node."),
		mcp.WithString("id", mcp.Description("Optional address ID")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("addresses"))

	s.AddTool(mcp.NewTool("talos_routes",
		mcp.WithDescription("List routing table entries on a node."),
		mcp.WithString("id", mcp.Description("Optional route ID")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("routes"))

	s.AddTool(mcp.NewTool("talos_interfaces",
		mcp.WithDescription("List network interfaces (links) on a node — status, MTU, speed, hardware addr."),
		mcp.WithString("id", mcp.Description("Optional interface name")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("links"))

	s.AddTool(mcp.NewTool("talos_cpu",
		mcp.WithDescription("Get CPU usage statistics from a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("cpustat"))

	s.AddTool(mcp.NewTool("talos_extensions",
		mcp.WithDescription("List installed system extensions on a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("extensions"))

	s.AddTool(mcp.NewTool("talos_machine_config",
		mcp.WithDescription("Get the running machine configuration from a node (v1alpha1 YAML)."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("mc"))

	s.AddTool(mcp.NewTool("talos_members",
		mcp.WithDescription("List cluster members (discovered via Talos discovery service)."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("members"))

	s.AddTool(mcp.NewTool("talos_resolvers",
		mcp.WithDescription("List configured DNS resolvers on a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("resolvers"))

	s.AddTool(mcp.NewTool("talos_hostname",
		mcp.WithDescription("Get the hostname of a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), resourceHandler("hostname"))
}
