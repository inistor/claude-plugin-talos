# Migrating legacy v1alpha1 to config documents

Upstream keeps the complete old → new field map; consult it for anything mechanical:
https://docs.siderolabs.com/talos/v1.14/reference/configuration/document-map

This file records what that map does not: the fields with **no replacement**, and the two gaps that
force a config to stay on legacy blocks. Conflict rule and procedure: `config-model.md`.

## What is still current in the legacy document

The whole `cluster:` section is deprecated, and most of `machine:`. Not deprecated:

- `.machine.type`, `.machine.token`, `.machine.ca`, `.machine.acceptedCAs`, `.machine.certSANs`
- `.machine.features` — only `rbac`, `apidCheckExtKeyUsage`, `diskQuotaSupport` and
  `nodeAddressSortAlgorithm`; every other subfield is deprecated
- `.machine.logging`, `.machine.seccompProfiles`
- `.cluster.token`, `.cluster.etcd` (except `.cluster.etcd.subnet` → `advertisedSubnets`)

`.machine.network` (the entire tree) was already deprecated in v1.13.

## `.machine` — replacements, and the dead ends

| Deprecated field | Replacement |
|---|---|
| `.machine.kubelet` (`image`, `clusterDNS`, `extraArgs`, `extraConfig`, `defaultRuntimeSeccompProfileEnabled`) | `KubeletConfig` |
| `.machine.kubelet.registerWithFQDN` / `.nodeIP` / `.skipNodeRegistration` | `KubeNodeConfig` |
| **`.machine.kubelet.extraMounts`** | **none — no replacement exists** |
| **`.machine.kubelet.disableManifestsDirectory`** | **none — locked to `true`** under `KubeletConfig` |
| `.machine.controlPlane` | `KubeControllerManagerConfig`, `KubeSchedulerConfig` |
| `.machine.pods` | `KubeStaticPodConfig` |
| `.machine.disks` | `UserVolumeConfig` |
| `.machine.systemDiskEncryption` | `VolumeConfig` |
| `.machine.install` | `UnattendedInstallConfig` (partial — see below) |
| **`.machine.install.extraKernelArgs`** | **none** — bake into an Image Factory schematic / imager profile |
| **`.machine.install.extensions`** | **none** — "Use custom `InstallImage` instead": extensions are baked into the installer image |
| **`.machine.install.bootloader`** | **none** — "Deprecated: It never worked" |
| `.machine.files` | `EtcFileConfig`, `CRICustomizationConfig` |
| `.machine.env` / `.time` / `.baseRuntimeSpecOverrides` | `EnvironmentConfig` / `TimeSyncConfig` / `CRIBaseRuntimeSpecConfig` |
| `.machine.registries` | `RegistryMirrorConfig`, `RegistryAuthConfig`, `RegistryTLSConfig` (`registries.md`) |
| `.machine.nodeLabels` / `.nodeAnnotations` / `.nodeTaints` | `KubeNodeConfig` |
| `.machine.udev`, `.machine.sysctls`, `.machine.sysfs`, `.machine.kernel.modules` | `UdevRulesConfig`, `SysctlConfig`, `SysfsConfig`, `KernelModuleConfig` — these four **merge** rather than conflict |
| `.machine.network.*` | `HostnameConfig`, `LinkConfig`, `ResolverConfig`, `StaticHostConfig`, `KubeSpanConfig` — see `references/networking/links.md` |
| `.machine.features.stableHostname` / `.hostDNS` | `HostnameConfig` (`auto: stable`) / `ResolverConfig` |
| `.machine.features.kubePrism` / `.kubernetesTalosAPIAccess` / `.imageCache` | `KubePrismConfig` / `KubeTalosAPIAccessConfig` / `ImageCacheConfig` |

## `.cluster`

Almost all of it maps one-to-one onto a `Kube*Config` document named after the field — take those
from the upstream document map. The rows worth knowing separately:

| Deprecated field | Replacement | Not mechanical because |
|---|---|---|
| `.cluster.network` | `KubeNetworkConfig` **+** `KubeFlannelCNIConfig` | splits in two; omitting the Flannel document is how a custom CNI is selected |
| `.cluster.controlPlane.localAPIServerPort` | `KubeAPIServerConfig.apiPort` | lands in a different document from the rest of `.cluster.controlPlane` |
| `.cluster.allowSchedulingOnControlPlanes` | `KubeNodeConfig` | no boolean — drop `node-role.kubernetes.io/control-plane: NoSchedule` from `taints` |
| `.cluster.inlineManifests` / `.extraManifests` | `KubeInlineManifestConfig` / `KubeExternalManifestConfig` | a list becomes one **named document per manifest** |
| `.cluster.discovery` | `DiscoveryServiceConfig` | named, so several coexist; ship no document to disable discovery |
| `.cluster.aescbc`/`.secretboxEncryptionSecret` | `KubeEtcdEncryptionConfig` | the bare secret becomes a full upstream `EncryptionConfiguration` |
| `.cluster.etcd.subnet` | `.cluster.etcd.advertisedSubnets` | stays inside the legacy document |

## Trap 1: `extraMounts` pins the whole kubelet block

`.machine.kubelet.extraMounts` has no document equivalent. A node needing an extra host mount in
the kubelet container must keep the whole legacy `.machine.kubelet` block, and therefore **cannot
use `KubeletConfig` at all** — adding it fails with `kubelet config is already set in v1alpha1
config (.machine.kubelet)`. There is no partial migration: image, `clusterDNS`, `extraArgs` and
seccomp settings stay legacy too.

## Trap 2: `UnattendedInstallConfig` covers only four fields

`UnattendedInstallConfig` exposes `installer.image`, `provisioning.diskSelector.match`,
`provisioning.wipe` and `reboot`. The remaining `.machine.install` fields — `disk`,
`legacyBIOSSupport`, `grubUseUKICmdline`, `extraKernelArgs`, `extensions`, `bootloader` — have no
counterpart, and the two are mutually exclusive
(`UnattendedInstallConfig config is incompatible with v1alpha1 config (.machine.install)`).

Three of those six are dead rather than missing: `bootloader` never worked, and `extraKernelArgs` /
`extensions` are now properties of the installer image, built through the Image Factory or the
local imager (`references/images/boot-assets.md`). `disk` rewrites cleanly as
`provisioning.diskSelector.match`. That leaves `legacyBIOSSupport` and `grubUseUKICmdline` as the
real blockers, on BIOS hardware only.
