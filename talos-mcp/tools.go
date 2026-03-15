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
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleVersion)

	// --- Node operations ---

	s.AddTool(mcp.NewTool("talos_apply_config",
		mcp.WithDescription("Apply machine configuration to a node. Config is YAML string."),
		mcp.WithString("config", mcp.Required(), mcp.Description("Machine configuration YAML")),
		mcp.WithString("mode", mcp.Description("Apply mode: auto, no-reboot, reboot, staged, try (default: auto)")),
		mcp.WithBoolean("dry_run", mcp.Description("Check how the config change will be applied without actually applying")),
		mcp.WithString("timeout", mcp.Description("Rollback timeout for try mode (e.g. '1m', '5m'). Default: 1m")),
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
		mcp.WithBoolean("kubernetes", mcp.Description("Use the k8s.io containerd namespace")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleLogs)

	s.AddTool(mcp.NewTool("talos_dmesg",
		mcp.WithDescription("Get kernel logs (dmesg) from a node."),
		mcp.WithBoolean("tail", mcp.Description("Only return recent messages (useful for large dmesg output). Without this, returns all messages since boot.")),
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
}
