# Talos Machine Config: the document model (v1.14)

Every field is documented upstream — look there rather than restating it:
https://docs.siderolabs.com/talos/v1.14/reference/configuration/overview
(legacy schema: `.../reference/configuration/v1alpha1/config`).

Siblings: `config-migration.md`, `kubernetes-documents.md`, `storage-volumes.md`, `registries.md`;
1.14 behaviour changes live in `references/v1.14-changes.md`.

## Multi-document stream

A machine config is a stream of YAML documents separated by `---`. Every non-legacy document
carries `apiVersion` + `kind`; named kinds also carry `name`.

```yaml
apiVersion: v1alpha1
kind: KubeClusterConfig
clusterName: example-cluster
endpoint: https://example.com:6443/
---
apiVersion: v1alpha1
kind: RegistryMirrorConfig
name: ghcr.io
endpoints:
    - url: https://my-private-registry.local:5000
```

- **Identity** is the tuple `(apiVersion, kind, name)`. `name` is absent for singleton kinds
  (`KubeNetworkConfig`, `SysctlConfig`, `ResolverConfig`, …).
- **Duplicates are rejected** at load: `duplicate document <apiVersion>/<kind>/<name> is not allowed (line N)`.
- **Order is irrelevant** — documents are addressed by identity, not position.
- The legacy `version: v1alpha1` document (holding `machine:` / `cluster:`) remains a valid member
  of the stream, subject to the conflict rules below. `talosctl gen config` on 1.14 emits a small
  legacy document (machine type, tokens, CAs, `machine.features`, `cluster.token`, `cluster.etcd`)
  plus roughly 25 typed documents.

## Conflict rule

For most deprecated fields, carrying **both** the legacy field and its replacement document makes
the config fail to load. The error names the exact legacy path — use it to diagnose a rejected
apply. Verbatim strings:

```
kubelet config is already set in v1alpha1 config (.machine.kubelet)
kube-apiserver config is already set in v1alpha1 config (.cluster.apiServer)
.machine.nodeLabels is already set in v1alpha1 config
cluster proxy config in v1alpha1 config (.machine.cluster.proxy) can't be used with KubeProxyConfig document, please remove it to avoid conflicts
cluster network config in v1alpha1 config (.machine.cluster.network) can't be used with KubeFlannelCNIConfig document, please remove it to avoid conflicts
UnattendedInstallConfig config is incompatible with v1alpha1 config (.machine.install)
```

Same shapes recur for `.cluster.ca`, `.cluster.aggregatorCA`, `.cluster.serviceAccount`,
`.cluster.scheduler`, `.cluster.controllerManager`, `.cluster.coreDNS`,
`.machine.features.kubePrism`, `.machine.features.kubernetesTalosAPIAccess`.

Migration is therefore **all-or-nothing per area**: delete the legacy block in the same patch that
adds the document.

**Four exceptions merge instead of erroring**, with the document winning per key:

| Legacy field | Document |
|---|---|
| `.machine.sysctls` | `SysctlConfig` |
| `.machine.sysfs` | `SysfsConfig` |
| `.machine.kernel.modules` | `KernelModuleConfig` |
| `.machine.udev.rules` | `UdevRulesConfig` |

## Patching (`talos_patch`)

A patch is itself a document stream. Matching identities merge; new identities are appended.

```yaml
apiVersion: v1alpha1
kind: KubeletConfig
clusterDNS:
  - 3.3.3.3
---
apiVersion: v1alpha1
kind: RegistryMirrorConfig
name: docker.io
endpoints:
  - url: https://mirror.internal:5000
```

Delete a **whole document** by pairing `$patch: delete` with its identity keys; delete a **list
element** by matching one of its other keys; delete a **legacy field** the same way. A delete
selector that matches nothing is silently skipped.

```yaml
apiVersion: v1alpha1
kind: SideroLinkConfig
$patch: delete
---
apiVersion: v1alpha1
kind: ExtensionServiceConfig
name: foo
configFiles:
  - content: hello
    $patch: delete
```

### Lists that replace instead of appending

Fields tagged `merge:"replace"` in the schema overwrite wholesale — a patch that adds one entry
drops the rest:

- `KubeNetworkConfig.podSubnets` / `.serviceSubnets` (and legacy `.cluster.network.*`)
- `KubeAuditPolicyConfig.configuration`, `KubeAuthenticationConfig.configuration`,
  `KubeCredentialProviderConfig.configuration` (and legacy `.cluster.apiServer.auditPolicy`)
- `KubeStaticPodConfig.pod`
- `CRIBaseRuntimeSpecConfig.overrides`
- `NetworkRuleConfig.ingress` / `.ports`
- `BGPInstanceConfig.advertise` / `.importRoutes` / `.neighbors`

Legacy lists with custom merge keys: `.machine.network.interfaces` by `interface:` or
`deviceSelector:`, `.vlans` by `vlanId:`, `.cluster.apiServer.admissionControl` by plugin `name:`.
`.machine.network.nameservers` overwrites all lower layers including platform defaults — setting
only IPv4 entries silently discards platform IPv6 ones. Prefer `ResolverConfig`.

## Applying

| Mode | Behaviour |
|---|---|
| `auto` | Talos decides: live where possible, reboot otherwise. **Preferred.** |
| `no-reboot` | Apply live; fail if the change needs a reboot. |
| `staged` | Write now, apply on next boot. |
| `try` | Apply with a timeout, roll back automatically. Good for network changes. |
| `reboot` | Deprecated — see `references/v1.14-changes.md`. |

Validate before applying: `talosctl validate --config <file> --mode metal`. It catches identity
duplicates, conflict-rule violations and CEL syntax — not runtime facts such as whether a matching
disk exists. For the upgrade flow, see `references/operations/upgrade-talos.md`.

## CRI customization without a reboot

`CRICustomizationConfig` writes containerd drop-ins live — no reboot, no image rebuild. It is
named, so several coexist; it replaces `.machine.files` + `/etc/cri/conf.d/*.part`.

```yaml
apiVersion: v1alpha1
kind: CRICustomizationConfig
name: enable-metrics
content: |
    [plugins."io.containerd.server.v1.metrics"]
      address = "0.0.0.0:11234"
```

`CRIBaseRuntimeSpecConfig.overrides` (OCI runtime spec defaults, e.g. `process.rlimits`) is
`merge:"replace"`.

## UnattendedInstallConfig

Replaces `.machine.install` — and only partially.

```yaml
apiVersion: v1alpha1
kind: UnattendedInstallConfig
installer:
    image: factory.talos.dev/metal-installer/<schematic-id>:v1.14.0
provisioning:
    diskSelector:
        match: disk.transport == "nvme"
    wipe: true      # defaults to true
# reboot: true      # default: reboot only when installer.image is set
```

Omitting `installer.image` makes Talos run the installer matching the running version and the
node's current schematic, which requires an Image-Factory-built boot asset (path shape and default
schematic id: `references/images/boot-assets.md`). Those four fields are the entire surface — see
`references/config/config-migration.md` for the `.machine.install` fields with no counterpart.

## Migration checklist

1. Take one area at a time (kubelet, proxy, registries, volumes, …).
2. In a single patch, add the document **and** delete the legacy block — except for the four
   merging kinds, the two cannot coexist.
3. On failure, read the error: it names the legacy path still present.
4. Keep `.machine.kubelet` only for `extraMounts`; there is no document equivalent.
5. Replace every `ghcr.io/siderolabs/installer:v1.14.x` reference with a Factory
   `metal-installer` ref (`references/images/boot-assets.md`).
6. Re-run `talosctl validate` before applying to a control-plane node.
