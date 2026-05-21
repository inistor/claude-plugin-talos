---
name: talos-image
description: Build custom Talos Linux images with extensions using the local imager
allowed-tools: ["Read", "Write", "Bash", "Grep", "mcp__talos__*", "mcp__plugin_talos_talos__*"]
argument-hint: "[output-type] [--extensions ext1,ext2]"
---

Build a custom Talos Linux image using the local imager container. Follow these steps:

1. **Resolve inputs** — Treat any arguments as defaults; prompt only for what's missing:
   - Output type: `iso`, `disk-image`, `installer`, `metal`, or a cloud target (default: `iso`)
   - Target Talos version (default: v1.13.2 — bump when the plugin tracks a newer release)
   - Extensions to include (versioned image refs, e.g. `ghcr.io/siderolabs/iscsi-tools:v0.1.4`). **Do not use `:latest`** — Siderolabs extensions are tagged to match each Talos release. Look the right tag up at https://github.com/siderolabs/extensions/releases or the image factory.
   - Platform/arch if relevant (e.g., `aws`, `azure`, `metal-rpi_generic`)
   - SecureBoot: yes/no
   - Any overlay to apply
   - For `installer`: the destination registry/repo (so the resulting image can be pushed and referenced later by `talos_upgrade`)

2. **Build the imager command** — Output goes to `/out` inside the container, so always bind-mount a host directory there. `--privileged` and `-v /dev:/dev` are only needed for **bootable-media** profiles (`iso`, `metal`, `disk-image`, cloud targets) which use loop devices; the `installer` profile does not need them.

   Bootable media (iso/metal/disk-image/cloud):
   ```bash
   mkdir -p _out
   docker run --rm -t --privileged \
     -v /dev:/dev \
     -v "$PWD/_out:/out" \
     ghcr.io/siderolabs/imager:v1.13.2 \
     <output-type> \
     --system-extension-image ghcr.io/siderolabs/<ext>:<tag> \
     [--extra-kernel-arg ...] \
     [--overlay-image ... --overlay-name ...]
   ```

   Installer (Docker image tar) — v1.13+ imager runs rootless, no `--privileged` or `-v /dev:/dev` needed:
   ```bash
   mkdir -p _out
   docker run --rm -t \
     -v "$PWD/_out:/out" \
     ghcr.io/siderolabs/imager:v1.13.2 \
     installer \
     --system-extension-image ghcr.io/siderolabs/<ext>:<tag>
   ```

3. **Show the command** to the user for review before executing.

4. **Execute** the imager command via Bash and confirm the output file appears under `_out/`.

5. **Installer profile only — load, tag, push:** the tar's filename matches the build arch (`installer-amd64.tar`, `installer-arm64.tar`, …). Substitute `<arch>` for the build target.
   ```bash
   ARCH=amd64   # or arm64, depending on what you built
   docker load -i "_out/installer-${ARCH}.tar"   # loads as ghcr.io/siderolabs/installer:vX.Y.Z
   docker tag  ghcr.io/siderolabs/installer:vX.Y.Z <registry>/<repo>:vX.Y.Z-custom
   docker push <registry>/<repo>:vX.Y.Z-custom
   ```
   Then the upgrade flow can reference `<registry>/<repo>:vX.Y.Z-custom` via `talos_upgrade` (or in `.machine.install.image` at install time).

6. **Report** the output file location and any relevant details (size, SHA, pushed image ref).

**Extension reference** (common — always tag-match to the Talos version, never `:latest`):
- `siderolabs/iscsi-tools` — iSCSI support
- `siderolabs/qemu-guest-agent` — QEMU/KVM guest agent
- `siderolabs/intel-ucode` — Intel microcode updates
- `siderolabs/amd-ucode` — AMD microcode updates
- `siderolabs/nvidia-container-toolkit` — NVIDIA GPU support
- `siderolabs/tailscale` — Tailscale VPN
- `siderolabs/util-linux-tools` — Additional Linux utilities
- `siderolabs/gasket-driver` — Google Coral TPU
- `siderolabs/drbd` — DRBD storage replication

**Important:**
- Always confirm the command with the user before running
- Docker must be available locally
- The imager pulls extension images automatically (network required)
- For SecureBoot, add `--overlay-name secureboot` (no `--privileged` needed for the installer profile)
- Refer to the Talos skill's boot-assets reference for detailed profiles and options
