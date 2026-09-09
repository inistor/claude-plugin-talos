---
name: talos-image
description: Build custom Talos Linux images with extensions using the local imager
allowed-tools: ["Read", "Write", "Bash", "Grep", "mcp__talos__*", "mcp__plugin_talos_talos__*"]
argument-hint: "[profile] [--extensions ext1,ext2]"
---

Build a custom Talos Linux image using the local imager container.

1. **Resolve inputs** — treat any arguments as defaults; prompt only for what is missing:
   - **Profile** (default: `iso`). Valid profiles are `iso`, `metal`, `metal-4k`, `metal-uki`, `installer`, their `secureboot-*` variants, and cloud targets (`aws`, `azure`, `gcp`, `nocloud`, `vmware`, `hcloud`, `openstack`, `oracle`, …). There is **no** `disk-image` profile.
   - **Talos version** (default: v1.14.0)
   - **Extensions** as versioned image refs. Never `:latest`.
   - **Arch/platform** if relevant (`--arch arm64`, SBC overlays)
   - **SecureBoot**: yes/no
   - For `installer`: the destination registry/repo, so the result can be pushed and referenced by `talos_upgrade`

2. **Resolve extension tags properly.** Only *kernel-module* extensions embed the Talos version in their tag (e.g. `gasket-driver:<sha>-v1.14.0`); userspace ones are versioned independently (`iscsi-tools:v0.2.0`, `tailscale:1.102.2`). Do not guess — look them up:

   ```bash
   crane export ghcr.io/siderolabs/extensions:v1.14.0 | tar x -O image-digests | grep <extension-name>
   ```

3. **Build the imager command.** Output goes to `/out` inside the container, so always bind-mount a host directory there. The imager runs rootless: `--privileged` and `-v /dev:/dev` are only needed for **bootable-media** profiles (`iso`, `metal`, cloud targets) that use loop devices. The `installer` profile does not need them.

   Bootable media:
   ```bash
   mkdir -p _out
   docker run --rm -t --privileged \
     -v /dev:/dev \
     -v "$PWD/_out:/out" \
     ghcr.io/siderolabs/imager:v1.14.0 \
     <profile> \
     --system-extension-image ghcr.io/siderolabs/<ext>:<tag> \
     [--extra-kernel-arg ...] \
     [--overlay-image ... --overlay-name ...]
   ```

   Installer (Docker image tar), no privileges needed:
   ```bash
   mkdir -p _out
   docker run --rm -t \
     -v "$PWD/_out:/out" \
     ghcr.io/siderolabs/imager:v1.14.0 \
     installer \
     --system-extension-image ghcr.io/siderolabs/<ext>:<tag>
   ```

   `ghcr.io/siderolabs/imager` is still published for v1.14 — it is only `ghcr.io/siderolabs/installer` that is gone.

4. **Show the command** to the user for review before executing.

5. **Execute** via Bash and confirm the output appears under `_out/`.

6. **Installer profile only — load, tag, push.** The tar's filename matches the build arch (`installer-amd64.tar`, `installer-arm64.tar`). The imager tags the loaded image `ghcr.io/siderolabs/installer-base:<version>` — note `installer-base`, not `installer`, and the tag carries **no** leading `v`. Capture the reference from `docker load` rather than reconstructing it:

   ```bash
   ARCH=amd64   # or arm64
   LOADED=$(docker load -i "_out/installer-${ARCH}.tar" | awk '/Loaded image:/ {print $NF}')
   echo "loaded: $LOADED"
   docker tag  "$LOADED" <registry>/<repo>:v1.14.0-custom
   docker push <registry>/<repo>:v1.14.0-custom
   ```

   Pass `<registry>/<repo>:v1.14.0-custom` to `talos_upgrade`, or set it as the installer image at install time.

7. **Report** the output location, size, SHA, and the pushed image reference.

**SecureBoot** is selected by *profile*, not by an overlay: use `secureboot-iso`, `secureboot-installer`, `secureboot-metal`, or `secureboot-metal-uki`, optionally with `--secureboot-include-well-known-certs`. `--overlay-name` is for **SBC overlays** (`rpi_generic`, `rock64`, …) and has nothing to do with SecureBoot.

**Common extensions** (tier in brackets):
- `iscsi-tools` [extra] — iSCSI support
- `qemu-guest-agent` [extra] — QEMU/KVM guest agent
- `intel-ucode` / `amd-ucode` [core] — microcode updates
- `nvidia-container-toolkit-lts` / `-production` [extra] — NVIDIA GPU support. The unsuffixed `nvidia-container-toolkit` no longer exists, and all NVIDIA extensions on a node must share the same branch.
- `tailscale` [extra] — Tailscale VPN
- `drbd` [extra] — DRBD storage replication
- `gasket-driver` [extra] — Google Coral TPU
- `util-linux-tools` [contrib] — additional Linux utilities

**Important:**
- Always confirm the command with the user before running
- Docker must be available locally, and the imager pulls extension images over the network
- An image built for one Talos version should not be reused across a minor upgrade — rebuild it
- See the skill's `references/images/boot-assets.md` for profiles and output formats, and `references/images/extensions.md` for the extension tag-lookup procedure
