# Boot Assets & Imager Reference

Docs: https://docs.siderolabs.com/talos/v1.12/talos-guides/install/boot-assets/

## Local Imager

The imager builds custom Talos images locally via Docker:

```bash
docker run --rm -t -v /dev:/dev --privileged \
  ghcr.io/siderolabs/imager:v1.12.0 \
  <output-type> [options]
```

### Output Types
- `iso` — bootable ISO image
- `metal` — raw disk image for bare metal
- `disk-image` — generic disk image
- `installer` — installer container image
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
--system-extension-image <image>   Add a system extension
--extra-kernel-arg <arg>           Add kernel command-line arg
--overlay-image <image>            Apply an overlay (e.g., SBC support)
--overlay-name <name>              Overlay name within the image
--meta <key>=<value>               Set META partition value
--base-installer-image <image>     Custom base installer
--output <path>                    Output directory (default: current dir)
```

### Examples

**ISO with extensions:**
```bash
docker run --rm -t -v $(pwd)/_out:/out \
  ghcr.io/siderolabs/imager:v1.12.0 iso \
  --system-extension-image ghcr.io/siderolabs/iscsi-tools:v0.1.4 \
  --system-extension-image ghcr.io/siderolabs/qemu-guest-agent:v8.2.2 \
  --output /out
```

**Metal image for Raspberry Pi:**
```bash
docker run --rm -t -v $(pwd)/_out:/out --privileged \
  ghcr.io/siderolabs/imager:v1.12.0 metal \
  --overlay-image ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0 \
  --overlay-name rpi_generic \
  --output /out
```

**Installer with custom extensions:**
```bash
docker run --rm -t -v $(pwd)/_out:/out \
  ghcr.io/siderolabs/imager:v1.12.0 installer \
  --system-extension-image ghcr.io/siderolabs/nvidia-container-toolkit:535.129.03-v1.14.3 \
  --output /out
```

**SecureBoot ISO:**
```bash
docker run --rm -t -v $(pwd)/_out:/out \
  ghcr.io/siderolabs/imager:v1.12.0 iso \
  --overlay-name secureboot \
  --output /out
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
| nvidia-container-toolkit | `ghcr.io/siderolabs/nvidia-container-toolkit` | NVIDIA GPU |
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
