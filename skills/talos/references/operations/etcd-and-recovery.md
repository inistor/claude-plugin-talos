# etcd maintenance and disaster recovery

The most dangerous area in Talos. etcd holds all Kubernetes state; losing quorum stops the API
server, and losing the data with no snapshot loses the cluster. Read this before touching a
control-plane node.

## Tools

| Task | Tool |
|---|---|
| List members | `talos_etcd_members` |
| Status and alarms | `talos_etcd_status`, `talos_etcd_alarm` |
| Snapshot | `talos_etcd_snapshot` |
| Defragment | `talos_etcd_defrag` |
| Forfeit leadership | `talos_etcd_forfeit_leadership` |
| Leave / remove a member | `talos_etcd_leave`, `talos_etcd_remove_member` |
| Restore from a snapshot | `talosctl bootstrap --recover-from=<snapshot>` (Bash) |

In v1.14 etcd metrics and health moved from port 2379 to **2383** — see
`references/v1.14-changes.md` before trusting an existing scrape or firewall rule.

## Member IDs are decimal strings

`talos_etcd_remove_member` takes the member ID **as a decimal string**, not a number. etcd member
IDs are uint64 and exceed what a JSON number represents exactly, so passing one numerically can
silently target a different member or none at all. Copy the value verbatim from
`talos_etcd_members`.

## Snapshots

Take one before every control-plane upgrade, every reset, and any risky config change.

The MCP server usually runs in an ephemeral container, so `talos_etcd_snapshot` writes to `/out`
and **refuses any path that would be discarded on exit**. Bind-mount a host directory to `/out` in
the MCP container, or the snapshot has nowhere durable to land. Verify the file exists on the host
afterwards — a snapshot that only ever existed inside a container is not a backup.

## Defragmentation

`talos_etcd_defrag` is resource-heavy and blocks the member it runs on. Run it on **one node at a
time**, checking `talos_etcd_status` between nodes and letting the cluster settle. Defragmenting
several members at once can drop quorum. It is worth doing when the database size has grown far
beyond the amount of live data, typically after a `NOSPACE` alarm has been cleared.

## Leadership and departure

Before maintenance on the **leader**, call `talos_etcd_forfeit_leadership` on it. That hands
leadership to another member cleanly rather than forcing an election when the node goes away.

Manual `talos_etcd_leave` / `talos_etcd_remove_member` are needed **only for non-graceful resets**
or nodes that are already gone. A graceful `talos_reset` performs the leave itself, and calling
these first is unnecessary. A member that is neither left nor removed keeps counting toward quorum
even though it will never vote again — a three-member cluster with one such ghost has no fault
tolerance left.

## Disaster recovery

### 1. Try to restore quorum first

Far simpler than a full recovery, and usually possible. Check `talos_etcd_members` and
`talos_etcd_status` to identify which members are actually gone, bring back the ones that can be
brought back, and remove the ones that cannot. A cluster that regains a majority recovers by
itself.

### 2. Restore from a snapshot

When quorum cannot be restored:

1. Wipe the EPHEMERAL partition on the recovery node:
   `talos_reset(node=..., graceful=false, reboot=true, system_labels_to_wipe="EPHEMERAL")`
   With etcd on a dedicated partition in v1.14, wiping only EPHEMERAL no longer clears etcd data — wipe the etcd volume as well.
2. Wait for the etcd service to report **Preparing**.
3. `talosctl bootstrap --recover-from=<snapshot>` (Bash) against that node.
4. Add `--recover-skip-hash-check` when the snapshot was **copied off disk** rather than produced by `talos_etcd_snapshot`. A raw copy has no matching integrity hash and the restore refuses it otherwise.

### 3. When quorum is lost and a snapshot cannot be taken

`talos_etcd_snapshot` needs a working etcd. With quorum gone, copy the database file straight off a
surviving member instead:

```bash
talosctl -n <node> cp /var/lib/etcd/member/snap/db .
```

Restore that with `--recover-from` plus `--recover-skip-hash-check`.

### Single vs multiple control planes

- **Single control plane:** reset the node, re-apply its machine config, and bootstrap. With a snapshot, recover from it; with no snapshot and no etcd data, the cluster is gone and has to be rebuilt.
- **Multiple control planes:** restore on **one** node only. The others rejoin the restored etcd automatically. Restoring on more than one produces separate clusters.

### Keep the secrets

`secrets.yaml` from `talosctl gen secrets`, the generated machine configs and the talosconfig
**cannot be regenerated**. Without them a snapshot is not restorable — the CA that signs every
certificate in the cluster lives there. Store them the way private keys are stored, off the
cluster.
