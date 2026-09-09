# Networking Reference (Talos v1.14)

Docs: https://docs.siderolabs.com/talos/v1.14/reference/configuration/network/

Talos v1.13 deprecated **all** of `machine.network.*`. In v1.14 networking is configured with
standalone multi-document config documents, one document per object, concatenated into the machine
config with `---`. Everything below is written in that style.

```yaml
version: v1alpha1   # v1alpha1 Config document (machine:, cluster:, ...)
machine:
  type: controlplane
  # no `network:` section
---
apiVersion: v1alpha1
kind: LinkConfig
name: enp0s1
addresses:
  - address: 10.0.0.2/24
```

Every network document uses `apiVersion: v1alpha1` and a `kind`. Most are *named* (`name:`), and the
name is usually the link name, the IP, or the rule priority — see each section.

## Migration: old v1alpha1 field → document

| Deprecated v1alpha1 field | Replacement document |
|---|---|
| `.machine.network.interfaces[].addresses/routes/mtu` | `LinkConfig` |
| `.machine.network.interfaces[].deviceSelector` | `LinkAliasConfig` (CEL) + name the alias |
| `.machine.network.interfaces[].dhcp` / `dhcpOptions` | `DHCPv4Config`, `DHCPv6Config` |
| `.machine.network.interfaces[].bond` | `BondConfig` |
| `.machine.network.interfaces[].bridge` | `BridgeConfig` |
| `.machine.network.interfaces[].vlans[]` | `VLANConfig` |
| `.machine.network.interfaces[].wireguard` | `WireguardConfig` |
| `.machine.network.interfaces[].vip` | `Layer2VIPConfig` / `HCloudVIPConfig` |
| `.machine.network.hostname` | `HostnameConfig` |
| `.machine.network.nameservers`, `.searchDomains`, `.disableSearchDomain` | `ResolverConfig` |
| `.machine.network.extraHostEntries` | `StaticHostConfig` |
| `.machine.network.kubespan` | `KubeSpanConfig` |
| `.machine.features.hostDNS` | `ResolverConfig.hostDNS` |
| `.machine.time` | `TimeSyncConfig` |
| `.machine.sysctls` | `SysctlConfig` |
| `.cluster.network` | `KubeNetworkConfig`, `KubeFlannelCNIConfig` |
| `.machine.features.kubePrism` | `KubePrismConfig` |

The old fields still parse for backward compatibility, but do not mix styles on the same link.

### Critical behaviour change when you adopt the new documents

Talos runs a **default DHCPv4 operator on every physical link** that has no explicit configuration.
That default is switched off *globally* — not per link — as soon as the machine config contains **any**
link document (`LinkConfig`, `BondConfig`, `BridgeConfig`, `VLANConfig`, `VRFConfig`, `VethConfig`,
`DummyLinkConfig`, `WireguardConfig`) **or any** `DHCPv4Config`/`DHCPv6Config`.

So the moment you add one `LinkConfig` for a static NIC, every other NIC stops getting DHCP and stops
being brought up. Add an explicit `DHCPv4Config` for each link that should still use DHCP.

Under the deprecated v1alpha1 style the suppression was per-interface, so this is the most common
migration trap.

## Selecting links

There is no `deviceSelector` in the new documents. Documents reference links **by name**: either the
kernel name (`enp0s1`) or an alias you define.

### LinkAliasConfig — stable names via CEL

```yaml
apiVersion: v1alpha1
kind: LinkAliasConfig
name: net0
selector:
    match: mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"
```

The selector is a CEL boolean expression over the `LinkStatus` resource, exposed as `link`. Two helper
functions are registered:

- `mac(bytes) -> string` — formats a hardware address as `aa:bb:cc:dd:ee:ff`
- `glob(pattern, string) -> bool` — pattern first, value second

Fields available on `link` (from `LinkStatus`; run `talosctl get links -o yaml` to see real values):
`permanent_addr`, `hardware_addr`, `alt_names`, `driver`, `driver_version`, `firmware_version`,
`bus_path`, `pciid`, `vendor`, `vendor_id`, `product`, `product_id`, `type`, `kind`, `slave_kind`,
`mtu`, `link_state`, `speed_megabits`, `port`, `duplex`, `operational_state`, `alias`, `index`.

Prefer `permanent_addr` (the burned-in MAC) over `hardware_addr` — the latter changes when a link is
enslaved to a bond with `failOverMac`.

Common selectors:

```yaml
match: mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"   # exact MAC
match: glob("00:1a:2b:*", mac(link.permanent_addr))      # MAC prefix
match: link.driver == "e1000"                            # by driver
match: "enx728c41bfd443" in link.alt_names               # by altname
match: glob("0000:00:1f.*", link.bus_path)               # by PCI bus path
```

Numbered aliases (`%d`) let one document name a whole class of NICs. The selector may then match
multiple links, and each receives a sequential alias ordered by hardware address; links already
aliased by an earlier document are skipped:

```yaml
apiVersion: v1alpha1
kind: LinkAliasConfig
name: net%d           # -> net0, net1, net2 ...
selector:
    match: link.driver == "mlx5_core"
```

Aliases can be used anywhere a link name is expected: `LinkConfig.name`, `BondConfig.links`,
`BridgeConfig.links`, `VLANConfig.parent`, `VRFConfig.links`, `Layer2VIPConfig.link`,
`BGPInstanceConfig.advertise` / `neighbors[].link`, `DHCPv4Config.name`, `EthernetConfig.name`.

Aliases are **not** accepted for `VethConfig.name`/`peer.name` — veth endpoints are created by the
document, not selected, so those must be literal kernel names (≤ 15 bytes, no `/` or `:`).

## Physical links — LinkConfig

```yaml
apiVersion: v1alpha1
kind: LinkConfig
name: enp0s1
up: true
mtu: 9000
addresses:
    - address: 192.168.1.100/24
    - address: 2001:db8::1/64
      routePriority: 100
routes:
    - destination: 10.3.5.0/24
      gateway: 10.3.5.1
    - gateway: fe80::1          # no destination -> default route for that family
multicast: true
```

`addresses[]`: `address` (required, must carry a prefix length), `routePriority` (metric for the
routes derived from this address).

`routes[]`: `destination`, `gateway`, `source`, `metric`, `mtu`, `table`. Omitting `destination`
creates a default route for the gateway's address family; omitting `gateway` creates a link-scope
route.

`up`, `mtu`, `addresses`, `routes`, `multicast` are the *common link settings* — they are shared by
`LinkConfig`, `BondConfig`, `BridgeConfig`, `VLANConfig`, `VRFConfig`, `VethConfig` (and its `peer`),
`DummyLinkConfig` and `WireguardConfig`.

Only one link document may own a given link name — the link kinds conflict with each other.

## DHCP

```yaml
apiVersion: v1alpha1
kind: DHCPv4Config
name: enp0s3
routeMetric: 512          # default 1024
ignoreHostname: true
ignoreRoutes: true        # NEW in 1.14 - drop default gw + classless static routes
clientIdentifier: duid    # none | mac (default) | duid
duidRaw: 00:01:00:01:23:45:67:89:ab:cd:ef:01:23:45
---
apiVersion: v1alpha1
kind: DHCPv6Config
name: enp0s3
clientIdentifier: mac
```

`ignoreRoutes` (new in 1.14) suppresses routes offered by the DHCP server while keeping the leased
address and its connected route — useful when a `LinkConfig` or BGP supplies the default route.

DHCPv6 has no `ignoreRoutes`. Both take `routeMetric`, `ignoreHostname`, `clientIdentifier`,
`duidRaw` (`duidRaw` is only read when `clientIdentifier: duid`).

Also new in 1.14: DHCPv4 search domains are applied to the resolver configuration.

## Logical links

### Bond

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

`bondMode`: `balance-rr`, `active-backup`, `balance-xor`, `broadcast`, `802.3ad`, `balance-tlb`,
`balance-alb`.

Other tunables (all optional, kernel defaults apply when unset): `hardwareAddr`, `useCarrier`,
`arpInterval`, `arpIpTargets`, `nsIp6Targets`, `arpValidate`, `arpAllTargets`, `failOverMac`,
`adSelect`, `adActorSysPrio`, `adUserPortKey`, `adLACPActive`, `primaryReselect`, `resendIGMP`,
`minLinks`, `lpInterval`, `packetsPerSlave`, `numPeerNotif`, `tlbLogicalLb`, `allSlavesActive`,
`peerNotifDelay`, `missedMax`.

### Bridge

```yaml
apiVersion: v1alpha1
kind: BridgeConfig
name: br0
links:
    - eno1
    - eno5
stp:
    enabled: true
vlan:
    filtering: false
up: true
addresses:
    - address: 10.0.0.2/24
```

### VLAN

One document per VLAN. The name is the resulting interface name; `parent` is the underlying link.

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

### VRF

```yaml
apiVersion: v1alpha1
kind: VRFConfig
name: vrf-blue
links:
    - eno1
    - eno5
table: "123"
up: true
addresses:
    - address: 1.2.3.5/32
```

`table` is a string. Links listed here are enslaved to the VRF; a `VethConfig` peer endpoint can be
named here too.

### Dummy link

```yaml
apiVersion: v1alpha1
kind: DummyLinkConfig
name: dummy0
hardwareAddr: 2e:3c:4d:5e:6f:70
up: true
addresses:
    - address: 192.168.1.100/32
```

Typical use: a stable loopback-style address for BGP to originate (see `BGPInstanceConfig.advertise`).

### Veth pairs (new in 1.14)

```yaml
apiVersion: v1alpha1
kind: VethConfig
name: veth-metallb
up: true
mtu: 1500
addresses:
    - address: fda1::1/127
peer:
    name: veth-router
    mtu: 1400
    addresses:
        - address: fda1::/127
```

**Both endpoints are created in the host network namespace** — this is not a container-style veth.
The peer supports the same common link settings (`up`, `mtu`, `addresses`, `routes`, `multicast`).
Name and peer name must differ, each ≤ 15 bytes, literal kernel names only. To place the peer in a
VRF, list `veth-router` in a `VRFConfig`'s `links`.

### WireGuard

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

## Routing

Ordinary routes live in the `routes:` block of the owning link document. Two dedicated documents cover
the rest.

### BlackholeRouteConfig

The document name *is* the destination prefix.

```yaml
apiVersion: v1alpha1
kind: BlackholeRouteConfig
name: 169.254.1.1/32
metric: 2000
```

### RoutingRuleConfig (`ip rule`)

The document name *is* the rule priority, as a string. Must be 1–32765 and unique; priorities
0, 32500, 32501, 32766 and 32767 are reserved.

```yaml
apiVersion: v1alpha1
kind: RoutingRuleConfig
name: "1000"
src: 10.0.0.0/8
dst: 192.168.0.0/16
table: "100"
action: unicast        # unicast (default) | blackhole | unreachable | prohibit
iifName: eth0
oifName: eth1
fwMark: 256
fwMask: 65280
```

## BGP (new in 1.14)

Talos embeds GoBGP, so fabric peering no longer needs the FRR extension. Documents are named and
repeatable — one per routing instance.

```yaml
apiVersion: v1alpha1
kind: BGPInstanceConfig
name: fabric
localASN: 65001
routerID: 10.0.0.1        # must be IPv4; derived from first advertised address if unset
advertise:
    - dummy0              # link names/aliases whose addresses are originated as connected networks
multipath: true           # ECMP for routes learned from multiple neighbors
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
      localASN: 65002     # per-neighbor local ASN override
      holdTime: 9s
      bfd:
        transmitInterval: 300ms
        receiveInterval: 300ms
        detectMultiplier: 3
```

Notes:

- `vrf:` binds the instance to a Linux VRF link; unset uses the default routing domain.
- `routeSource:` sets the preferred source address (`RTA_PREFSRC`) on installed routes — the
  equivalent of FRR's `ip protocol bgp route-map SETSRC`.
- A neighbor takes **either** `address` **or** `link`, not both. `peerASN: 0` accepts any ASN.
- Presence of the `bfd:` block enables BFD; an empty block uses defaults. **BFD only works when the
  instance is in the default routing domain** — GoBGP's embedded BFD listener is not VRF-aware.
- `importRoutes` is one-way: best paths learned by the named source instance and contained by one of
  the `prefixes` are re-advertised by this instance with its own next hop. Locally originated and
  already-imported paths are not recursively imported, a source instance may appear once, and
  selectors across entries must not overlap.
- Peer state: `talosctl get bgppeerstatus`.
- A config with no neighbors validates with a warning, not an error.

## Virtual IPs

Both VIP documents are named by the **IP address** and take a `link`. The VIP uses etcd leader
election, so it only works on control plane nodes.

```yaml
apiVersion: v1alpha1
kind: Layer2VIPConfig
name: 10.0.0.100
link: net0
```

Hetzner Cloud floating IP (moves the IP via the HCloud API instead of gratuitous ARP):

```yaml
apiVersion: v1alpha1
kind: HCloudVIPConfig
name: 1.2.3.4
link: net33
apiToken: s3cr3t-t0k3n
```

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

`excludeAdvertisedNetworks` and `endpoints` are nested under **`filters:`**, not at the top level.

- `advertiseKubernetesNetworks` — when true KubeSpan carries pod-to-pod traffic directly instead of
  the CNI encapsulating it.
- `allowDownPeerBypass` — send traffic outside KubeSpan when a peer is not up (availability over
  confidentiality).
- `harvestExtraEndpoints` — collect and publish peer-observed WireGuard endpoints. Off by default;
  do not enable above ~50 peers.
- `filters.excludeAdvertisedNetworks` — CIDRs not advertised over KubeSpan. Excluded networks must
  stay reachable by another route, and exclusions must be symmetric between any pair of peers.
- `filters.endpoints` — which local addresses are advertised as KubeSpan endpoints; entries prefixed
  with `!` are exclusions.
- `mtu` defaults to 1420.

Extra endpoints that Talos cannot discover (NAT mappings, for example):

```yaml
apiVersion: v1alpha1
kind: KubeSpanEndpointsConfig
extraAnnouncedEndpoints:
    - 3.4.5.6:123
    - 10.11.12.13:456
```

## Ingress firewall

Two documents. `NetworkDefaultActionConfig` sets the chain policy; `NetworkRuleConfig` (note the
capital `N` — the kind is **not** `networkRuleConfig`) adds rules. Rules are host-wide, matched on
destination port plus source subnet, not per interface.

```yaml
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block            # accept (default) | block
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: kubelet-ingress
portSelector:
    ports:
        - 10250
    protocol: tcp
ingress:
    - subnet: 172.20.0.0/24
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: dns-and-highports
portSelector:
    ports:
        - 53
        - 8000-9000       # inclusive range, as a string
    protocol: udp
ingress:
    - subnet: 192.168.0.0/16
      except: 192.168.0.3/32
    - subnet: 2001::/16
```

Semantics:

- `ingress: block` — chain policy is drop, and each `NetworkRuleConfig` is an **allowlist**.
- `ingress: accept` — chain policy is accept, and each `NetworkRuleConfig` inverts into a
  **blocklist**: traffic to those ports is dropped unless it comes from the listed subnets.
- `protocol`: `tcp`, `udp`, `icmp`, `icmpv6`. `ports` accepts bare integers and `lo-hi` strings;
  ranges must not overlap within one rule.
- `except` carves a prefix out of the `subnet` above it.

With `ingress: block` Talos implicitly allows: established/related conntrack (and drops invalid),
traffic on trusted interfaces (loopback, SideroLink, KubeSpan), ICMP and ICMPv6 rate-limited to
5 pps, and pod/service CIDR traffic to the host DNS address on port 53 when
`hostDNS.forwardKubeDNSToHost` is on. It also drops ICMP timestamp/address-mask requests
(CVE-1999-0524). **Nothing else is opened for you** — you must add rules for the ports your cluster
needs:

| Port | Purpose |
|---|---|
| 50000/tcp | Talos API (apid) |
| 50001/tcp | trustd (control plane) |
| 6443/tcp | kube-apiserver |
| 2379/tcp | etcd client (control plane) |
| 2380/tcp | etcd peer (control plane) |
| 2383/tcp | etcd HTTP metrics/health — **new in 1.14**, moved off 2379 |
| 10250/tcp | kubelet |
| 4789/udp | Flannel VXLAN (default backend port) |
| 51820/udp | KubeSpan WireGuard (default) |
| 179/tcp | BGP (default) |

Note the 1.14 etcd split: `2379` now serves gRPC only and the HTTP endpoints (`/metrics`, `/health`,
gRPC-gateway JSON) moved to `2383`. Existing firewall rules that blocked `2379` need a matching rule
for `2383`.

Inspect the compiled chains with `talosctl get nftableschains -o yaml`.

## DNS

### ResolverConfig

```yaml
apiVersion: v1alpha1
kind: ResolverConfig
nameservers:
    - address: 10.0.0.1
    - address: 2001:4860:4860::8888
      protocol: DoT               # Do53 (default) | DoT | DoH
      tlsServerName: dns.google
searchDomains:
    domains:
        - example.org
        - example.com
    disableDefault: false
hostDNS:
    enabled: true
    forwardKubeDNSToHost: true
    resolveMemberNames: false
```

- Defaults are `1.1.1.1` and `8.8.8.8`. Setting `nameservers` **overwrites** lower layers (DHCP,
  platform, defaults) rather than merging — if you list only IPv4 servers, platform IPv6 servers are
  dropped. Configure both families explicitly when you need both.
- `searchDomains.domains: []` (explicit empty list) clears DHCP/platform search domains; leaving the
  field unset inherits them. `disableDefault: true` stops deriving a search domain from the hostname
  FQDN.

**Encrypted DNS (new in 1.14).** `protocol: DoT` is DNS over TLS (RFC 7858, TCP/853); `protocol: DoH`
is DNS over HTTPS (RFC 8484, TCP/443, path `/dns-query`). `tlsServerName` is required for both — it is
the SNI, the name verified against the certificate, and for DoH the host portion of the request URL
(`https://<tlsServerName>/dns-query`), while the connection still goes to `address`. It must be empty
for `Do53`.

> Certificate validation needs a correct clock. If every nameserver is DoT/DoH **and** your NTP
> servers are hostnames, the boot can stall: NTP needs DNS, and TLS needs valid time. Talos emits a
> validation warning for this. Keep one plain-DNS fallback nameserver, configure NTP servers by IP,
> or rely on a working hardware clock.

**Host DNS** (moved in 1.14 from `.machine.features.hostDNS`) is a caching resolver on the node at
`169.254.116.108` (IPv6: `fd54:616c:6f73::204f:5320:444e:531`), with the `nameservers` above as
upstreams. `forwardKubeDNSToHost` points CoreDNS at it instead of at the upstreams directly.
`resolveMemberNames` resolves cluster member hostnames locally and needs cluster discovery.
`forwardKubeDNSToHost` and `resolveMemberNames` are rejected when `enabled: false`.

### StaticHostConfig (`/etc/hosts` entries)

The document name is the IP address.

```yaml
apiVersion: v1alpha1
kind: StaticHostConfig
name: 10.5.0.2
hostnames:
    - example.org
    - example.com
```

## Hostname

```yaml
apiVersion: v1alpha1
kind: HostnameConfig
hostname: controlplane1.example.org
```

or automatic generation:

```yaml
apiVersion: v1alpha1
kind: HostnameConfig
auto: stable        # stable | off
```

`hostname` has the highest priority over DHCP/cloud-init; `auto` has the lowest. `stable` derives a
hostname from machine identity; `off` waits for an external source. The two fields conflict — set one.

## Time synchronization

```yaml
apiVersion: v1alpha1
kind: TimeSyncConfig
enabled: true
bootTimeout: 1m0s
ntp:
    servers:
        - time.cloudflare.com
    useNTS: true
```

`bootTimeout` is how long the boot sequence waits for time sync (default: forever; sync continues in
the background either way).

**NTS (new in 1.14).** `ntp.useNTS` enables Network Time Security — authenticated, encrypted NTP over
TLS. It defaults to `true` when no `TimeSyncConfig` is supplied at all (with the default server
`time.cloudflare.com`). With NTS enabled every NTP server must be given as a **hostname**, not an IP —
which interacts with the DoT/DoH warning above.

PTP is mutually exclusive with NTP:

```yaml
apiVersion: v1alpha1
kind: TimeSyncConfig
ptp:
    devices:
        - /dev/ptp_kvm
```

## Network probes

Probes let you gate readiness on a reachability condition of your choosing.

```yaml
apiVersion: v1alpha1
kind: TCPProbeConfig
name: proxy-check
endpoint: proxy.example.com:3128
interval: 1s
failureThreshold: 3
timeout: 10s
---
apiVersion: v1alpha1
kind: HTTPProbeConfig       # new in 1.14
name: http-check
url: https://example.com
interval: 1s
failureThreshold: 3
timeout: 10s
```

HTTP probes succeed on 2xx/3xx and do not follow redirects; connection and transport errors count as
failures. `interval` defaults to 1s, `timeout` to 10s, `failureThreshold` to 0 (fail on first
failure). Results: `talosctl get probestatuses`.

## Ethernet tuning

```yaml
apiVersion: v1alpha1
kind: EthernetConfig
name: enp0s1
features:
    tx-checksum-ipv4: true
rings:
    rx: 16
channels:
    combined: 1
wakeOnLan:
    - unicast
    - multicast
```

`features` is the `ethtool -K` surface, `rings` is `ethtool -G`
(`rx`, `tx`, `rx-mini`, `rx-jumbo`, `rx-buf-len`, `cqe-size`, `tx-push`, `rx-push`,
`tx-push-buf-len`, `tcp-data-split`), `channels` is `ethtool -L` (`rx`, `tx`, `other`, `combined`),
`wakeOnLan` is `ethtool -s ... wol` (`phy`, `unicast`, `multicast`, `broadcast`, `arp`, `magic`,
`magicsecure`, `filter`; an empty list disables WoL, omitting the field leaves it unchanged).

Available features are driver specific — discover them with
`talosctl get ethernetstatus <link> -o yaml`.

## Cluster (Kubernetes) networking

```yaml
apiVersion: v1alpha1
kind: KubeNetworkConfig
dnsDomain: cluster.local
podSubnets:
    - 10.244.0.0/16
serviceSubnets:
    - 10.96.0.0/12
nodeCIDRMaskSizeIPv4: 24
nodeCIDRMaskSizeIPv6: 64
---
apiVersion: v1alpha1
kind: KubeFlannelCNIConfig
backendType: vxlan
backendPort: 4789
backendMTU: 1420
kubeNetworkPoliciesEnabled: true
extraArgs:
    - --iface-can-reach=192.168.1.1
```

`kubeNetworkPoliciesEnabled` deploys
[kube-network-policies](https://github.com/kubernetes-sigs/kube-network-policies) alongside Flannel
for NetworkPolicy enforcement. If the cluster is already running, sync the bootstrap manifests after
applying the patch.

KubePrism (the local control plane load balancer):

```yaml
apiVersion: v1alpha1
kind: KubePrismConfig
port: 7445
```

Remove the document to disable KubePrism. `tlsServerName` overrides the SNI in the generated kubelet
kubeconfig, for setups where KubePrism's upstream is behind an SNI-routing L4 proxy.

## Other v1.14 behaviour changes worth knowing

- **ICMP redirects off by default.** Talos now sets
  `net.ipv4.conf.all.send_redirects=0` and `net.ipv4.conf.default.send_redirects=0` (CIS Benchmark).
  Normal pod and service traffic is unaffected, but a node deliberately acting as an L3 gateway that
  relies on ICMP redirects will break. Restore with:

  ```yaml
  apiVersion: v1alpha1
  kind: SysctlConfig
  params:
      net.ipv4.conf.all.send_redirects: "1"
      net.ipv4.conf.default.send_redirects: "1"
  ```

- **Flannel runs with `EnableNFTables`**, using the native nftables backend instead of the
  `iptables-nft` compatibility layer.
- **`talosctl apply-config --mode=reboot` was removed.** Config now applies without a reboot by
  default; the docs list the changes that still need one.
- DHCPv4 search domains now feed the resolver configuration.

## Diagnostics

MCP tools:

- `talos_addresses` — assigned IP addresses
- `talos_routes` — routing table
- `talos_interfaces` — interface status (up/down, MTU, ...)
- `talos_netstat` — active connections and listeners
- `talos_resolvers` — configured DNS resolvers
- `talos_hostname` — node hostname
- `talos_time` — NTP sync status
- `talos_get <resource>` — any resource below

Useful resources (`talosctl get <type>` / `talos_get`):

| Resource | Shows |
|---|---|
| `links` / `linkstatuses` | link state, MAC, driver, PCI path, altnames — the input to CEL selectors |
| `linkaliasspecs` | which alias resolved to which link |
| `addressstatuses`, `routestatuses` | live addresses and routes |
| `nodeaddresses` | addresses Talos considers node addresses |
| `routingrulestatuses` | active `ip rule` entries |
| `resolverstatuses`, `dnsupstreams`, `hostdnsconfigs` | effective DNS configuration |
| `timeserverstatuses` | effective NTP servers |
| `probestatuses` | TCP/HTTP probe results |
| `bgppeerstatuses` | BGP session state per instance |
| `ethernetstatuses` | ethtool features/rings/channels available on a link |
| `nftableschains` | compiled ingress firewall chains |
| `operatorspecs` | which DHCP/VIP operators are running (add `-n network-config` to see the per-layer inputs before merging) |
