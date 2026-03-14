---
name: talos-image
description: Build custom Talos Linux images with extensions using the local imager
allowed-tools: ["Read", "Write", "Bash", "Grep"]
argument-hint: "[output-type] [--extensions ext1,ext2]"
---

Build a custom Talos Linux image using the local imager container. Follow these steps:

1. **Gather requirements** — Ask the user for:
   - Output type: `iso`, `disk-image`, `installer`, `metal` (default: `iso`)
   - Target Talos version (default: v1.12)
   - Extensions to include (e.g., `siderolabs/iscsi-tools`, `siderolabs/qemu-guest-agent`)
   - Platform/arch if not default (e.g., `metal`, `aws`, `azure`)
   - SecureBoot: yes/no
   - Any overlay to apply

2. **Build the imager command:**
   ```bash
   docker run --rm -t -v /dev:/dev --privileged \
     ghcr.io/siderolabs/imager:v1.12.0 \
     <output-type> \
     --system-extension-image ghcr.io/siderolabs/<extension>:latest \
     [--extra-kernel-arg ...] \
     [--overlay-image ...] \
     [--output /out]
   ```

3. **Show the command** to the user for review before executing.

4. **Execute** the imager command via Bash.

5. **Report** the output file location and any relevant details (size, SHA).

**Extension reference** (common extensions):
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
- The imager pulls extension images automatically
- For SecureBoot, add `--overlay-name secureboot`
- Refer to the Talos skill's boot-assets reference for detailed profiles and options
