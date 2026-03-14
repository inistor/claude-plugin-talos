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
		mcp.WithDescription("Set the talosconfig file path for this session. All subsequent tools will use this config. Use when a local talosconfig exists instead of ~/.talos/config."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the talosconfig file")),
	), handleSetConfig)

	// --- Cluster operations ---

	s.AddTool(mcp.NewTool("talos_bootstrap",
		mcp.WithDescription("Bootstrap etcd on a control plane node. Only run on ONE node per cluster."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleBootstrap)

	s.AddTool(mcp.NewTool("talos_health",
		mcp.WithDescription("Check cluster health: etcd, API server, kubelet, connectivity."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleHealth)

	s.AddTool(mcp.NewTool("talos_version",
		mcp.WithDescription("Get Talos and Kubernetes version info from a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleVersion)

	s.AddTool(mcp.NewTool("talos_get",
		mcp.WithDescription("Get Talos resources (members, services, routes, addresses, etc). Like 'talosctl get <type> [id]'."),
		mcp.WithString("resource_type", mcp.Required(), mcp.Description("Resource type: members, services, routes, addresses, links, etc.")),
		mcp.WithString("resource_id", mcp.Description("Optional resource ID to get a specific resource")),
		mcp.WithString("namespace", mcp.Description("Resource namespace (default: runtime)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleGet)

	// --- Node operations ---

	s.AddTool(mcp.NewTool("talos_apply_config",
		mcp.WithDescription("Apply machine configuration to a node. Config is YAML string."),
		mcp.WithString("config", mcp.Required(), mcp.Description("Machine configuration YAML")),
		mcp.WithString("mode", mcp.Description("Apply mode: auto, no-reboot, staged, try (default: auto)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleApplyConfig)

	s.AddTool(mcp.NewTool("talos_reboot",
		mcp.WithDescription("Reboot a Talos node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleReboot)

	s.AddTool(mcp.NewTool("talos_shutdown",
		mcp.WithDescription("Shutdown a Talos node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleShutdown)

	s.AddTool(mcp.NewTool("talos_reset",
		mcp.WithDescription("Reset a Talos node (wipe and return to maintenance mode)."),
		mcp.WithBoolean("graceful", mcp.Description("Graceful reset (default: true)")),
		mcp.WithBoolean("reboot", mcp.Description("Reboot after reset (default: false)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleReset)

	s.AddTool(mcp.NewTool("talos_upgrade",
		mcp.WithDescription("Upgrade Talos on a node to a new version."),
		mcp.WithString("image", mcp.Required(), mcp.Description("Talos image reference (e.g. ghcr.io/siderolabs/installer:v1.12.3)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleUpgrade)

	// --- Diagnostics ---

	s.AddTool(mcp.NewTool("talos_logs",
		mcp.WithDescription("Get service logs from a node."),
		mcp.WithString("service", mcp.Required(), mcp.Description("Service name (e.g. kubelet, etcd, apid, machined)")),
		mcp.WithNumber("tail_lines", mcp.Description("Number of lines from the end (default: 100)")),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleLogs)

	s.AddTool(mcp.NewTool("talos_dmesg",
		mcp.WithDescription("Get kernel logs (dmesg) from a node."),
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

	s.AddTool(mcp.NewTool("talos_cpu",
		mcp.WithDescription("Get CPU usage info from a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleCPU)

	s.AddTool(mcp.NewTool("talos_netstat",
		mcp.WithDescription("List network connections on a node."),
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

	// --- Configuration ---

	s.AddTool(mcp.NewTool("talos_gen_config",
		mcp.WithDescription("Generate machine configuration for a new cluster."),
		mcp.WithString("cluster_name", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("endpoint", mcp.Required(), mcp.Description("Control plane endpoint URL (e.g. https://10.0.0.1:6443)")),
		mcp.WithString("config_patch", mcp.Description("Optional YAML config patch to apply")),
	), handleGenConfig)

	s.AddTool(mcp.NewTool("talos_patch",
		mcp.WithDescription("Apply a strategic merge patch to a machine configuration file."),
		mcp.WithString("config", mcp.Required(), mcp.Description("Original machine configuration YAML")),
		mcp.WithString("patch", mcp.Required(), mcp.Description("Patch YAML to apply")),
	), handlePatch)

	s.AddTool(mcp.NewTool("talos_get_config",
		mcp.WithDescription("Get the current machine configuration from a node."),
		mcp.WithString("node", mcp.Description("Target node IP or hostname")),
		mcp.WithString("context", mcp.Description("Talosconfig context name")),
	), handleGetConfig)

	// --- Contexts ---

	s.AddTool(mcp.NewTool("talos_config_contexts",
		mcp.WithDescription("List available talosconfig contexts."),
	), handleConfigContexts)

	s.AddTool(mcp.NewTool("talos_config_info",
		mcp.WithDescription("Show current talosconfig context info (endpoints, nodes)."),
		mcp.WithString("context", mcp.Description("Talosconfig context name (default: current)")),
	), handleConfigInfo)
}
