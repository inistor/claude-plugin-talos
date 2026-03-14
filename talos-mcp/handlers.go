package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// byteStream is implemented by Logs and Dmesg stream responses.
type byteStream interface {
	Recv() (*common.Data, error)
}

// collectStream reads all data from a byte stream, stopping at EOF.
func collectStream(stream byteStream) (string, error) {
	var lines []string
	for {
		data, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		lines = append(lines, string(data.GetBytes()))
	}
	return strings.Join(lines, ""), nil
}

// --- Configuration management ---

func handleSetConfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	content, _ := args["content"].(string)

	// Try base64 decode first — if it succeeds, use the decoded content
	if decoded, err := base64.StdEncoding.DecodeString(content); err == nil {
		content = string(decoded)
	}

	path, err := setConfigFromContent(content)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to set config: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Talosconfig set (written to %s)", path)), nil
}

func handleConfigInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfgPath := getConfigPath()
	if cfgPath == "" {
		cfgPath = os.Getenv("TALOSCONFIG")
	}
	if cfgPath == "" {
		home, _ := os.UserHomeDir()
		cfgPath = home + "/.talos/config"
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read talosconfig failed: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
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

	args := req.GetArguments()
	waitTimeout := 300 * time.Second
	if v, ok := args["wait_timeout"].(float64); ok {
		waitTimeout = time.Duration(v) * time.Second
	}

	resp, err := c.ClusterHealthCheck(nCtx, waitTimeout, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("health check failed: %v", err)), nil
	}

	var messages []string
	for {
		msg, err := resp.Recv()
		if err != nil {
			if err != io.EOF {
				messages = append(messages, fmt.Sprintf("error: %v", err))
			}
			break
		}
		messages = append(messages, msg.GetMessage())
	}
	return jsonResult(map[string]any{"messages": messages})
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

	args := req.GetArguments()
	short, _ := args["short"].(bool)

	if short {
		var tags []string
		for _, msg := range resp.GetMessages() {
			tags = append(tags, msg.GetVersion().GetTag())
		}
		return mcp.NewToolResultText(strings.Join(tags, ", ")), nil
	}

	var results []map[string]any
	for _, msg := range resp.GetMessages() {
		v := msg.GetVersion()
		results = append(results, map[string]any{
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
	dryRun, _ := args["dry_run"].(bool)

	applyMode := machine.ApplyConfigurationRequest_AUTO
	switch strings.ToLower(mode) {
	case "no-reboot":
		applyMode = machine.ApplyConfigurationRequest_NO_REBOOT
	case "reboot":
		applyMode = machine.ApplyConfigurationRequest_REBOOT
	case "staged":
		applyMode = machine.ApplyConfigurationRequest_STAGED
	case "try":
		applyMode = machine.ApplyConfigurationRequest_TRY
	}

	resp, err := c.ApplyConfiguration(nCtx, &machine.ApplyConfigurationRequest{
		Data:   []byte(config),
		Mode:   applyMode,
		DryRun: dryRun,
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

	args := req.GetArguments()
	var opts []client.RebootMode
	if mode, ok := args["mode"].(string); ok {
		switch strings.ToLower(mode) {
		case "powercycle":
			opts = append(opts, client.WithPowerCycle)
		case "force":
			opts = append(opts, client.WithForce)
		}
	}

	if err := c.Reboot(nCtx, opts...); err != nil {
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

	args := req.GetArguments()
	force, _ := args["force"].(bool)

	var opts []client.ShutdownOption
	if force {
		opts = append(opts, client.WithShutdownForce(true))
	}

	if err := c.Shutdown(nCtx, opts...); err != nil {
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

	resetReq := &machine.ResetRequest{
		Graceful: graceful,
		Reboot:   reboot,
	}

	if wipeMode, ok := args["wipe_mode"].(string); ok {
		switch strings.ToLower(wipeMode) {
		case "system-disk":
			resetReq.Mode = machine.ResetRequest_SYSTEM_DISK
		case "user-disks":
			resetReq.Mode = machine.ResetRequest_USER_DISKS
		default:
			resetReq.Mode = machine.ResetRequest_ALL
		}
	}

	if err := c.ResetGeneric(nCtx, resetReq); err != nil {
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
	force, _ := args["force"].(bool)
	stage, _ := args["stage"].(bool)

	upgradeOpts := []client.UpgradeOption{
		client.WithUpgradeImage(image),
		client.WithUpgradeForce(force),
		client.WithUpgradeStage(stage),
	}

	if rebootMode, ok := args["reboot_mode"].(string); ok && strings.ToLower(rebootMode) == "powercycle" {
		upgradeOpts = append(upgradeOpts, client.WithUpgradeRebootMode(machine.UpgradeRequest_POWERCYCLE))
	}

	resp, err := c.UpgradeWithOptions(nCtx, upgradeOpts...)
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

	namespace := "system"
	driver := common.ContainerDriver_CONTAINERD
	if k8s, ok := args["kubernetes"].(bool); ok && k8s {
		namespace = "k8s.io"
		driver = common.ContainerDriver_CRI
	}

	stream, err := c.Logs(nCtx, namespace, driver, service, false, tailLines)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("logs failed: %v", err)), nil
	}

	output, err := collectStream(stream)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("logs stream failed: %v", err)), nil
	}
	return mcp.NewToolResultText(output), nil
}

func handleDmesg(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	tail, _ := args["tail"].(bool)

	stream, err := c.Dmesg(nCtx, false, tail)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("dmesg failed: %v", err)), nil
	}

	output, err := collectStream(stream)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("dmesg stream failed: %v", err)), nil
	}
	return mcp.NewToolResultText(output), nil
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

	var services []map[string]any
	for _, msg := range resp.GetMessages() {
		for _, svc := range msg.GetServices() {
			services = append(services, map[string]any{
				"id":     svc.GetId(),
				"state":  svc.GetState(),
				"health": svc.GetHealth().GetHealthy(),
				"events": len(svc.GetEvents().GetEvents()),
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

	var containers []map[string]any
	for _, msg := range resp.GetMessages() {
		for _, ct := range msg.GetContainers() {
			containers = append(containers, map[string]any{
				"id":        ct.GetId(),
				"pod_id":    ct.GetPodId(),
				"name":      ct.GetName(),
				"status":    ct.GetStatus(),
				"image":     ct.GetImage(),
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

	// Sort parameter is for client-side sorting — the API returns all processes
	args := req.GetArguments()
	sortBy, _ := args["sort"].(string)
	_ = sortBy // TODO: implement client-side sort by rss/cpu if needed

	var procs []map[string]any
	for _, msg := range resp.GetMessages() {
		for _, p := range msg.GetProcesses() {
			procs = append(procs, map[string]any{
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

	var disks []map[string]any
	for _, msg := range resp.GetMessages() {
		for _, d := range msg.GetDisks() {
			disks = append(disks, map[string]any{
				"device":      d.GetDeviceName(),
				"size":        d.GetSize(),
				"model":       d.GetModel(),
				"serial":      d.GetSerial(),
				"type":        d.GetType(),
				"bus_path":    d.GetBusPath(),
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

	var mounts []map[string]any
	for _, msg := range resp.GetMessages() {
		for _, m := range msg.GetStats() {
			mounts = append(mounts, map[string]any{
				"filesystem":  m.GetFilesystem(),
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

func handleNetstat(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	filter := machine.NetstatRequest_ALL
	if f, ok := args["filter"].(string); ok {
		switch strings.ToLower(f) {
		case "connected":
			filter = machine.NetstatRequest_CONNECTED
		case "listening":
			filter = machine.NetstatRequest_LISTENING
		}
	}

	resp, err := c.Netstat(nCtx, &machine.NetstatRequest{
		Filter: filter,
		Feature: &machine.NetstatRequest_Feature{
			Pid: true,
		},
		L4Proto: &machine.NetstatRequest_L4Proto{
			Tcp:  true,
			Tcp6: true,
			Udp:  true,
			Udp6: true,
		},
		Netns: &machine.NetstatRequest_NetNS{
			Hostnetwork: true,
		},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("netstat failed (may not be supported on this node): %v", err)), nil
	}
	if resp == nil {
		return mcp.NewToolResultError("netstat returned empty response"), nil
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

	var members []map[string]any
	for _, msg := range resp.GetMessages() {
		for _, m := range msg.GetMembers() {
			members = append(members, map[string]any{
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
	defer reader.Close()

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
