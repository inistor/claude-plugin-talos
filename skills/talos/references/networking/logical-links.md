# Logical links: bond, bridge, VLAN, VRF, dummy, veth, WireGuard (Talos v1.14)

Field reference: https://docs.siderolabs.com/talos/v1.14/reference/configuration/network/

Every logical link is its own config document named after the interface it creates, and every one of
them accepts the *common link settings* (`up`, `mtu`, `addresses`, `routes`, `multicast`) described
in `references/networking/links.md`. Member links are given by kernel name or by a `LinkAliasConfig`
alias. Creating any of these documents disables default DHCP everywhere — see the trap in
`links.md`.

## Bond

```yaml
apiVersion: v1alpha1
kind: BondConfig
name: bond0
links:
    - enp1s0
    - enp2s0
bondMode: 802.3ad
lacpRate: fast
xmitHashPolicy: layer3+4
miimon: 100
updelay: 200
downdelay: 200
up: true
addresses:
    - address: 10.0.0.2/24
routes:
    - gateway: 10.0.0.1
```

`bondMode` drives everything else and is one of `balance-rr` (the default when unset),
`active-backup`, `balance-xor`, `broadcast`, `802.3ad`, `balance-tlb`, `balance-alb`. Use `802.3ad`
with a switch configured for LACP, `active-backup` when the two ports go to switches that are not
stacked.

The remaining two dozen tunables (`arpInterval`, `arpIpTargets`, `arpValidate`, `failOverMac`,
`adSelect`, `primaryReselect`, `minLinks`, `missedMax`, …) pass straight through to the kernel
bonding driver and keep the kernel default when unset, so set only what the deployment actually
needs. `hardwareAddr` pins the bond MAC.

## Bridge

`BridgeConfig` takes `links:` plus `stp.enabled` and `vlan.filtering`, and the usual common link
settings for addressing the bridge itself.

## VLAN

One document per VLAN. **The document name is the resulting interface name** — it is not derived
from `parent` and `vlanID`, so a mismatch produces a working interface with a confusing name.

```yaml
apiVersion: v1alpha1
kind: VLANConfig
name: enp0s3.100
vlanID: 100
vlanMode: 802.1q      # or 802.1ad
parent: enp0s3
up: true
addresses:
    - address: 10.100.0.2/24
routes:
    - destination: 10.100.0.0/24
      gateway: 10.100.0.1
```

## VRF

`VRFConfig` enslaves the links in `links:` to a VRF device. **`table` is a string**, not an integer
(`table: "123"`). A `VethConfig` peer endpoint can be listed in `links:` too, which is the way to
move one end of a veth pair into a VRF.

## Dummy link

`DummyLinkConfig` creates a dummy interface with an optional `hardwareAddr` and the common link
settings. The usual reason to create one is a stable loopback-style `/32` or `/128` address for BGP
to originate — list the dummy in `BGPInstanceConfig.advertise` (see `bgp-and-vips.md`).

## Veth pairs (new in 1.14)

`VethConfig` creates a pair: the document names one end, `peer.name` the other, and `peer` accepts
the same common link settings (`up`, `mtu`, `addresses`, `routes`, `multicast`) as the parent.

**Both endpoints are created in the host network namespace** — this is not a container-style veth.
Names must differ from each other, must be literal kernel names (aliases are rejected, since the
document creates the link rather than selecting it), at most **15 bytes**, and must contain neither
`/` nor `:` nor whitespace.

## WireGuard

```yaml
apiVersion: v1alpha1
kind: WireguardConfig
name: wg0
privateKey: <base64, from `wg genkey`>
listenPort: 51820
firewallMark: 176
peers:
    - publicKey: 735jkJdcVDninU5PzLJ/S+bfN6Q3QOk6svWrVLMJQAk=
      allowedIPs:
        - 192.168.1.0/24
    - publicKey: uvdlJNva1X8/OCOZM+0gGT4Yu9x20odd3AWbbQUF7nM=
      presharedKey: <base64, from `wg genpsk`>
      endpoint: 10.3.4.3:2222
      persistentKeepaliveInterval: 25s
up: true
mtu: 1420
addresses:
    - address: 192.168.1.100/32
```

For a mesh across sites, prefer KubeSpan (`bgp-and-vips.md`) over hand-managed WireGuard peers.
