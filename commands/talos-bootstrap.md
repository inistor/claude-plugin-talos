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

3. **Point the MCP server at the new talosconfig** — the server reads `TALOSCONFIG` (or `~/.talos/config`) at startup and cannot be repointed mid-session. Either copy the generated `talosconfig` to `~/.talos/config`, or set `TALOSCONFIG` in the server's launch configuration and restart it. Confirm with `talos_config_info` before continuing.

4. **Apply configs** — For each node (which is in maintenance mode):
   - Read the appropriate file (`controlplane.yaml` or `worker.yaml`) and call `talos_apply_config` with the YAML content, the node IP, and `insecure: true` (maintenance mode has no TLS auth yet).
   - Wait for the node to reboot into its provisioned state before moving on.

5. **Bootstrap etcd** — Call `talos_bootstrap` on **ONE** control plane node only.

6. **Verify** — Call `talos_health` to check cluster health. Once healthy, confirm Kubernetes is up by listing nodes with `mcp__kubernetes-mcp-server__resources_list` (apiVersion `v1`, kind `Node`). Do **not** use `nodes_top` here: it needs metrics-server, which a freshly bootstrapped cluster does not have.

7. **Retrieve kubeconfig** — Call `talos_kubeconfig` to fetch the admin kubeconfig and save/merge it where the user prefers (commonly `~/.kube/config`).

**Important:**
- Never bootstrap more than one node
- Wait for each step to complete before proceeding
- Use `yq` or `jq` for parsing any YAML/JSON output, not grep
- If the cluster will use system extensions, decide on the installer image before step 2. Either pick the matching Image Factory schematic, or build a custom installer via `/talos-image` and push it. Then either (a) pass it to `talosctl gen config` via `--config-patch '@patch.yaml'` setting the install image, or (b) edit the generated `controlplane.yaml` / `worker.yaml` before step 4.
- **Installer image on v1.14**: `ghcr.io/siderolabs/installer` is no longer published. Use `factory.talos.dev/metal-installer/<schematic-id>:v1.14.0` — the empty/default schematic is `376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba`.
- **On v1.14, `.machine.install` is deprecated** in favour of `UnattendedInstallConfig`, and a config carrying both is rejected. `.machine.install` still works and remains the only way to set `disk`, `extraKernelArgs`, `legacyBIOSSupport` or `grubUseUKICmdline` — so pick one and stay with it for the whole config.
- Report progress at each step
