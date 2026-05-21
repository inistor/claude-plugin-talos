package main

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	timeapi "github.com/siderolabs/talos/pkg/machinery/api/time"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"go.yaml.in/yaml/v4"
	"k8s.io/client-go/kubernetes"
)

// helper to extract common params
func extractParams(req mcp.CallToolRequest) (node, ctxName string, insecure bool) {
	args := req.GetArguments()
	node, _ = args["node"].(string)
	ctxName, _ = args["context"].(string)
	insecure, _ = args["insecure"].(bool)
	return
}

// helper to create client and node context
func setupClient(ctx context.Context, req mcp.CallToolRequest) (*client.Client, context.Context, error) {
	node, ctxName, insecure := extractParams(req)

	var (
		c   *client.Client
		err error
	)

	if insecure {
		if node == "" {
			return nil, nil, fmt.Errorf("node is required for insecure mode")
		}
		c, err = newInsecureClient(ctx, node)
	} else {
		c, err = newClient(ctx, ctxName)
	}
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

// statusAwareResult marshals payload to JSON and returns it as a tool result.
// The result is flagged as an error (IsError=true) if payload["status"] is set
// to anything other than "ok"/"" — or if any payload key ends in "_error"
// (drain_failed, uncordon_error, wait_error, ...). Callers see the failure at
// the protocol level, not just buried in the JSON body.
func statusAwareResult(payload map[string]any) (*mcp.CallToolResult, error) {
	isErr := false
	if s, _ := payload["status"].(string); s != "ok" && s != "" {
		isErr = true
	}
	for k := range payload {
		if strings.HasSuffix(k, "_error") {
			isErr = true
			break
		}
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	if isErr {
		return mcp.NewToolResultError(string(b)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// parseApplyMode maps the user-facing mode string onto the protobuf enum.
// An empty or unknown value yields AUTO, matching talosctl behaviour.
func parseApplyMode(mode string) machine.ApplyConfigurationRequest_Mode {
	switch strings.ToLower(mode) {
	case "no-reboot":
		return machine.ApplyConfigurationRequest_NO_REBOOT
	case "reboot":
		return machine.ApplyConfigurationRequest_REBOOT
	case "staged":
		return machine.ApplyConfigurationRequest_STAGED
	case "try":
		return machine.ApplyConfigurationRequest_TRY
	default:
		return machine.ApplyConfigurationRequest_AUTO
	}
}

// byteStream is implemented by Logs and Dmesg stream responses.
type byteStream interface {
	Recv() (*common.Data, error)
}

// collectStream reads all data from a byte stream, stopping at EOF.
// If filter is non-empty, only lines containing the filter string are included.
func collectStream(stream byteStream, filter string) (string, error) {
	var lines []string
	for {
		data, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		chunk := string(data.GetBytes())
		if filter == "" {
			lines = append(lines, chunk)
			continue
		}
		for line := range strings.SplitSeq(chunk, "\n") {
			if strings.Contains(line, filter) {
				lines = append(lines, line+"\n")
			}
		}
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

	cfg, err := clientconfig.Open(cfgPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read talosconfig failed: %v", err)), nil
	}

	info := map[string]any{
		"current_context": cfg.Context,
		"config_path":     cfgPath,
		"contexts":        map[string]any{},
	}

	ctxMap := info["contexts"].(map[string]any)
	for name, c := range cfg.Contexts {
		ctxInfo := map[string]any{
			"endpoints": c.Endpoints,
		}
		if len(c.Nodes) > 0 {
			ctxInfo["nodes"] = c.Nodes
		}
		if c.Cluster != "" {
			ctxInfo["cluster"] = c.Cluster
		}
		// Parse cert to get expiry and roles
		if c.Crt != "" {
			if crtBytes, err := base64.StdEncoding.DecodeString(c.Crt); err == nil {
				if block, _ := pem.Decode(crtBytes); block != nil {
					if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
						ctxInfo["roles"] = cert.Subject.Organization
						ctxInfo["certificate_expires"] = cert.NotAfter.Format(time.RFC3339)
					}
				}
			}
		}
		ctxMap[name] = ctxInfo
	}

	return jsonResult(info)
}

// --- Resource operations ---

// getResource lists or gets COSI resources by type alias. Shared by handleGet and dedicated resource tools.
func getResource(c *client.Client, nCtx context.Context, resourceType, resourceID string) (*mcp.CallToolResult, error) {
	namespace := ""
	rd, err := c.ResolveResourceKind(nCtx, &namespace, resourceType)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("resolve resource type failed: %v", err)), nil
	}

	resolvedType := rd.TypedSpec().Type
	resolvedNs := rd.TypedSpec().DefaultNamespace

	if resourceID != "" {
		r, err := c.COSI.Get(nCtx,
			resource.NewMetadata(resolvedNs, resolvedType, resourceID, resource.VersionUndefined),
		)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get resource failed: %v", err)), nil
		}
		return jsonResult(resourceToMap(r))
	}

	items, err := c.COSI.List(nCtx,
		resource.NewMetadata(resolvedNs, resolvedType, "", resource.VersionUndefined),
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list resources failed: %v", err)), nil
	}

	var results []map[string]any
	for _, r := range items.Items {
		results = append(results, resourceToMap(r))
	}
	return jsonResult(results)
}

func handleGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)

	return getResource(c, nCtx, resourceType, resourceID)
}

// resourceHandler returns an MCP handler for a fixed COSI resource type.
func resourceHandler(resourceType string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, nCtx, err := setupClient(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer c.Close()

		args := req.GetArguments()
		resourceID, _ := args["id"].(string)

		return getResource(c, nCtx, resourceType, resourceID)
	}
}

// resourceToMap converts a COSI resource to a JSON-friendly map.
func resourceToMap(r resource.Resource) map[string]any {
	m := map[string]any{
		"metadata": map[string]any{
			"namespace": r.Metadata().Namespace(),
			"type":      r.Metadata().Type(),
			"id":        r.Metadata().ID(),
			"version":   r.Metadata().Version().String(),
			"phase":     r.Metadata().Phase().String(),
		},
	}
	// Marshal spec via YAML then decode to Go map for JSON output
	if b, err := yaml.Marshal(r.Spec()); err == nil {
		var spec any
		if yaml.Unmarshal(b, &spec) == nil {
			m["spec"] = spec
		}
	}
	return m
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

	resp, err := c.ApplyConfiguration(nCtx, &machine.ApplyConfigurationRequest{
		Data:   []byte(config),
		Mode:   parseApplyMode(mode),
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

	// system_labels_to_wipe: selective partition wipe by label
	if labels, ok := args["system_labels_to_wipe"].(string); ok && labels != "" {
		for label := range strings.SplitSeq(labels, ",") {
			resetReq.SystemPartitionsToWipe = append(resetReq.SystemPartitionsToWipe,
				&machine.ResetPartitionSpec{Label: strings.TrimSpace(label)})
		}
	}

	if err := c.ResetGeneric(nCtx, resetReq); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reset failed: %v", err)), nil
	}
	return mcp.NewToolResultText("Reset initiated."), nil
}

// versionTagRE matches the leading "vMAJOR.MINOR" of a Talos version tag.
var versionTagRE = regexp.MustCompile(`^v(\d+)\.(\d+)`)

// serverSupportsLifecycleService probes the target node for its Talos version
// and reports whether it is v1.13 or newer (i.e. exposes LifecycleService).
// Returns (supported, tag, error). On parse failure the tag is returned for diagnostics.
func serverSupportsLifecycleService(c *client.Client, nCtx context.Context) (bool, string, error) {
	resp, err := c.Version(nCtx)
	if err != nil {
		return false, "", err
	}
	msgs := resp.GetMessages()
	if len(msgs) == 0 {
		return false, "", fmt.Errorf("no version messages returned")
	}
	tag := msgs[0].GetVersion().GetTag()
	m := versionTagRE.FindStringSubmatch(tag)
	if m == nil {
		return false, tag, fmt.Errorf("could not parse version tag %q", tag)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major > 1 || (major == 1 && minor >= 13), tag, nil
}

func handleUpgrade(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	image, _ := args["image"].(string)
	if image == "" {
		return mcp.NewToolResultError("image is required"), nil
	}

	autoReboot := true
	if v, ok := args["auto_reboot"].(bool); ok {
		autoReboot = v
	}
	rebootMode, _ := args["reboot_mode"].(string)

	useLifecycle, tag, err := serverSupportsLifecycleService(c, nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("version probe failed: %v", err)), nil
	}

	// If auto_reboot is false, the operator wants install-only — skip the full
	// drain/wait/uncordon orchestration and just run the install step.
	if !autoReboot {
		if useLifecycle {
			return upgradeViaLifecycleService(nCtx, c, image, tag, false, rebootMode)
		}
		return upgradeViaLegacyAPI(nCtx, c, args, image, tag, false)
	}

	return drainUpgradeReboot(ctx, nCtx, c, args, image, tag, useLifecycle, rebootMode)
}

// drainUpgradeReboot is the full talosctl-equivalent upgrade flow for
// auto_reboot=true: cordon -> drain (via the kubectl drain library) -> install
// -> reboot -> wait for the Talos node to come back -> wait for K8s Ready ->
// uncordon. K8s steps are skipped gracefully if the target isn't a Kubernetes
// node (e.g. fresh cluster, or non-K8s Talos use case).
//
// baseCtx is the un-noded context (uses the talosconfig endpoints, i.e. CPs)
// — needed for cluster-level RPCs like Kubeconfig that workers cannot serve.
// nCtx is the node-targeted context for per-node operations (Reboot, Version,
// COSI resource reads on the target).
//
// Uncordon runs in a deferred closure so the node is never left cordoned by a
// failure earlier in the flow (drain timeout, install error, reboot stuck,
// etc.). If the explicit uncordon at the end succeeds, the defer is a no-op
// (RunCordonOrUncordon is idempotent).
func drainUpgradeReboot(baseCtx, nCtx context.Context, c *client.Client, args map[string]any, image, tag string, useLifecycle bool, rebootMode string) (finalRes *mcp.CallToolResult, _ error) {
	stages := []string{}
	addStage := func(msg string) { stages = append(stages, msg) }
	payload := map[string]any{}
	// emit serializes the payload via statusAwareResult so failures (any
	// non-"ok" status, or any "_error" field) are flagged as IsError=true.
	emit := func() {
		payload["stages"] = stages
		finalRes, _ = statusAwareResult(payload)
	}

	// Phase 0 — discover K8s. Tolerant of failures: upgrade still proceeds.
	// k8s.Nodename is a per-node COSI resource (use nCtx); kubeconfig is
	// cluster-wide and only CPs serve it (use baseCtx, which routes through
	// the talosconfig endpoints).
	nodeName, _ := getKubernetesNodeName(nCtx, c)
	var clientset *kubernetes.Clientset
	if nodeName != "" {
		cs, csErr := newK8sClientset(baseCtx, c)
		if csErr != nil {
			addStage(fmt.Sprintf("k8s skipped: %v", csErr))
		} else {
			clientset = cs
			payload["k8s_node_name"] = nodeName
		}
	} else {
		addStage("k8s skipped: node not registered as a Kubernetes member")
	}

	// Defer the uncordon. didCordon controls whether the defer attempts it
	// (no-op if we never cordoned). Set to false after a successful explicit
	// uncordon so the defer doesn't double-call (harmless either way, but
	// avoids spurious "node already uncordoned" log noise).
	var didCordon bool
	defer func() {
		if didCordon && clientset != nil {
			if uErr := uncordonNode(baseCtx, clientset, nodeName); uErr != nil {
				if _, already := payload["uncordoned"]; !already {
					payload["uncordon_error"] = uErr.Error()
				}
			} else if _, already := payload["uncordoned"]; !already {
				payload["uncordoned"] = true
				addStage(fmt.Sprintf("node %s uncordoned (deferred)", nodeName))
			}
			emit()
		}
	}()

	// Phase 1 — cordon (separate from drain so the deferred uncordon above
	// covers any failure after this point).
	if clientset != nil {
		if err := cordonNode(baseCtx, clientset, nodeName); err != nil {
			payload["status"] = "cordon_failed"
			payload["error"] = err.Error()
			emit()
			return finalRes, nil
		}
		didCordon = true
		addStage(fmt.Sprintf("cordoned node %s", nodeName))

		// Phase 2 — drain. PDB-blocked pods retry internally; we hit
		// defaultDrainTimeout if a pod can't be evicted (e.g. PDB with
		// maxUnavailable=0). Defer-uncordon will recover.
		if err := drainNodePods(baseCtx, clientset, nodeName, defaultDrainTimeout, addStage); err != nil {
			payload["status"] = "drain_failed"
			payload["error"] = err.Error()
			emit()
			return finalRes, nil
		}
		addStage(fmt.Sprintf("node %s drained", nodeName))
	}

	// Phase 3 — install (and reboot, since auto_reboot=true got us here).
	var installRes *mcp.CallToolResult
	var installErr error
	if useLifecycle {
		installRes, installErr = upgradeViaLifecycleService(nCtx, c, image, tag, true, rebootMode)
	} else {
		installRes, installErr = upgradeViaLegacyAPI(nCtx, c, args, image, tag, true)
	}
	if installErr != nil {
		// install returned an error result already. Re-emit it (defer will uncordon).
		return installRes, nil
	}
	if installRes == nil {
		payload["status"] = "internal_error"
		payload["error"] = "install returned no result"
		emit()
		return finalRes, nil
	}

	// Merge install payload into our running payload (preserve our stages/k8s_node_name).
	for k, v := range extractJSONResult(installRes) {
		payload[k] = v
	}
	if status, _ := payload["status"].(string); status != "ok" {
		// install reported failure (e.g. installed_no_reboot). Bail; defer uncordons.
		emit()
		return finalRes, nil
	}

	// Phase 4 — wait for Talos to come back.
	addStage("waiting for talos to reboot")
	if err := waitForTalosReboot(nCtx, c, 10*time.Minute); err != nil {
		payload["wait_error"] = err.Error()
		payload["status"] = "rebooted_no_wait"
		emit()
		return finalRes, nil
	}
	addStage("talos reachable again")
	payload["talos_back"] = true

	// Phase 5 — wait for K8s Ready (only if we have a clientset).
	if clientset != nil {
		if err := waitNodeReady(baseCtx, clientset, nodeName, defaultNodeReadyTimeout); err != nil {
			payload["k8s_ready_error"] = err.Error()
		} else {
			addStage(fmt.Sprintf("node %s is Ready", nodeName))
			payload["k8s_ready"] = true
		}
		// Phase 6 — explicit uncordon. Mark didCordon=false so the defer skips.
		if err := uncordonNode(baseCtx, clientset, nodeName); err != nil {
			payload["uncordon_error"] = err.Error()
		} else {
			addStage(fmt.Sprintf("node %s uncordoned", nodeName))
			payload["uncordoned"] = true
			didCordon = false
		}
	}

	emit()
	return finalRes, nil
}

// extractJSONResult unmarshals the JSON-encoded text in an MCP tool result back
// into a map. Used to enrich the install result with drain/wait/uncordon
// outcomes before re-emitting it.
func extractJSONResult(res *mcp.CallToolResult) map[string]any {
	out := map[string]any{}
	if res == nil {
		return out
	}
	for _, item := range res.Content {
		tc, ok := item.(mcp.TextContent)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(tc.Text), &out); err == nil {
			return out
		}
	}
	return out
}

// rebootModeOpts maps the user-facing reboot_mode string onto client.RebootMode options.
// Empty/unknown values yield a default reboot (no special mode).
func rebootModeOpts(mode string) []client.RebootMode {
	switch strings.ToLower(mode) {
	case "powercycle":
		return []client.RebootMode{client.WithPowerCycle}
	case "force":
		return []client.RebootMode{client.WithForce}
	default:
		return nil
	}
}

// upgradeViaLifecycleService is the v1.13+ path. Pulls the image first
// (LifecycleService.Upgrade no longer pulls internally — that was split out
// into ImageService.Pull in v1.13), then streams the upgrade progress and
// aggregates everything into a single response. If autoReboot is true,
// issues an explicit Reboot RPC after a successful install (matching
// talosctl's behaviour where install + reboot are orchestrated client-side).
// The legacy force/stage options are not part of the new API and are ignored.
func upgradeViaLifecycleService(nCtx context.Context, c *client.Client, image, tag string, autoReboot bool, rebootMode string) (*mcp.CallToolResult, error) {
	// Talos requires Containerd to be set on both Pull and Upgrade requests
	// (zero-value enum is NS_UNKNOWN which the server rejects). Installer
	// images live in the system containerd namespace; Driver defaults to
	// CONTAINERD which is correct for the system instance.
	sysNs := &common.ContainerdInstance{Namespace: common.ContainerdNamespace_NS_SYSTEM}

	// Phase 1: pull the image into the node's system containerd.
	pullStream, err := c.ImageClient.Pull(nCtx, &machine.ImageServicePullRequest{
		Containerd: sysNs,
		ImageRef:   image,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("image pull failed (LifecycleService prep, server %s): %v", tag, err)), nil
	}
	pulledName := image
	for {
		resp, err := pullStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return mcp.NewToolResultError(fmt.Sprintf("image pull stream failed (server %s): %v", tag, err)), nil
		}
		if name := resp.GetName(); name != "" {
			pulledName = name
		}
	}

	// Phase 2: run the upgrade with the pre-pulled image reference.
	stream, err := c.LifecycleClient.Upgrade(nCtx, &machine.LifecycleServiceUpgradeRequest{
		Containerd: sysNs,
		Source:     &machine.InstallArtifactsSource{ImageName: pulledName},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("upgrade failed (LifecycleService, server %s, image %q was pulled OK): %v", tag, pulledName, err)), nil
	}

	var (
		messages []string
		exitCode int32
	)
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return mcp.NewToolResultError(fmt.Sprintf("upgrade stream failed (server %s): %v\nprogress so far:\n%s", tag, err, strings.Join(messages, "\n"))), nil
		}
		prog := resp.GetProgress()
		if msg := prog.GetMessage(); msg != "" {
			messages = append(messages, msg)
		}
		if code := prog.GetExitCode(); code != 0 {
			exitCode = code
		}
	}

	out := map[string]any{
		"api":          "lifecycle",
		"server_tag":   tag,
		"pulled_image": pulledName,
		"messages":     messages,
		"rebooted":     false,
	}
	if exitCode != 0 {
		out["status"] = "failed"
		out["exit_code"] = exitCode
		return statusAwareResult(out)
	}
	out["status"] = "ok"

	// Phase 3 (optional): activate by rebooting. LifecycleService.Upgrade is
	// install-only — it writes Talos to the alternate A/B partition and
	// updates META, but doesn't activate. talosctl issues an explicit Reboot
	// here; we mirror that by default (auto_reboot=true).
	if autoReboot {
		if err := c.Reboot(nCtx, rebootModeOpts(rebootMode)...); err != nil {
			out["status"] = "installed_no_reboot"
			out["reboot_error"] = err.Error()
			return statusAwareResult(out)
		}
		out["rebooted"] = true
	}
	return statusAwareResult(out)
}

// upgradeViaLegacyAPI is the pre-v1.13 path using MachineService.Upgrade.
// Honors force/stage/reboot_mode for compatibility with older clusters.
// autoReboot=false maps to stage=true (legacy primitive for "install but
// don't reboot"); an explicit stage flag from args wins if set.
func upgradeViaLegacyAPI(nCtx context.Context, c *client.Client, args map[string]any, image, tag string, autoReboot bool) (*mcp.CallToolResult, error) {
	force, _ := args["force"].(bool)
	stage, stageSet := args["stage"].(bool)
	if !stageSet && !autoReboot {
		// auto_reboot=false on a legacy server: stage the upgrade so the
		// server doesn't reboot at the end. The operator activates later
		// via talos_reboot.
		stage = true
	}

	upgradeOpts := []client.UpgradeOption{
		client.WithUpgradeImage(image),
		client.WithUpgradeForce(force),
		client.WithUpgradeStage(stage),
	}
	if rebootMode, ok := args["reboot_mode"].(string); ok && strings.ToLower(rebootMode) == "powercycle" {
		upgradeOpts = append(upgradeOpts, client.WithUpgradeRebootMode(machine.UpgradeRequest_POWERCYCLE))
	}

	resp, err := c.UpgradeWithOptions(nCtx, upgradeOpts...) //nolint:staticcheck // legacy path, intentionally kept for <v1.13 servers
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("upgrade failed (legacy API, server %s): %v", tag, err)), nil
	}
	return statusAwareResult(map[string]any{
		"status":     "ok",
		"api":        "legacy",
		"server_tag": tag,
		"rebooted":   !stage, // legacy upgrade reboots unless staged
		"response":   resp,
	})
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

	filter, _ := args["filter"].(string)

	stream, err := c.Logs(nCtx, namespace, driver, service, false, tailLines)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("logs failed: %v", err)), nil
	}

	output, err := collectStream(stream, filter)
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
	filter, _ := args["filter"].(string)

	stream, err := c.Dmesg(nCtx, false, tail)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("dmesg failed: %v", err)), nil
	}

	output, err := collectStream(stream, filter)
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

	var procs []map[string]any
	for _, msg := range resp.GetMessages() {
		for _, p := range msg.GetProcesses() {
			procs = append(procs, map[string]any{
				"pid":             p.GetPid(),
				"ppid":            p.GetPpid(),
				"state":           p.GetState(),
				"command":         p.GetCommand(),
				"threads":         p.GetThreads(),
				"cpu_time":        p.GetCpuTime(),
				"resident_memory": p.GetResidentMemory(),
				"virtual_memory":  p.GetVirtualMemory(),
			})
		}
	}

	args := req.GetArguments()
	if sortBy, ok := args["sort"].(string); ok {
		sort.Slice(procs, func(i, j int) bool {
			switch strings.ToLower(sortBy) {
			case "cpu":
				ci, _ := procs[i]["cpu_time"].(float64)
				cj, _ := procs[j]["cpu_time"].(float64)
				return ci > cj
			default: // rss
				ri, _ := procs[i]["resident_memory"].(uint64)
				rj, _ := procs[j]["resident_memory"].(uint64)
				return ri > rj
			}
		})
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

func handlePatch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	patchYAML, _ := args["patch"].(string)
	mode, _ := args["mode"].(string)
	dryRun, _ := args["dry_run"].(bool)

	// 1. Get current machine config from node via COSI
	namespace := ""
	rd, err := c.ResolveResourceKind(nCtx, &namespace, "mc")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("resolve mc type failed: %v", err)), nil
	}

	items, err := c.COSI.List(nCtx,
		resource.NewMetadata(rd.TypedSpec().DefaultNamespace, rd.TypedSpec().Type, "", resource.VersionUndefined),
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get machine config failed: %v", err)), nil
	}
	if len(items.Items) == 0 {
		return mcp.NewToolResultError("no machine config found on node"), nil
	}

	// The machine config spec comes as a YAML string via the COSI protobuf layer.
	// Marshal it, then if the result is a quoted YAML string, unwrap it.
	configYAMLBytes, err := yaml.Marshal(items.Items[0].Spec())
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal config failed: %v", err)), nil
	}
	// yaml.Marshal wraps a plain string in quotes — try to unwrap
	var asString string
	if yaml.Unmarshal(configYAMLBytes, &asString) == nil && strings.HasPrefix(asString, "version:") {
		configYAMLBytes = []byte(asString)
	}

	// 2. Load and apply the patch
	patch, err := configpatcher.LoadPatch([]byte(patchYAML))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid patch: %v", err)), nil
	}

	output, err := configpatcher.Apply(configpatcher.WithBytes(configYAMLBytes), []configpatcher.Patch{patch})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("apply patch failed: %v", err)), nil
	}

	patchedBytes, err := output.Bytes()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encode patched config failed: %v", err)), nil
	}

	// 3. Apply the patched config
	resp, err := c.ApplyConfiguration(nCtx, &machine.ApplyConfigurationRequest{
		Data:   patchedBytes,
		Mode:   parseApplyMode(mode),
		DryRun: dryRun,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("apply patched config failed: %v", err)), nil
	}
	return jsonResult(resp)
}

func handleKubeconfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	data, err := c.Kubeconfig(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("kubeconfig failed: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func handleEtcdRemoveMember(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	memberID, _ := args["member_id"].(float64)

	if err := c.EtcdRemoveMemberByID(nCtx, &machine.EtcdRemoveMemberByIDRequest{
		MemberId: uint64(memberID),
	}); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("etcd remove member failed: %v", err)), nil
	}
	return mcp.NewToolResultText("Etcd member removed."), nil
}

func handleEtcdForfeitLeadership(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.EtcdForfeitLeadership(nCtx, &machine.EtcdForfeitLeadershipRequest{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("etcd forfeit leadership failed: %v", err)), nil
	}
	return jsonResult(resp)
}

func handleEtcdLeave(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	if err := c.EtcdLeaveCluster(nCtx, &machine.EtcdLeaveClusterRequest{}); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("etcd leave failed: %v", err)), nil
	}
	return mcp.NewToolResultText("Node left etcd cluster."), nil
}

func handleEtcdAlarm(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	resp, err := c.EtcdAlarmList(nCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("etcd alarm list failed: %v", err)), nil
	}
	return jsonResult(resp)
}

// --- Additional operations ---

func handleRollback(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	if err := c.Rollback(nCtx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rollback failed: %v", err)), nil
	}
	return mcp.NewToolResultText("Rollback initiated."), nil
}

func handleServiceRestart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	service, _ := args["service"].(string)

	resp, err := c.ServiceRestart(nCtx, service)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("service restart failed: %v", err)), nil
	}
	return jsonResult(resp)
}

func handleImageList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	ns := common.ContainerdNamespace_NS_CRI
	if namespace, ok := args["namespace"].(string); ok && namespace == "system" {
		ns = common.ContainerdNamespace_NS_SYSTEM
	}

	stream, err := c.ImageList(nCtx, ns)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("image list failed: %v", err)), nil
	}

	var images []map[string]any
	for {
		img, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return mcp.NewToolResultError(fmt.Sprintf("image list stream failed: %v", err)), nil
		}
		images = append(images, map[string]any{
			"name":    img.GetName(),
			"digest":  img.GetDigest(),
			"size":    img.GetSize(),
			"created": img.GetCreatedAt().AsTime().Format(time.RFC3339),
		})
	}
	return jsonResult(images)
}

func handleStats(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	resp, err := c.Stats(nCtx, namespace, driver)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stats failed: %v", err)), nil
	}
	return jsonResult(resp)
}

func handleLS(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	path, _ := args["path"].(string)
	if path == "" {
		path = "/"
	}
	recurse, _ := args["recurse"].(bool)
	pattern, _ := args["pattern"].(string)

	recursionDepth := int32(0)
	if recurse {
		recursionDepth = 1
	}

	stream, err := c.LS(nCtx, &machine.ListRequest{
		Root:           path,
		RecursionDepth: recursionDepth,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}

	var files []map[string]any
	for {
		info, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return mcp.NewToolResultError(fmt.Sprintf("list stream failed: %v", err)), nil
		}
		if info.GetError() != "" {
			continue
		}
		name := info.GetRelativeName()
		if pattern != "" && !strings.Contains(name, pattern) {
			continue
		}
		files = append(files, map[string]any{
			"name":     name,
			"size":     info.GetSize(),
			"mode":     fmt.Sprintf("%o", info.GetMode()),
			"modified": time.Unix(info.GetModified(), 0).Format(time.RFC3339),
			"is_dir":   info.GetIsDir(),
		})
	}
	return jsonResult(files)
}

func handleRead(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	path, _ := args["path"].(string)

	reader, err := c.Read(nCtx, path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read failed: %v", err)), nil
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read stream failed: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func handleDiskUsage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	path, _ := args["path"].(string)
	if path == "" {
		path = "/"
	}

	stream, err := c.DiskUsage(nCtx, &machine.DiskUsageRequest{
		RecursionDepth: 1,
		Paths:          []string{path},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("disk usage failed: %v", err)), nil
	}

	var entries []map[string]any
	for {
		info, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return mcp.NewToolResultError(fmt.Sprintf("disk usage stream failed: %v", err)), nil
		}
		if info.GetError() != "" {
			continue
		}
		entries = append(entries, map[string]any{
			"name":          info.GetName(),
			"relative_name": info.GetRelativeName(),
			"size":          info.GetSize(),
		})
	}
	return jsonResult(entries)
}

func handleTime(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	var resp *timeapi.TimeResponse
	args := req.GetArguments()
	if server, ok := args["server"].(string); ok && server != "" {
		resp, err = c.TimeCheck(nCtx, server)
	} else {
		resp, err = c.Time(nCtx)
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("time failed: %v", err)), nil
	}
	return jsonResult(resp)
}

func handleWipe(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, nCtx, err := setupClient(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer c.Close()

	args := req.GetArguments()
	device, _ := args["device"].(string)

	if err := c.BlockDeviceWipe(nCtx, &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{Device: device},
		},
	}); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("wipe failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Device %s wiped.", device)), nil
}
