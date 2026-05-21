# Boot Assets & Imager Reference

Docs: https://docs.siderolabs.com/talos/v1.13/talos-guides/install/boot-assets/

## Local Imager

The imager builds custom Talos images locally via Docker. As of v1.13 the imager runs rootless — `--privileged` and `-v /dev:/dev` are only needed for **bootable-media** profiles (`iso`, `metal`, `disk-image`, cloud targets) that need loop devices. The `installer` profile does not need them. Always bind-mount a host directory to `/out` (the in-container output path).

Bootable media (iso/metal/disk-image/cloud):
```bash
mkdir -p _out
docker run --rm -t --privileged \
  -v /dev:/dev \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.13.2 \
  <output-type> [options]
```

Installer (Docker image tar):
```bash
mkdir -p _out
docker run --rm -t \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.13.2 \
  installer [options]
```

### Output Types
- `iso` — bootable ISO image
- `metal` — raw disk image for bare metal
- `disk-image` — generic disk image
- `installer` — installer container image (Docker tar)
- `aws` — AMI-compatible image
- `azure` — VHD for Azure
- `gcp` — GCE image
- `vmware` — VMDK for VMware
- `oracle` — OCI image
- `digital-ocean` — DO custom image
- `hcloud` — Hetzner Cloud image
- `vultr` — Vultr image
- `nocloud` — cloud-init compatible
- `openstack` — OpenStack image

### Common Options
```
--system-extension-image <image>   Add a system extension (use a versioned tag)
--extra-kernel-arg <arg>           Add kernel command-line arg
--overlay-image <image>            Apply an overlay (e.g., SBC support)
--overlay-name <name>              Overlay name within the image
--meta <key>=<value>               Set META partition value
--base-installer-image <image>     Custom base installer
```

The imager writes its output to `/out` inside the container by default — bind a host path to `/out` rather than passing an `--output` flag.

### Extension Tag Matching

System extension images are tagged per Talos release. **Never use `:latest`** — there is no `:latest` tag and a copy-pasted invocation will fail to pull. Look up the matching tag for your Talos version at:
- https://github.com/siderolabs/extensions/releases/tag/v1.13.2
- or the image factory's schematic UI at https://factory.talos.dev/

### Reproducible Images (v1.13)

As of v1.13 disk images are reproducible — building the same Talos version multiple times yields byte-identical output. Verify via SHA. Note: VHD and VMDK (Azure, VMware) images are not currently reproducible due to limitations in the underlying image creation tools; use raw images for verification and convert afterward.

### Examples

**ISO with extensions:**
```bash
docker run --rm -t --privileged \
  -v /dev:/dev \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.13.2 iso \
  --system-extension-image ghcr.io/siderolabs/iscsi-tools:<tag-for-v1.13.2> \
  --system-extension-image ghcr.io/siderolabs/qemu-guest-agent:<tag-for-v1.13.2>
```

**Metal image for Raspberry Pi:**
```bash
docker run --rm -t --privileged \
  -v /dev:/dev \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.13.2 metal \
  --overlay-image ghcr.io/siderolabs/sbc-raspberrypi:<tag> \
  --overlay-name rpi_generic
```

**Installer with custom extensions:**
```bash
docker run --rm -t \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.13.2 installer \
  --system-extension-image ghcr.io/siderolabs/nvidia-container-toolkit:<tag-for-v1.13.2>
```

After building the installer, load it into the local Docker daemon, tag it for your registry, and push so the cluster nodes can pull it:
```bash
docker load -i _out/installer-amd64.tar
docker tag  ghcr.io/siderolabs/installer:v1.13.2 <registry>/<repo>:v1.13.2-custom
docker push <registry>/<repo>:v1.13.2-custom
```
Then reference `<registry>/<repo>:v1.13.2-custom` in `mcp__talos__talos_upgrade` or in `.machine.install.image` at install time.

**SecureBoot ISO:**
```bash
docker run --rm -t --privileged \
  -v /dev:/dev \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.13.2 iso \
  --overlay-name secureboot
```

## Common System Extensions

### Core (Official)
| Extension | Image | Purpose |
|---|---|---|
| iscsi-tools | `ghcr.io/siderolabs/iscsi-tools` | iSCSI initiator |
| qemu-guest-agent | `ghcr.io/siderolabs/qemu-guest-agent` | QEMU/KVM agent |
| intel-ucode | `ghcr.io/siderolabs/intel-ucode` | Intel microcode |
| amd-ucode | `ghcr.io/siderolabs/amd-ucode` | AMD microcode |

### Extra
| Extension | Image | Purpose |
|---|---|---|
| nvidia-container-toolkit | `ghcr.io/siderolabs/nvidia-container-toolkit` | NVIDIA GPU (v1.13 uses CDI; see gpu-operator notes) |
| tailscale | `ghcr.io/siderolabs/tailscale` | Tailscale VPN |
| util-linux-tools | `ghcr.io/siderolabs/util-linux-tools` | Linux utilities |
| drbd | `ghcr.io/siderolabs/drbd` | DRBD replication |
| gasket-driver | `ghcr.io/siderolabs/gasket-driver` | Google Coral TPU |
| thunderbolt | `ghcr.io/siderolabs/thunderbolt` | Thunderbolt support |
| usb-modem-drivers | `ghcr.io/siderolabs/usb-modem-drivers` | USB modem support |

### SBC Overlays
| Board | Overlay Image |
|---|---|
| Raspberry Pi | `ghcr.io/siderolabs/sbc-raspberrypi` |
| Jetson Nano | `ghcr.io/siderolabs/sbc-jetson` |
| Orange Pi 5 | `ghcr.io/siderolabs/sbc-rockchip` |
| Rock 5B | `ghcr.io/siderolabs/sbc-rockchip` |
| Turing RK1 | `ghcr.io/siderolabs/sbc-turingrk1` |
| Banana Pi M64 | `ghcr.io/siderolabs/sbc-allwinner` |
| Pine64 | `ghcr.io/siderolabs/sbc-allwinner` |

## Image Factory (Online)

Alternative to local imager — hosted service at https://factory.talos.dev/

Uses **schematics** (content-addressable configurations) to define image contents. Useful for PXE boot and automated provisioning but not covered by the `/talos-image` command (which uses local imager).
