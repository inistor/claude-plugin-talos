# BGP, virtual IPs and KubeSpan (Talos v1.14)

Field reference: https://docs.siderolabs.com/talos/v1.14/reference/configuration/network/

## BGP (new in 1.14)

Talos embeds GoBGP, so fabric peering no longer needs the FRR extension. `BGPInstanceConfig`
documents are named and repeatable — one per routing instance.

```yaml
apiVersion: v1alpha1
kind: BGPInstanceConfig
name: fabric
localASN: 65001
routerID: 10.0.0.1        # must be IPv4; derived from the first advertised address if unset
advertise:
    - dummy0              # link names/aliases whose addresses are originated as connected networks
multipath: true           # ECMP over routes learned from several neighbours
maxPaths: 4
installRoutes: false      # default true; false keeps routes in the BGP RIB only
importRoutes:
    - bgpInstance: workload
      prefixes:
        - 198.51.100.0/24
neighbors:
    - address: 10.5.0.1   # numbered session
      peerASN: 65000
      passive: true
      holdTime: 9s
    - link: enp0s1        # unnumbered (IPv6 link-local, RFC 8950 extended next-hop)
      peerASN: 65000
      localASN: 65002     # per-neighbour local ASN override
      holdTime: 9s
      bfd:
        transmitInterval: 300ms
        receiveInterval: 300ms
        detectMultiplier: 3
```

- `vrf:` binds the instance to a Linux VRF link; unset means the default routing domain.
- `routeSource:` sets the preferred source address (`RTA_PREFSRC`) on installed routes — the
  equivalent of FRR's `ip protocol bgp route-map SETSRC`.
- A neighbour takes **either** `address` **or** `link`, never both. `peerASN: 0` accepts any ASN.
- The presence of a `bfd:` block enables BFD; an empty block takes the defaults. **BFD is rejected on
  a VRF-bound instance** — GoBGP's embedded BFD listener is not VRF-aware.
- `importRoutes` is one-way: best paths learned by the named source instance and covered by one of
  the `prefixes` are re-advertised by this instance with its own next hop. Locally originated and
  already-imported paths are not recursively imported, a source instance may appear only once, and
  prefixes must not overlap across entries.
- A config with no neighbours validates with a warning, not an error.
- Session state: `talosctl get bgppeerstatus`.

## Virtual IPs

Both VIP documents are named by the **IP address** and take a `link`. The VIP is claimed through
etcd leader election, so it works only on control plane nodes.

```yaml
apiVersion: v1alpha1
kind: Layer2VIPConfig
name: 10.0.0.100
link: net0
```

`HCloudVIPConfig` is the Hetzner Cloud floating-IP variant — same `name` and `link`, plus
`apiToken`; it moves the IP through the HCloud API instead of by gratuitous ARP.

## KubeSpan

Mesh WireGuard across sites. Requires cluster discovery (`DiscoveryServiceConfig`).

```yaml
apiVersion: v1alpha1
kind: KubeSpanConfig
enabled: true
advertiseKubernetesNetworks: false
allowDownPeerBypass: false
harvestExtraEndpoints: false
mtu: 1420
filters:
    excludeAdvertisedNetworks:
        - 2007::/64
    endpoints:
        - 0.0.0.0/0
        - "!192.168.0.0/16"
        - ::/0
```

`excludeAdvertisedNetworks` and `endpoints` nest under **`filters:`**, not at the top level — the
pre-rewrite documentation had them at the top and such a config is silently ignored.

- `advertiseKubernetesNetworks` — when true, KubeSpan carries pod-to-pod traffic directly instead of
  the CNI encapsulating it.
- `allowDownPeerBypass` — send traffic outside KubeSpan while a peer is down (availability over
  confidentiality).
- `harvestExtraEndpoints` — collect and publish peer-observed WireGuard endpoints. Off by default;
  leave it off above roughly 50 peers.
- `filters.excludeAdvertisedNetworks` — CIDRs not advertised over KubeSpan. Excluded networks must
  stay reachable by another route, and the exclusions must be symmetric between any pair of peers.
- `filters.endpoints` — which local addresses are advertised as KubeSpan endpoints; an entry
  prefixed with `!` is an exclusion.
- `mtu` defaults to 1420.

Endpoints Talos cannot discover itself, such as NAT mappings, go in a separate document:

```yaml
apiVersion: v1alpha1
kind: KubeSpanEndpointsConfig
extraAnnouncedEndpoints:
    - 3.4.5.6:123
    - 10.11.12.13:456
```
