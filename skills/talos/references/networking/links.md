# Links and addressing (Talos v1.14)

Every field is documented upstream —
https://docs.siderolabs.com/talos/v1.14/reference/configuration/network/ — so this file carries the
traps and decisions, not the schema.

v1.13 deprecated **all** of `machine.network.*`. In v1.14 networking is standalone config documents,
one per object, concatenated with `---`. Each uses `apiVersion: v1alpha1` and a `kind`; most are
*named*, the name being the link name, an IP, or a rule priority.

## The trap: one link document disables DHCP on every other NIC

Talos runs a **default DHCPv4 operator on every physical link** that has no explicit configuration.
That default is switched off **globally — not per link** — as soon as the config contains **any**
link document (`LinkConfig`, `BondConfig`, `BridgeConfig`, `VLANConfig`, `VRFConfig`, `VethConfig`,
`DummyLinkConfig`, `WireguardConfig`) **or any** `DHCPv4Config` / `DHCPv6Config`. Default DHCP runs
only when the config holds zero link documents *and* zero DHCP documents.

So adding one `LinkConfig` for a single static NIC silently stops every other NIC from getting DHCP
and from being brought up at all — add an explicit `DHCPv4Config` for each link that should keep
using DHCP. The deprecated v1alpha1 style suppressed per interface, which makes this the most common
migration surprise.

## Migration: deprecated field → document

| Deprecated v1alpha1 field | Replacement document |
|---|---|
| `.machine.network.interfaces[].addresses/routes/mtu` | `LinkConfig` |
| `.machine.network.interfaces[].deviceSelector` | `LinkAliasConfig` (CEL) + reference the alias |
| `.machine.network.interfaces[].dhcp` / `dhcpOptions` | `DHCPv4Config`, `DHCPv6Config` |
| `.machine.network.interfaces[].bond` / `.bridge` / `.vlans[]` / `.wireguard` | `BondConfig`, `BridgeConfig`, `VLANConfig`, `WireguardConfig` |
| `.machine.network.interfaces[].vip` | `Layer2VIPConfig` / `HCloudVIPConfig` |
| `.machine.network.hostname` | `HostnameConfig` |
| `.machine.network.nameservers`, `.searchDomains`, `.disableSearchDomain` | `ResolverConfig` |
| `.machine.network.extraHostEntries` | `StaticHostConfig` |
| `.machine.network.kubespan` | `KubeSpanConfig` |

Old fields still parse, but do not mix styles on the same link. For everything outside
`machine.network` — `.machine.time`, `.machine.sysctls`, `.machine.features.*` — see
`references/config/config-migration.md`; for `.cluster.network` and KubePrism see
`references/config/kubernetes-documents.md`.

## Selecting links

There is no `deviceSelector`. Documents reference links **by name** — the kernel name (`enp0s1`) or
an alias.

### LinkAliasConfig — stable names via CEL

```yaml
apiVersion: v1alpha1
kind: LinkAliasConfig
name: net0
selector:
    match: mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"
```

The selector is a CEL boolean expression over the `LinkStatus` resource, exposed as `link`, with two
helpers registered: `mac(bytes) -> string` formats a hardware address as `aa:bb:cc:dd:ee:ff`, and
`glob(pattern, string) -> bool` takes **pattern first, value second** (the upstream CEL test has the
arguments reversed — follow the registration order).

`talosctl get links -o yaml` lists every field on `link` with real values. Prefer `permanent_addr`
(the burned-in MAC) over `hardware_addr`, which changes when a link is enslaved to a bond with
`failOverMac`.

```yaml
match: mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"   # exact MAC
match: glob("00:1a:2b:*", mac(link.permanent_addr))      # MAC prefix
match: link.driver == "e1000"                            # by driver
match: glob("0000:00:1f.*", link.bus_path)               # by PCI bus path
```

Numbered aliases (`%d`) let one document name a whole class of NICs: the selector may match several
links, each receiving a sequential alias ordered by hardware address, and links already aliased by
an earlier document are skipped. Without a format verb the selector must match exactly one link.

```yaml
apiVersion: v1alpha1
kind: LinkAliasConfig
name: net%d           # -> net0, net1, net2 ...
selector:
    match: link.driver == "mlx5_core"
```

An alias is applied as the kernel link alias, so it works anywhere a link name is expected:
`LinkConfig.name`, `BondConfig.links`, `BridgeConfig.links`, `VLANConfig.parent`, `VRFConfig.links`,
`Layer2VIPConfig.link`, `HCloudVIPConfig.link`, `BGPInstanceConfig.advertise` and `neighbors[].link`,
`DHCPv4Config.name`, `EthernetConfig.name`. Aliases are **not** accepted for `VethConfig.name` /
`peer.name` — veth endpoints are created by the document rather than selected, so those must be
literal kernel names.

## LinkConfig — physical links

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

`addresses[].address` must carry a prefix length; `routePriority` is the metric for routes derived
from it. In `routes[]`, omitting `destination` gives a default route for the gateway's address
family, omitting `gateway` gives a link-scope route.

`up`, `mtu`, `addresses`, `routes` and `multicast` are the *common link settings*, shared by
`LinkConfig`, `BondConfig`, `BridgeConfig`, `VLANConfig`, `VRFConfig`, `VethConfig` (and its `peer`),
`DummyLinkConfig` and `WireguardConfig`. Only one link document may own a given link name — the link
kinds conflict with each other.

## DHCP

```yaml
apiVersion: v1alpha1
kind: DHCPv4Config
name: enp0s3
routeMetric: 512          # default 1024
ignoreHostname: true
ignoreRoutes: true        # new in 1.14
clientIdentifier: duid    # none | mac (default) | duid
duidRaw: 00:01:00:01:23:45:67:89:ab:cd:ef:01:23:45
---
apiVersion: v1alpha1
kind: DHCPv6Config
name: enp0s3
clientIdentifier: mac
```

`ignoreRoutes` (new in 1.14, DHCPv4 only) drops the default gateway and classless static routes
offered by the server while keeping the leased address and its connected route — useful when a
`LinkConfig` or BGP supplies the default route. `duidRaw` is read only with `clientIdentifier: duid`.
Also new in 1.14: DHCPv4 search domains feed the resolver (`dns-and-time.md`).

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

`features` is `ethtool -K`, `rings` is `ethtool -G`, `channels` is `ethtool -L` and `wakeOnLan` is
`ethtool -s ... wol` (empty list disables WoL, omitting the field leaves it unchanged). Which knobs
exist is driver specific — discover them with `talosctl get ethernetstatus <link> -o yaml`.

## Routing documents

Ordinary routes live in the `routes:` block of the owning link document. Two named documents cover
the rest:

- **`BlackholeRouteConfig`** — the name *is* the destination prefix (`169.254.1.1/32`).
- **`RoutingRuleConfig`** (`ip rule`) — the name *is* the rule priority, as a **string**: 1–32765,
  unique, and not one of the reserved 0, 32500 (KubeSpan), 32501, 32766, 32767.

v1.14 behaviour changes that affect networking — `send_redirects=0`, Flannel on nftables, the etcd
2379→2383 move — are in `references/v1.14-changes.md`.
