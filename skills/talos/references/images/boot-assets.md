# Boot assets and the imager

Targets Talos **v1.14.0** (default Kubernetes 1.37.0). Enough to build an ISO, disk image or
installer without leaving the skill; upstream detail lives at
https://docs.siderolabs.com/talos/v1.14/platform-specific-installations/boot-assets and
https://docs.siderolabs.com/talos/v1.14/learn-more/image-factory.

## Where installer images come from in v1.14

`ghcr.io/siderolabs/installer` is no longer published — `installer:v1.14.0` returns **404**, while
`installer:v1.13.10` still resolves. Replace every occurrence: the `image` argument to
`talos_upgrade`, `.machine.install.image`, PXE and provisioning templates. A 404 during an upgrade
almost always means the old path is still in use.

Still published and safe to reference: `ghcr.io/siderolabs/imager:v1.14.0`,
`ghcr.io/siderolabs/installer-base:v1.14.0`, `ghcr.io/siderolabs/extensions:v1.14.0`,
`ghcr.io/siderolabs/overlays:v1.14.0`.

### Image Factory

A **schematic** — content-addressable YAML holding extensions, extra kernel args, an overlay and
META values — is POSTed once to https://factory.talos.dev/ and yields a schematic id that names
every asset built from it:

```
factory.talos.dev/metal-installer/<schematic-id>:v1.14.0
factory.talos.dev/image/<schematic-id>/v1.14.0/metal-amd64.raw.zst
factory.talos.dev/image/<schematic-id>/v1.14.0/metal-amd64.iso
https://pxe.factory.talos.dev/pxe/<schematic-id>/v1.14.0/metal-amd64
```

The legacy `factory.talos.dev/installer/<id>:<ver>` path still resolves, but `metal-installer` is
the current name. `curl -s https://factory.talos.dev/versions` lists what the Factory can build.
The Factory and a self-built installer pushed to a private registry are the only two sources.

The empty/default schematic — no extensions, no extra kernel args — gives the stock installer:

```
factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.14.0
```

**Never assume that default for a running node.** Ask the node which schematic it was installed
from before picking an upgrade image:

```
talos_get(resource_type="imagefactoryschematics", node="10.0.0.11")
```

The singleton resource carries `schematicId`, `flavor` and `apiUrl`. An empty result means the node
was not installed from a Factory asset, and the image must be sourced however it originally was.

## Local imager

The imager builds assets locally via Docker. It runs rootless: `--privileged` and `-v /dev:/dev`
are needed only for **bootable-media** profiles that require loop devices (`iso`, `metal*`, cloud
targets). The `installer` profiles do not need them. Bind a host directory to `/out` in every case.

```bash
mkdir -p _out
docker run --rm -t --privileged -v /dev:/dev -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 <profile> [options]
```

### Profiles

`imager <profile>` accepts one of 32 built-in names, or `-` to read a full profile YAML on stdin.
Anything else is rejected — in particular there is **no `disk-image` profile**.

Bare metal and generic: `iso`, `metal`, `metal-4k` (4096-byte sectors), `metal-uki`, `installer`,
plus `secureboot-iso`, `secureboot-metal`, `secureboot-metal-uki`, `secureboot-installer`.
`metal*` outputs are zstd-compressed raw images; `installer` outputs a Docker/OCI tarball.

Cloud and virtualized targets — `akamai`, `alibabacloud`, `aws`, `azure`, `cloudstack`,
`digital-ocean`, `exoscale`, `gcp`, `hcloud`, `nocloud`, `opennebula`, `openstack`, `oracle`,
`scaleway`, `upcloud`, `vmware`, `vultr`, plus `secureboot-` variants for azure, cloudstack,
nocloud, opennebula, openstack and vmware — nearly all emit a raw disk image in a
platform-appropriate wrapper (zstd, gzip or tar). Two exceptions: **`oracle` produces QCOW2** and
**`vmware` produces an OVA**. Confirm a specific output against `pkg/imager/profile/default.go` at
the tag being built.

### Options that matter

Full list: `docker run --rm ghcr.io/siderolabs/imager:v1.14.0 --help`.

| Flag | Use |
|---|---|
| `--system-extension-image <ref>` | Add a system extension; repeatable, digest-pinned |
| `--overlay-image <ref>` / `--overlay-name <name>` | SBC overlay image and which overlay inside it |
| `--arch <amd64\|arm64>` | Target architecture; defaults to the host arch |
| `--image-cache <image\|oci-path>` | Embed a pull-through image cache into the asset |
| `--embedded-config-path <file>` | Bake a machine config in for an unattended install |

`--output` defaults to `/out` inside the container, so bind a host path to `/out` rather than
passing the flag.

### Reproducibility

Disk images are reproducible: the same Talos version with the same inputs yields byte-identical
output, verifiable by SHA. VHD and VMDK/OVA outputs (Azure, VMware) are **not** reproducible —
limitations of the conversion tools. Verify the raw image and convert afterwards.

## Worked examples

**ISO with extensions:**

```bash
mkdir -p _out
docker run --rm -t --privileged -v /dev:/dev -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 iso \
  --system-extension-image ghcr.io/siderolabs/iscsi-tools:v0.2.0 \
  --system-extension-image ghcr.io/siderolabs/qemu-guest-agent:11.1.0
```

**Installer with an NVIDIA stack, then load, tag and push:**

```bash
mkdir -p _out
docker run --rm -t -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 installer \
  --system-extension-image ghcr.io/siderolabs/nonfree-kmod-nvidia-lts:580.178.04-v1.14.0 \
  --system-extension-image ghcr.io/siderolabs/nvidia-container-toolkit-lts:580.178.04-v1.19.1

LOADED=$(docker load -i _out/installer-amd64.tar | awk '/Loaded image:/ {print $NF}')
# $LOADED is ghcr.io/siderolabs/installer-base:1.14.0 — installer-arm64.tar for arm64
docker tag  "$LOADED" <registry>/<repo>:v1.14.0-custom
docker push <registry>/<repo>:v1.14.0-custom
```

The tarball loads as **`installer-base`, not `installer`, and with a tag that has no leading `v`**.
Capture the reference from `docker load` rather than reconstructing it. Pass the pushed ref to
`talos_upgrade` or set it in `.machine.install.image`.

**Metal image for a Raspberry Pi:**

```bash
docker run --rm -t --privileged -v /dev:/dev -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 metal \
  --overlay-image ghcr.io/siderolabs/sbc-raspberrypi:v0.2.1 --overlay-name rpi_generic
```

## SecureBoot

SecureBoot is selected by **profile**, not by an overlay. `--overlay-name` picks an SBC overlay
installer and has nothing to do with SecureBoot.

```bash
docker run --rm -t --privileged -v /dev:/dev -v "$PWD/_out:/out" \
  ghcr.io/siderolabs/imager:v1.14.0 secureboot-iso --secureboot-include-well-known-certs
```

- `--secureboot-include-well-known-certs` adds the Microsoft UEFI certificates to the generated db, needed when the firmware must still boot other signed payloads such as option ROMs.
- `--secureboot-enroll-keys` drives systemd-boot's `secure-boot-enroll`: `if-safe` (default, auto-enrols only in a VM) or `force` for unattended bare-metal enrolment with firmware in setup mode.
- Signing keys come from `--secureboot-signer-address` / `--pcr-signer-address` or a profile YAML on stdin; the default profiles bake in no key material.
- v1.14 dropped `lockdown=confidentiality` from SecureBoot images — see `references/v1.14-changes.md`.

## SBC overlays

Overlays resolve from `ghcr.io/siderolabs/overlays:v1.14.0`, whose `overlays.yaml` maps every board
to its image and digest. Look the board up rather than guessing:

```bash
crane export ghcr.io/siderolabs/overlays:v1.14.0 | tar x -O overlays.yaml
```

Two traps: there is **no `sbc-turingrk1` image** — the Turing RK1 overlay lives in `sbc-rockchip`;
and rockchip overlay names are hyphenated (`orangepi-5`, `rock5b-plus`) while Allwinner, Jetson and
Raspberry Pi names use underscores (`jetson_nano`, `pine64`, `rpi_generic`).
