# Scaling and resetting nodes

## Adding a worker

Boot the new machine so it lands in maintenance mode, then apply the **same** `worker.yaml`
generated for the cluster:

```
talos_apply_config(config=<worker.yaml>, node="10.0.0.30", insecure=true)
```

The node installs, reboots, and joins by itself — the config already carries the cluster CA, the
control-plane endpoint and the join token, and discovery does the rest. There is no separate join
command, and nothing has to be registered on the control plane first.

If the worker config was generated a while ago, check it still matches the cluster before applying
it: a rotated CA or a changed endpoint makes an old `worker.yaml` fail with a certificate error
rather than joining. Confirm the result with `talos_health` and a Kubernetes node list.

Adding a control-plane node works the same way with `controlplane.yaml`. The new member joins the
existing etcd automatically — **do not** run `talos_bootstrap` on it (see
`references/operations/bootstrap.md`).

## Removing a node

```
talos_reset(node="10.0.0.30", graceful=true)
```

A graceful reset cordons and drains the node, leaves etcd if it is a control-plane member, wipes
the disks and powers the machine down. It is destructive and unattended: the node does not come
back without a fresh config apply.

Then delete the Kubernetes node object, which the reset does not remove:

```
mcp__kubernetes-mcp-server__resources_delete(apiVersion="v1", kind="Node", name="worker-03")
```

## When etcd needs manual attention

A graceful reset of a control-plane node handles etcd departure on its own. Manual intervention —
`talos_etcd_forfeit_leadership`, `talos_etcd_leave`, `talos_etcd_remove_member` — is needed only
when that path was not taken:

- a **non-graceful** reset (`graceful=false`), which skips the leave entirely
- a node that is already dead or unreachable, so nothing can run on it
- a reset that failed partway, leaving a member listed but gone

In those cases the stale member must be removed from the outside, or it keeps counting toward
quorum. Procedure in `references/operations/etcd-and-recovery.md`.
