# Upgrading Kubernetes

Kubernetes version upgrades run through `talosctl upgrade-k8s` in Bash. This stays **client-side in
v1.14** and has no MCP equivalent: `LifecycleService` covers the Talos OS install only, not the
Kubernetes control-plane rollout, which interleaves Talos and Kubernetes API calls. Do not try to
reproduce it by patching component images with `talos_patch`.

```bash
talosctl --nodes <control-plane-node> upgrade-k8s --to 1.37.0 --dry-run
talosctl --nodes <control-plane-node> upgrade-k8s --to 1.37.0
```

Use a `talosctl` binary matching the **target** Talos minor, not whatever happens to be installed.

## What it does

Runs its own precondition checks first — cluster health, node readiness, that every control-plane
node is reachable and running a Talos version that supports the target — then rolls the static pod
manifests for kube-apiserver, kube-controller-manager and kube-scheduler node by node, followed by
kubelet on every node and the bundled manifests (CoreDNS, kube-proxy, bootstrap RBAC).

**Always run `--dry-run` first.** It prints every change without applying anything, which is the
cheapest way to confirm the target version and see which components will move.

The command is **resumable**: an interrupted run can simply be re-run with the same `--to`, and it
picks up from the components not yet rolled.

## Supported versions

| | v1.14 | v1.13 |
|---|---|---|
| Default | 1.37.0 | 1.36.0 |
| Accepted by code | 1.32.0 – 1.37.99 | 1.31.0 – 1.36.99 |
| Documented and tested | 1.33 – 1.37 | 1.32 – 1.36 |

The documented floor is stricter than the code floor: Talos will not block 1.32, but it sits
outside the tested set. Upgrade one Kubernetes minor at a time, and finish the Talos upgrade before
starting the Kubernetes one — the Talos version determines which Kubernetes versions are accepted.
