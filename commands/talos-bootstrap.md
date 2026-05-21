---
name: talos-bootstrap
description: Bootstrap a new Talos Linux cluster from scratch
allowed-tools: ["Read", "Write", "Bash", "Grep", "mcp__talos__*", "mcp__plugin_talos_talos__*", "mcp__kubernetes-mcp-server__*"]
argument-hint: "[cluster-name] [endpoint]"
---

Bootstrap a new Talos Linux cluster. Follow these steps:

1. **Gather information** — Ask the user for:
   - Cluster name (use argument if provided)
   - Control plane endpoint (VIP or load balancer address, e.g. `https://10.0.0.10:6443`)
   - Node IPs and roles (control plane vs worker)
   - Any config patches (extensions, network config, etc.)

2. **Generate configuration** — Run `talosctl gen config <cluster-name> <endpoint>` via Bash. This produces `controlplane.yaml`, `worker.yaml`, and `talosconfig` in the current directory.

   Config generation is **client-side only** — it creates the cluster PKI, encryption keys, and bootstrap secrets, so there is no server-side MCP equivalent (analogous to how `talosctl upgrade-k8s` stays on the client in `/talos-upgrade`). Apply patches with `--config-patch @patch.yaml`, `--config-patch-control-plane`, or `--config-patch-worker`. After generation, the user can also edit the YAML files directly.

3. **Load talosconfig into the MCP session** — Run `base64 < talosconfig` via Bash and pass the output to `mcp__talos__talos_set_config`. All subsequent MCP calls will use this config.

4. **Apply configs** — For each node (which is in maintenance mode):
   - Read the appropriate file (`controlplane.yaml` or `worker.yaml`) and call `mcp__talos__talos_apply_config` with the YAML content, the node IP, and `insecure: true` (maintenance mode has no TLS auth yet).
   - Wait for the node to reboot into its provisioned state before moving on.

5. **Bootstrap etcd** — Call `mcp__talos__talos_bootstrap` on **ONE** control plane node only.

6. **Verify** — Call `mcp__talos__talos_health` to check cluster health. Once healthy, use `mcp__kubernetes-mcp-server__nodes_top` to confirm Kubernetes is up.

7. **Retrieve kubeconfig** — Call `mcp__talos__talos_kubeconfig` to fetch the admin kubeconfig and save/merge it where the user prefers (commonly `~/.kube/config`).

**Important:**
- Never bootstrap more than one node
- Wait for each step to complete before proceeding
- Use `yq` or `jq` for parsing any YAML/JSON output, not grep
- If the cluster will use system extensions, build a custom installer first via `/talos-image`. Then either (a) pass it to `talosctl gen config` in step 2 via `--config-patch '@patch.yaml'` where the patch sets `.machine.install.image`, or (b) edit the generated `controlplane.yaml` / `worker.yaml` to set `.machine.install.image` before step 4 (apply)
- Report progress at each step
