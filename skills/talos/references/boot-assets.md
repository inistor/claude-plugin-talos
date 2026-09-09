# Boot Assets & Imager Reference

Targets Talos **v1.14.0** (released 2026-09-03, default Kubernetes 1.37.0).

Docs:
- Boot assets — https://docs.siderolabs.com/talos/v1.14/platform-specific-installations/boot-assets
- System extensions — https://docs.siderolabs.com/talos/v1.14/build-and-extend-talos/custom-images-and-development/system-extensions
- Overlays — https://docs.siderolabs.com/talos/v1.14/build-and-extend-talos/custom-images-and-development/overlays
- SecureBoot — https://docs.siderolabs.com/talos/v1.14/platform-specific-installations/bare-metal-platforms/secureboot
- Image Factory — https://docs.siderolabs.com/talos/v1.14/learn-more/image-factory

## Breaking change in 1.14: no more `ghcr.io/siderolabs/installer`

`ghcr.io/siderolabs/installer` is **no longer published** as of v1.14.0 (`installer:v1.14.0` → 404;
`installer:v1.13.10` → 200). The default installer image is now served by the Image Factory:

```
factory.talos.dev/metal-installer/<schematic-id>:v1.14.0
```

The empty/default schematic (no extensions, no extra kernel args) is
`376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba`, so a stock install/upgrade image is:

```
factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.14.0
```

Still published and safe to use:
- `ghcr.io/siderolabs/imager:v1.14.0` — the local imager (verified present in the registry)
- `ghcr.io/siderolabs/installer-base:v1.14.0` — the base the imager layers onto
- `ghcr.io/siderolabs/extensions:v1.14.0`, `ghcr.io/siderolabs/overlays:v1.14.0` — manifest images

Anywhere the old `ghcr.io/siderolabs/installer:vX.Y.Z` string appears (`.machine.install.image`,
`talos_upgrade` arguments, PXE configs), replace it with a Factory `metal-installer` ref or with an
installer you built and pushed yourself.

## Local Imager

The imager builds custom Talos images locally via Docker. It runs rootless — `--privileged` and
`-v /dev:/dev` are only needed for **bootable-media** profiles (`iso`, `metal*`, cloud targets) that need
loop devices. The `installer` profiles do not need them. Always bind-mount a host directory to `/out`
(the in-container output path).

Bootable media (iso / metal / cloud):
```bash
mkdir -p _out
docker run --rm -t --privileged \
  -v /dev:/dev \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 \
  <profile> [options]
```

Installer (Docker image tar):
```bash
mkdir -p _out
docker run --rm -t \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 \
  installer [options]
```

### Profiles

`imager <profile>` takes one of the built-in profile names below (source of truth:
`pkg/imager/profile/default.go` at the tag you are building). Anything not in this list is rejected —
in particular there is **no `disk-image` profile**. You can also pass `-` and feed a full profile YAML
on stdin.

**Bare metal / generic:**

| Profile | Output |
|---|---|
| `iso` | Bootable ISO |
| `secureboot-iso` | SecureBoot ISO (systemd-boot key enrollment `if-safe`) |
| `metal` | Raw disk image, **zstd-compressed** (`.raw.zst`) |
| `metal-4k` | Same, 4096-byte sector size |
| `metal-uki` | Unified Kernel Image (`.efi`) |
| `secureboot-metal` | SecureBoot raw disk image, zstd-compressed |
| `secureboot-metal-uki` | Signed UKI |
| `installer` | Installer container image, as a Docker/OCI tarball |
| `secureboot-installer` | SecureBoot installer container image tarball |

**Clouds and virtualized platforms:**

| Profile | Output |
|---|---|
| `akamai` | Raw, gzip |
| `alibabacloud` | QCOW2 |
| `aws` | Raw, zstd (AMI import) |
| `azure` / `secureboot-azure` | VHD (fixed), zstd |
| `cloudstack` / `secureboot-cloudstack` | Raw, zstd |
| `digital-ocean` | Raw, gzip |
| `exoscale` | QCOW2, zstd |
| `gcp` | Raw, tar |
| `hcloud` | Raw, zstd |
| `nocloud` / `secureboot-nocloud` | Raw, zstd |
| `opennebula` / `secureboot-opennebula` | Raw, zstd |
| `openstack` / `secureboot-openstack` | Raw, zstd |
| `oracle` | **QCOW2**, zstd (Oracle Cloud Infrastructure) |
| `scaleway` | Raw, zstd |
| `upcloud` | Raw, zstd |
| `vmware` / `secureboot-vmware` | **OVA** |
| `vultr` | Raw, zstd |

### Options

```
--system-extension-image <image>          Add a system extension (repeatable; use a pinned digest)
--extra-kernel-arg <arg>                  Add a kernel command-line arg (repeatable)
--meta <key>=<value>                      Set a META partition value (repeatable)
--base-installer-image <image>            Override the base installer (default: installer-base for the version)
--arch <amd64|arm64>                      Target architecture (defaults to the host arch)
--platform <name>                         Override the platform kernel param
--output <path>                           Output directory inside the container (default /out)
--output-kind <kind>                      Override the profile's output kind (iso, image, uki, installer, ...)
--tar-to-stdout                           Tar the output and stream it to stdout instead of /out
--image-cache <image|oci-path>            Embed a pull-through image cache into the asset
--embedded-config-path <file>             Bake a machine config into the image (unattended install)
--insecure                                Pull assets from an insecure (plain HTTP / bad TLS) registry
--overlay-image <image>                   SBC overlay image (see SBC Overlays)
--overlay-name <name>                     Which overlay inside that image to use (rpi_generic, rock5b, ...)
--overlay-option <k=v>                    Extra option passed through to the overlay (repeatable)
--secureboot-include-well-known-certs     Include Microsoft UEFI certs in the generated SecureBoot db
--secureboot-enroll-keys <mode>           systemd-boot key enrollment: if-safe (default), force, ...
--secureboot-signer-address <unix://...>  gRPC SecureBoot signer service
--pcr-signer-address <unix://...>         gRPC PCR signer service
```

`--output` defaults to `/out` inside the container, so bind a host path to `/out` rather than passing it.

### Finding the right extension tag

Extension tags are **not** uniform, and most of them are not the Talos version. The canonical lookup is
the `extensions` manifest image for your Talos release:

```bash
crane export ghcr.io/siderolabs/extensions:v1.14.0 | tar x -O image-digests | grep <extension-name>
```

It prints a fully pinned `name:tag@sha256:...` reference — use that digest-pinned form in
`--system-extension-image`. There is **no `:latest` tag**; a copy-pasted `:latest` will fail to pull.

Two families of tags:

- **Kernel-module extensions** are built against a specific Talos version, so their tag embeds it:
  `thunderbolt:v1.14.0`, `zfs:2.4.4-v1.14.0`, `gasket-driver:5815ee3-v1.14.0`,
  `nonfree-kmod-nvidia-lts:580.178.04-v1.14.0`. These **must** match the Talos version you are building.
- **Userspace extensions** are versioned independently and carry the upstream software version:
  `iscsi-tools:v0.2.0`, `tailscale:1.102.2`, `qemu-guest-agent:11.1.0`, `util-linux-tools:2.42.2`,
  `nvidia-container-toolkit-lts:580.178.04-v1.19.1`. Do not "bump" these to the Talos version.

The extensions repo (https://github.com/siderolabs/extensions) also has a generated table with the tier
and current version of every extension.

### Reproducible images

Disk images are reproducible — building the same Talos version with the same inputs yields byte-identical
output; verify via SHA. VHD and VMDK/OVA outputs (Azure, VMware) are not reproducible due to limitations
in the underlying conversion tools; verify the raw image and convert afterward.

### Examples

**ISO with extensions:**
```bash
docker run --rm -t --privileged \
  -v /dev:/dev \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 iso \
  --system-extension-image ghcr.io/siderolabs/iscsi-tools:v0.2.0 \
  --system-extension-image ghcr.io/siderolabs/qemu-guest-agent:11.1.0
```

**Metal image for Raspberry Pi:**
```bash
docker run --rm -t --privileged \
  -v /dev:/dev \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 metal \
  --overlay-image ghcr.io/siderolabs/sbc-raspberrypi:v0.2.1 \
  --overlay-name rpi_generic
```

**Installer with an NVIDIA extension stack:**
```bash
docker run --rm -t \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 installer \
  --system-extension-image ghcr.io/siderolabs/nonfree-kmod-nvidia-lts:580.178.04-v1.14.0 \
  --system-extension-image ghcr.io/siderolabs/nvidia-container-toolkit-lts:580.178.04-v1.19.1
```

After building the installer, load it into the local Docker daemon, tag it for your registry, and push so
the nodes can pull it. The tarball is tagged with the **base installer** reference, i.e.
`ghcr.io/siderolabs/installer-base:1.14.0` — note `installer-base`, not `installer`, and a tag **without**
the leading `v`. Capture the loaded reference rather than guessing:
```bash
LOADED=$(docker load -i _out/installer-amd64.tar | awk '/Loaded image:/ {print $NF}')
# $LOADED will be ghcr.io/siderolabs/installer-base:1.14.0 (installer-arm64.tar for arm64 builds)
docker tag  "$LOADED" <registry>/<repo>:v1.14.0-custom
docker push <registry>/<repo>:v1.14.0-custom
```
Then reference `<registry>/<repo>:v1.14.0-custom` in `talos_upgrade` or in `.machine.install.image`.

### SecureBoot

SecureBoot is selected by **profile**, not by an overlay. `--overlay-name` picks an SBC overlay installer
(`rpi_generic`, `rock5b`, …) and has nothing to do with SecureBoot.

```bash
# SecureBoot ISO
docker run --rm -t --privileged \
  -v /dev:/dev \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 secureboot-iso \
  --secureboot-include-well-known-certs

# SecureBoot installer image
docker run --rm -t \
  -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 secureboot-installer
```

- `secureboot-iso`, `secureboot-metal`, `secureboot-metal-uki`, `secureboot-installer` (plus the
  `secureboot-<cloud>` variants) are the SecureBoot entry points.
- `--secureboot-include-well-known-certs` adds the Microsoft UEFI certificates to the generated
  SecureBoot database — needed when the firmware still has to boot other signed payloads (option ROMs).
- `--secureboot-enroll-keys` controls systemd-boot's `secure-boot-enroll`: `if-safe` (default, auto-enrolls
  only in a VM) or `force` for unattended bare-metal enrollment with the firmware in setup mode.
- Signing keys come from `--secureboot-signer-address` / `--pcr-signer-address`, or from a full profile
  YAML on stdin; there is no key material baked into the default profiles.
- 1.14 change: SecureBoot images no longer set `lockdown=confidentiality` by default. Add it via kernel
  args (or Image Factory) if you need it.

## Common System Extensions (v1.14.0)

Tiers are from the `siderolabs/extensions` README: **core** = fully supported by Sidero Labs,
**extra** = best-effort support, **contrib** = community-supported. Tags below are the ones shipped in
`extensions:v1.14.0` — re-check with the `crane export` lookup above before pinning.

### Core

| Extension | Image | Tag in 1.14.0 | Purpose |
|---|---|---|---|
| iscsi-tools | `ghcr.io/siderolabs/iscsi-tools` | `v0.2.0` | iSCSI initiator |
| intel-ucode | `ghcr.io/siderolabs/intel-ucode` | `20260812` | Intel microcode |
| amd-ucode | `ghcr.io/siderolabs/amd-ucode` | `20260810` | AMD microcode |
| fuse3 | `ghcr.io/siderolabs/fuse3` | `3.18.2` | FUSE support |
| gvisor | `ghcr.io/siderolabs/gvisor` | `20260810.0` | gVisor runtime handler |
| libvirtd | `ghcr.io/siderolabs/libvirtd` | `12.6.0` | libvirt daemon (new `hypervisors/` category) |
| nvidia-container-toolkit-lts | `ghcr.io/siderolabs/nvidia-container-toolkit-lts` | `580.178.04-v1.19.1` | NVIDIA userspace + persistence daemon (LTS branch) |
| nvidia-container-toolkit-production | `ghcr.io/siderolabs/nvidia-container-toolkit-production` | `595.91.07-v1.19.1` | Same, production branch |
| nonfree-kmod-nvidia-lts | `ghcr.io/siderolabs/nonfree-kmod-nvidia-lts` | `580.178.04-v1.14.0` | Proprietary NVIDIA kernel modules |
| nvidia-open-gpu-kernel-modules-lts | `ghcr.io/siderolabs/nvidia-open-gpu-kernel-modules-lts` | `580.178.04-v1.14.0` | Open NVIDIA kernel modules |

> The plain `nvidia-container-toolkit` / `nonfree-kmod-nvidia` / `nvidia-open-gpu-kernel-modules` names no
> longer exist. Every NVIDIA extension is split into `-lts` and `-production` variants, and **all NVIDIA
> extensions on a node must come from the same branch and the same release**.

### Extra

| Extension | Image | Tag in 1.14.0 | Purpose |
|---|---|---|---|
| qemu-guest-agent | `ghcr.io/siderolabs/qemu-guest-agent` | `11.1.0` | QEMU/KVM guest agent |
| hyperv-guest-agent | `ghcr.io/siderolabs/hyperv-guest-agent` | `6.18.44` | Hyper-V KVP/VSS daemons (new in 1.14) |
| tailscale | `ghcr.io/siderolabs/tailscale` | `1.102.2` | Tailscale VPN |
| drbd | `ghcr.io/siderolabs/drbd` | `9.3.3-v1.14.0` | DRBD replication |
| zfs | `ghcr.io/siderolabs/zfs` | `2.4.4-v1.14.0` | ZFS modules + tools |
| btrfs | `ghcr.io/siderolabs/btrfs` | `v1.14.0` | btrfs (required for btrfs user volumes in 1.14) |
| multipath-tools | `ghcr.io/siderolabs/multipath-tools` | `v0.1.0` | **Breaking in 1.14** — see troubleshooting.md |
| gasket-driver | `ghcr.io/siderolabs/gasket-driver` | `5815ee3-v1.14.0` | Google Coral TPU |
| thunderbolt | `ghcr.io/siderolabs/thunderbolt` | `v1.14.0` | Thunderbolt/USB4 |
| usb-modem-drivers | `ghcr.io/siderolabs/usb-modem-drivers` | `v1.14.0` | USB modems |
| kata-containers | `ghcr.io/siderolabs/kata-containers` | `3.32.0` | Kata runtime handler |
| kata-containers-snp | `ghcr.io/siderolabs/kata-containers-snp` | `3.32.0` | AMD SEV-SNP confidential VMs (new in 1.14) |
| kata-containers-nvidia-gpu-snp | `ghcr.io/siderolabs/kata-containers-nvidia-gpu-snp` | `3.32.0` | SEV-SNP + NVIDIA GPU passthrough (new in 1.14) |
| harbor-credential-provider | `ghcr.io/siderolabs/harbor-credential-provider` | `v0.0.1` | Harbor kubelet credential provider (new in 1.14) |

### Contrib

| Extension | Image | Tag in 1.14.0 | Purpose |
|---|---|---|---|
| util-linux-tools | `ghcr.io/siderolabs/util-linux-tools` | `2.42.2` | Minimal util-linux |
| nvme-cli | `ghcr.io/siderolabs/nvme-cli` | `v2.14` | NVMe CLI |
| cachefilesd | `ghcr.io/siderolabs/cachefilesd` | `0.10.10-v1.14.0` | FS-Cache backing daemon (new in 1.14) |
| nfs-utils / nfs-server | `ghcr.io/siderolabs/nfs-utils`, `…/nfs-server` | `v0.1.1`, `v0.1.0` | NFS client locking / NFS server |

New in 1.14 overall: `cachefilesd`, `kata-containers-snp`, `kata-containers-nvidia-gpu-snp`,
`harbor-credential-provider`, `hyperv-guest-agent`, and a new `hypervisors/` category (`libvirtd`).

### SBC Overlays

Overlays are resolved from `ghcr.io/siderolabs/overlays:v1.14.0`, which carries an `overlays.yaml`
mapping every board name to its image and digest:

```bash
crane export ghcr.io/siderolabs/overlays:v1.14.0 | tar x -O overlays.yaml
```

| Board(s) | `--overlay-image` | `--overlay-name` |
|---|---|---|
| Raspberry Pi 3/4 | `ghcr.io/siderolabs/sbc-raspberrypi:v0.2.1` | `rpi_generic` |
| Raspberry Pi 5 | `ghcr.io/siderolabs/sbc-raspberrypi:v0.2.1` | `rpi_5` |
| RevolutionPi | `ghcr.io/siderolabs/sbc-raspberrypi:v0.2.1` | `revpi_generic` |
| Jetson Nano | `ghcr.io/siderolabs/sbc-jetson:v0.1.5` | `jetson_nano` |
| Pine64 / Banana Pi M64 / Libretech H3-CC-H5 | `ghcr.io/siderolabs/sbc-allwinner:v0.1.5` | `pine64`, `bananapi_m64`, `libretech_all_h3_cc_h5` |
| Orange Pi 5 / 5 Plus / 5 Max | `ghcr.io/siderolabs/sbc-rockchip:v0.2.1` | `orangepi-5`, `orangepi-5-plus`, `orangepi-5-max` |
| Rock 5A/5B/5T/5B+ | `ghcr.io/siderolabs/sbc-rockchip:v0.2.1` | `rock5a`, `rock5b`, `rock5t`, `rock5b-plus` |
| Rock Pi 4 / 4C / 4C+ / 4SE, Rock64, RockPro64 | `ghcr.io/siderolabs/sbc-rockchip:v0.2.1` | `rockpi4`, `rockpi4c`, `rock4cplus`, `rock4se`, `rock64`, `rockpro64` |
| Turing RK1 | `ghcr.io/siderolabs/sbc-rockchip:v0.2.1` | `turingrk1` |
| NanoPi R4S / R5S, Odroid M1, Helios64, CM3588 NAS, Radxa Zero 3E | `ghcr.io/siderolabs/sbc-rockchip:v0.2.1` | `nanopi-r4s`, `nanopi-r5s`, `odroid-m1`, `helios64`, `friendlyelec-cm3588-nas`, `radxa-zero-3e` |

> There is no `sbc-turingrk1` image — Turing RK1 lives in `sbc-rockchip`. Rockchip overlay names use
> hyphens (`orangepi-5`), while Allwinner/Jetson/RPi names use underscores (`jetson_nano`, `rpi_generic`).

## Image Factory (Online)

Hosted alternative to the local imager: https://factory.talos.dev/

A **schematic** (content-addressable YAML: extensions, extra kernel args, overlay, META) is POSTed once
and yields a schematic ID that names every asset built from it:

```
factory.talos.dev/metal-installer/<schematic-id>:v1.14.0    # installer image (replaces ghcr installer)
factory.talos.dev/image/<schematic-id>/v1.14.0/metal-amd64.raw.zst
factory.talos.dev/image/<schematic-id>/v1.14.0/metal-amd64.iso
https://pxe.factory.talos.dev/pxe/<schematic-id>/v1.14.0/metal-amd64
```

- The legacy `factory.talos.dev/installer/<id>:<ver>` path still resolves, but `metal-installer` is the
  current name.
- `376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba` is the empty/default schematic.
- `curl -s https://factory.talos.dev/versions` lists the Talos versions the Factory can build.
- Because 1.14 no longer publishes `ghcr.io/siderolabs/installer`, the Factory (or a self-built installer
  pushed to your own registry) is the only supported source of an installer image.

The `/talos-image` command in this plugin uses the local imager, not the Factory.
