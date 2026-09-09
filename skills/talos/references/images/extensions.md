# System extensions

Extensions add drivers, tools and services to Talos. They are **baked into a boot asset**, never
installed at runtime, so changing the extension set means rebuilding an image or schematic and
reinstalling — see `references/images/boot-assets.md`. Upstream:
https://docs.siderolabs.com/talos/v1.14/build-and-extend-talos/custom-images-and-development/system-extensions

Check what a node actually carries with `talos_extensions`. Run it again after every upgrade: it
catches the most common mistake, upgrading an extension-using cluster to a stock image and silently
losing every extension on reboot.

## Finding the right tag

Extension tags are **not** uniform and most are not the Talos version. Do not guess, and do not
copy a tag out of a table that may have gone stale — look it up in the `extensions` manifest image
for the Talos release being built:

```bash
crane export ghcr.io/siderolabs/extensions:v1.14.0 | tar x -O image-digests | grep <extension-name>
```

That prints a fully pinned `name:tag@sha256:...` reference. Pass the digest-pinned form to
`--system-extension-image`. There is **no `:latest` tag**; a copied `:latest` fails to pull.

The generated table at https://github.com/siderolabs/extensions lists the tier and current version
of every extension.

## The two tag families

- **Kernel-module extensions** are built against one Talos version, so the tag embeds it: `thunderbolt:v1.14.0`, `zfs:2.4.4-v1.14.0`, `gasket-driver:5815ee3-v1.14.0`, `nonfree-kmod-nvidia-lts:580.178.04-v1.14.0`. These **must** match the Talos version being built; a mismatch fails to load at boot.
- **Userspace extensions** are versioned independently and carry the upstream software version: `iscsi-tools:v0.2.0`, `tailscale:1.102.2`, `qemu-guest-agent:11.1.0`, `util-linux-tools:2.42.2`. Never "bump" these to the Talos version.

## Tiers

From the `siderolabs/extensions` README: **core** — fully supported by Sidero Labs; **extra** —
best-effort support; **contrib** — community-supported. The tier affects expectations of support,
not how an extension is consumed.

## Durable rules

- **NVIDIA is split into `-lts` and `-production` variants.** The unsuffixed `nvidia-container-toolkit`, `nonfree-kmod-nvidia` and `nvidia-open-gpu-kernel-modules` names no longer exist. Every NVIDIA extension on a node must come from the **same branch and the same release** — the kernel module and the container toolkit are versioned together.
- **`multipath-tools` needs a config migration applied before upgrading to v1.14**, or `multipathd` hangs forever. See `references/v1.14-changes.md`.
- **btrfs user volumes in v1.14 require the `btrfs` extension.** A volume document referencing btrfs on a node without it stays unprovisioned.
- Extensions that ship a service and expect configuration are only served by `ExtensionServiceConfig` when their manifest declares `depends: - configuration: true`.
