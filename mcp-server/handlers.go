package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// helper to extract common params
func extractParams(req mcp.CallToolRequest) (node, ctxName string) {
	args := req.GetArguments()
	node, _ = args["node"].(string)
	ctxName, _ = args["context"].(string)
	return
}

// helper to create client and node context
func setupClient(ctx context.Context, req mcp.CallToolRequest) (*client.Client, context.Context, error) {
	node, ctxName := extractParams(req)
	c, err := newClient(ctx, ctxName)
	if err != nil {
		return nil, nil, err
	}
	return c, nodeCtx(ctx, node), nil
}

// jsonResult marshals v to JSON and returns as tool result text.
func jsonResult(v interface{}) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// --- Cluster operations ---

func handleBootstrap(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	if err := c.Bootstrap(nCtx, &machine.BootstrapRequest{}); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("bootstrap failed: %v", err)), nil
	}
	return mcp.NewToolResultText("Bootstrap initiated successfully."), nil
}

func handleHealth(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.ClusterHealthCheck(nCtx, 0, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("health check failed: %v", err)), nil
	}

	var messages []string
	for {
		msg, err := resp.Recv()
		if err != nil {
			break
		}
		messages = append(messages, msg.GetMessage())
	}
	return jsonResult(map[string]interface{}{"messages": messages})
}

func handleVersion(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.Version(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("version failed: %v", err)), nil
	}

	var results []map[string]interface{}
	for _, msg := range resp.GetMessages() {
		v := msg.GetVersion()
		results = append(results, map[string]interface{}{
			"tag":        v.GetTag(),
			"sha":        v.GetSha(),
			"built":      v.GetBuilt(),
			"go_version": v.GetGoVersion(),
			"os":         v.GetOs(),
			"arch":       v.GetArch(),
		})
	}
	return jsonResult(results)
}

func handleGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// The COSI resource API requires complex type registration for generic resource
	// listing by string name. Delegate to talosctl with JSON output for this one tool.
	args := req.GetArguments()
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)
	namespace, _ := args["namespace"].(string)
	node, ctxName := extractParams(req)

	cmdArgs := []string{"get", resourceType, "--output", "json"}
	if resourceID != "" {
		cmdArgs = append(cmdArgs, resourceID)
	}
	if namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", namespace)
	}
	if node != "" {
		cmdArgs = append(cmdArgs, "-n", node)
	}
	if ctxName != "" {
		cmdArgs = append(cmdArgs, "--context", ctxName)
	}

	out, err := exec.CommandContext(ctx, "talosctl", cmdArgs...).CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("talosctl get failed: %s\n%s", err, string(out))), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

// --- Node operations ---

func handleApplyConfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	config, _ := args["config"].(string)
	mode, _ := args["mode"].(string)

	applyMode := machine.ApplyConfigurationRequest_AUTO
	switch strings.ToLower(mode) {
	case "no-reboot":
		applyMode = machine.ApplyConfigurationRequest_NO_REBOOT
	case "staged":
		applyMode = machine.ApplyConfigurationRequest_STAGED
	case "try":
		applyMode = machine.ApplyConfigurationRequest_TRY
	}

	resp, err := c.ApplyConfiguration(nCtx, &machine.ApplyConfigurationRequest{
		Data: []byte(config),
		Mode: applyMode,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("apply config failed: %v", err)), nil
	}
	return jsonResult(resp)
}

func handleReboot(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	if err := c.Reboot(nCtx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reboot failed: %v", err)), nil
	}
	return mcp.NewToolResultText("Reboot initiated."), nil
}

func handleShutdown(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	if err := c.Shutdown(nCtx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("shutdown failed: %v", err)), nil
	}
	return mcp.NewToolResultText("Shutdown initiated."), nil
}

func handleReset(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	graceful := true
	reboot := false
	if v, ok := args["graceful"].(bool); ok {
		graceful = v
	}
	if v, ok := args["reboot"].(bool); ok {
		reboot = v
	}

	if err := c.ResetGeneric(nCtx, &machine.ResetRequest{
		Graceful: graceful,
		Reboot:   reboot,
	}); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reset failed: %v", err)), nil
	}
	return mcp.NewToolResultText("Reset initiated."), nil
}

func handleUpgrade(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	image, _ := args["image"].(string)

	resp, err := c.Upgrade(nCtx, image, false, false)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("upgrade failed: %v", err)), nil
	}
	return jsonResult(resp)
}

// --- Diagnostics ---

func handleLogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	service, _ := args["service"].(string)
	tailLines := int32(100)
	if v, ok := args["tail_lines"].(float64); ok {
		tailLines = int32(v)
	}

	stream, err := c.Logs(nCtx, "system", common.ContainerDriver_CONTAINERD, service, false, tailLines)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("logs failed: %v", err)), nil
	}

	var lines []string
	for {
		data, err := stream.Recv()
		if err != nil {
			break
		}
		lines = append(lines, string(data.GetBytes()))
	}
	return mcp.NewToolResultText(strings.Join(lines, "")), nil
}

func handleDmesg(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	stream, err := c.Dmesg(nCtx, false, false)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("dmesg failed: %v", err)), nil
	}

	var lines []string
	for {
		data, err := stream.Recv()
		if err != nil {
			break
		}
		lines = append(lines, string(data.GetBytes()))
	}
	return mcp.NewToolResultText(strings.Join(lines, "")), nil
}

func handleServices(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.ServiceList(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("service list failed: %v", err)), nil
	}

	var services []map[string]interface{}
	for _, msg := range resp.GetMessages() {
		for _, svc := range msg.GetServices() {
			services = append(services, map[string]interface{}{
				"id":      svc.GetId(),
				"state":   svc.GetState(),
				"health":  svc.GetHealth().GetHealthy(),
				"events":  len(svc.GetEvents().GetEvents()),
			})
		}
	}
	return jsonResult(services)
}

func handleContainers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	namespace := "cri"
	if v, ok := args["namespace"].(string); ok && v != "" {
		namespace = v
	}

	driver := common.ContainerDriver_CRI
	if namespace == "system" {
		driver = common.ContainerDriver_CONTAINERD
	}
	resp, err := c.Containers(nCtx, namespace, driver)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("containers failed: %v", err)), nil
	}

	var containers []map[string]interface{}
	for _, msg := range resp.GetMessages() {
		for _, c := range msg.GetContainers() {
			containers = append(containers, map[string]interface{}{
				"id":        c.GetId(),
				"pod_id":    c.GetPodId(),
				"name":      c.GetName(),
				"status":    c.GetStatus(),
				"image":     c.GetImage(),
				"namespace": namespace,
			})
		}
	}
	return jsonResult(containers)
}

func handleProcesses(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.Processes(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("processes failed: %v", err)), nil
	}

	var procs []map[string]interface{}
	for _, msg := range resp.GetMessages() {
		for _, p := range msg.GetProcesses() {
			procs = append(procs, map[string]interface{}{
				"pid":     p.GetPid(),
				"ppid":    p.GetPpid(),
				"state":   p.GetState(),
				"command": p.GetCommand(),
				"threads": p.GetThreads(),
			})
		}
	}
	return jsonResult(procs)
}

// --- System info ---

func handleDisks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.Disks(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("disks failed: %v", err)), nil
	}

	var disks []map[string]interface{}
	for _, msg := range resp.GetMessages() {
		for _, d := range msg.GetDisks() {
			disks = append(disks, map[string]interface{}{
				"device":     d.GetDeviceName(),
				"size":       d.GetSize(),
				"model":      d.GetModel(),
				"serial":     d.GetSerial(),
				"type":       d.GetType(),
				"bus_path":   d.GetBusPath(),
				"system_disk": d.GetSystemDisk(),
			})
		}
	}
	return jsonResult(disks)
}

func handleMounts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.Mounts(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("mounts failed: %v", err)), nil
	}

	var mounts []map[string]interface{}
	for _, msg := range resp.GetMessages() {
		for _, m := range msg.GetStats() {
			mounts = append(mounts, map[string]interface{}{
				"filesystem": m.GetFilesystem(),
				"mount_point": m.GetMountedOn(),
				"size":        m.GetSize(),
				"available":   m.GetAvailable(),
			})
		}
	}
	return jsonResult(mounts)
}

func handleMemory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.Memory(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("memory failed: %v", err)), nil
	}
	return jsonResult(resp)
}

func handleCPU(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// No direct CPU method on the client — delegate to talosctl
	node, ctxName := extractParams(req)
	_ = ctx // used by exec.CommandContext below
	cmdArgs := []string{"get", "cpustat", "-o", "json"}
	if node != "" {
		cmdArgs = append(cmdArgs, "-n", node)
	}
	if ctxName != "" {
		cmdArgs = append(cmdArgs, "--context", ctxName)
	}
	out, err := exec.CommandContext(ctx, "talosctl", cmdArgs...).CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("cpu failed: %s\n%s", err, string(out))), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func handleNetstat(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.Netstat(nCtx, &machine.NetstatRequest{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("netstat failed: %v", err)), nil
	}
	return jsonResult(resp)
}

// --- etcd operations ---

func handleEtcdMembers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.EtcdMemberList(nCtx, &machine.EtcdMemberListRequest{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("etcd members failed: %v", err)), nil
	}

	var members []map[string]interface{}
	for _, msg := range resp.GetMessages() {
		for _, m := range msg.GetMembers() {
			members = append(members, map[string]interface{}{
				"id":          m.GetId(),
				"hostname":    m.GetHostname(),
				"peer_urls":   m.GetPeerUrls(),
				"client_urls": m.GetClientUrls(),
				"is_learner":  m.GetIsLearner(),
			})
		}
	}
	return jsonResult(members)
}

func handleEtcdSnapshot(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	outputPath, _ := args["output_path"].(string)

	reader, err := c.EtcdSnapshot(nCtx, &machine.EtcdSnapshotRequest{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("etcd snapshot failed: %v", err)), nil
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create file failed: %v", err)), nil
	}
	defer f.Close()

	n, err := io.Copy(f, reader)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("write snapshot failed: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Snapshot saved to %s (%d bytes)", outputPath, n)), nil
}

func handleEtcdDefrag(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.EtcdDefragment(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("etcd defrag failed: %v", err)), nil
	}
	return jsonResult(resp)
}

func handleEtcdStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.EtcdStatus(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("etcd status failed: %v", err)), nil
	}
	return jsonResult(resp)
}

// --- Configuration ---

func handleGenConfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	clusterName, _ := args["cluster_name"].(string)
	endpoint, _ := args["endpoint"].(string)
	configPatch, _ := args["config_patch"].(string)

	cmdArgs := []string{"gen", "config", clusterName, endpoint, "--output-dir", "-"}
	if configPatch != "" {
		cmdArgs = append(cmdArgs, "--config-patch", configPatch)
	}

	out, err := exec.CommandContext(ctx, "talosctl", cmdArgs...).CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("gen config failed: %s\n%s", err, string(out))), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func handlePatch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	config, _ := args["config"].(string)
	patch, _ := args["patch"].(string)

	// Write config and patch to temp files, run talosctl machineconfig patch
	configFile, err := os.CreateTemp("", "talos-config-*.yaml")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create temp file failed: %v", err)), nil
	}
	defer os.Remove(configFile.Name())
	configFile.WriteString(config)
	configFile.Close()

	patchFile, err := os.CreateTemp("", "talos-patch-*.yaml")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create temp file failed: %v", err)), nil
	}
	defer os.Remove(patchFile.Name())
	patchFile.WriteString(patch)
	patchFile.Close()

	out, err := exec.CommandContext(ctx, "talosctl", "machineconfig", "patch",
		configFile.Name(), "--patch", "@"+patchFile.Name()).CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("patch failed: %s\n%s", err, string(out))), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func handleGetConfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Use talosctl get to retrieve the running machine config
	node, ctxName := extractParams(req)

	cmdArgs := []string{"get", "mc", "-o", "yaml"}
	if node != "" {
		cmdArgs = append(cmdArgs, "-n", node)
	}
	if ctxName != "" {
		cmdArgs = append(cmdArgs, "--context", ctxName)
	}

	out, err := exec.CommandContext(ctx, "talosctl", cmdArgs...).CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get config failed: %s\n%s", err, string(out))), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

// --- Contexts ---

func handleConfigContexts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfgPath := os.Getenv("TALOSCONFIG")
	if cfgPath == "" {
		home, _ := os.UserHomeDir()
		cfgPath = home + "/.talos/config"
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read talosconfig failed: %v", err)), nil
	}

	// Return raw config — Claude can parse with yq
	return mcp.NewToolResultText(string(data)), nil
}

func handleConfigInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Same as contexts — returns full config for Claude to parse
	return handleConfigContexts(ctx, req)
}
