# On-node image cache

Each Talos node runs **two containerd instances with separate image stores**, and every image tool
takes which one to act on:

- **`cri`** (the default) — kubelet's containerd, namespace `k8s.io`. Holds Kubernetes workload images. Old ones accumulate because kubelet garbage collection is watermark-driven and only fires under disk pressure. This is the usual prune target.
- **`system`** — Talos's own containerd. Holds the installer image and the system extensions.

**Do not remove the installer image for the running version from the `system` store.**
`talos_rollback` needs it; without it, an A/B rollback has nothing to boot back into.

## Tools

| Task | Tool |
|---|---|
| List what is cached | `talos_image_list` |
| Force a re-pull of one tag | `talos_image_remove` |
| Reclaim disk space | `talos_image_prune` |

`talos_image_prune` defaults to `dry_run=true`. Always read the dry-run output before letting it
delete anything.

## Why remove rarely frees space

containerd indexes each image under **three** references: the tag, the digest, and the raw content
ID. All three pin the same blob. Removing only the tag therefore frees nothing — the digest and
content ID still hold it, and `talos_disk_usage` will show no change, which reads as the removal
having silently failed.

`talos_image_prune` resolves all three, which is why prune is the tool for reclaiming space.
`talos_image_remove` is the tool for a different job: dropping one tag so the next pull fetches it
again.
