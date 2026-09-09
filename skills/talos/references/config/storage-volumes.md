# Storage and volume documents

Field reference: https://docs.siderolabs.com/talos/v1.14/reference/configuration/ — this file
covers the selectors, defaults and rejections that the schema does not make obvious.

Disk and volume selectors are CEL expressions over discovered hardware — `disk.transport`,
`disk.rotational`, `disk.size`, `system_disk`, `volume.partition_label`. The shape below is the
same everywhere a `diskSelector` or `volumeSelector` appears.

## UserVolumeConfig — the one operators hand-write

Mounted at `/var/mnt/<name>`. Name is 1–34 chars, letters/digits/hyphens.

```yaml
apiVersion: v1alpha1
kind: UserVolumeConfig
name: ceph-data
provisioning:
    diskSelector:
        match: disk.transport == "nvme" && !system_disk
    minSize: 10GiB
    maxSize: 100GiB
    # grow: true
filesystem:
    type: xfs                      # xfs (default) | ext4 | btrfs
    projectQuotaSupport: true
    xfs:
        minAllocationGroupSize: 128GiB
mount:
    disableAccessTime: true
    # secure: true
trim:
    enabled: true
scrub:
    enabled: true
    interval: 168h0m0s
# volumeType: partition            # directory | disk | partition
```

- `xfs.minAllocationGroupSize` applies only at format time. On non-rotational devices `mkfs.xfs`
  sizes the allocation-group count to the CPU count — hundreds of tiny groups on a many-core host
  with a modest disk. Talos bounds the group size from below; this field overrides that bound, and
  `0` falls back to plain `mkfs.xfs` defaults.
- With `volumeType: directory`, the `provisioning`, `filesystem` and `encryption` sections and
  `mount.disableAccessTime` are all **rejected** at validation
  (`provisioning spec is invalid for volumeType directory`, and so on).

Related block documents, all sharing the same `provisioning` / selector shape:

| Kind | Use | Notes |
|---|---|---|
| `RawVolumeConfig` | unformatted block device (Ceph OSDs, DBs) | `provisioning` + optional `encryption`; no `filesystem` |
| `SwapVolumeConfig` | swap | `provisioning` + optional `encryption` only |
| `ExistingVolumeConfig` | mount a volume provisioned elsewhere | `discovery.volumeSelector.match`, e.g. `volume.partition_label == "MY-DATA"`; plus `mount`, `trim`, `scrub` |
| `ExternalVolumeConfig` | virtiofs share from the hypervisor | `filesystemType: virtiofs`, `mount.virtiofs.tag` |
| `ZswapConfig` | compressed swap cache | `maxPoolPercent`, `shrinkerEnabled` |

## VolumeConfig — system volumes (`EPHEMERAL`, `STATE`, …)

```yaml
apiVersion: v1alpha1
kind: VolumeConfig
name: STATE
encryption:
    provider: luks2
    keys:
        - slot: 0
          tpm:
            options:
                pcrs: [0, 7]
        - slot: 1
          static:
            passphrase: topsecret
```

Key types: `static.passphrase`, `nodeID`, `kms.endpoint`, `tpm` (with `options.pcrs`,
`checkSecurebootStatusOnEnroll`, `lockToState`); `pcrs: []` disables PCR binding.

**Conflict:** encryption on `EPHEMERAL` or `STATE` in a `VolumeConfig` while the legacy
`.machine.systemDiskEncryption` still carries an entry for the same volume fails to load with
`system disk encryption for "STATE" is configured in both v1alpha1.Config and VolumeConfig`.
Only those two names can conflict; other `VolumeConfig` names load alongside the legacy field.

## Trim and scrub

```yaml
apiVersion: v1alpha1
kind: FilesystemTrimConfig
interval: 168h0m0s
---
apiVersion: v1alpha1
kind: FilesystemScrubConfig
interval: 168h0m0s
```

Both are singletons and accept an empty body (defaults apply). Absent the trim document, nothing
is trimmed automatically unless a volume enables it in its own `trim:` block; per-volume `trim` /
`scrub` override the global interval. Each run fires at a **stable hash-derived offset** within the
interval, distinct per node and per volume, so a fleet never trims in lockstep — the times will
look arbitrary; do not try to align them.

## LVM

```yaml
apiVersion: v1alpha1
kind: LVMVolumeGroupConfig
name: vg-pool
provisioning:
    volumeSelector:
        match: disk.transport == "nvme"
---
apiVersion: v1alpha1
kind: LVMLogicalVolumeConfig
name: lv-data
type: linear                # linear | raid0 | raid1 | raid10
provisioning:
    volumeGroup: vg-pool
    maxSize: 50GiB          # bytes, or a percentage of the VG ("80%")
    # minSize: 10GiB
# mirrors: 1                # raid1 / raid10 only; defaults to 1 (two-way)
# stripes: 2                # raid0 / raid10 only; defaults to all PVs, minimum 2
```

**Growing works, shrinking does not.** Lowering `maxSize` below the current size is refused with
`requested size N bytes is smaller than current size M bytes; shrinking logical volumes is not
supported`. `raid1` stores two copies, so a volume-group change can read as a shrink request even
when the declared size is unchanged.

## RAID

```yaml
apiVersion: v1alpha1
kind: RAIDArrayConfig
name: data
level: raid1                # the only level supported today
metadata: "1.2"             # "1.0" is the default
provisioning:
    volumeSelector:
        match: disk.transport == "virtio"
```

Metadata **`1.0` (the default) puts the superblock at the end** of each member device, which is
what lets the array back a bootable partition. Use `1.2` for pure data arrays. The format is fixed
at creation.
