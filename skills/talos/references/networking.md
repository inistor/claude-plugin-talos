# Networking Reference

Docs: https://docs.siderolabs.com/talos/v1.13/talos-guides/network/

## Interface Configuration

### Device Selection (Preferred)
```yaml
machine:
  network:
    interfaces:
      - deviceSelector:
          hardwareAddr: "00:11:22:*"    # glob pattern
        addresses:
          - 10.0.0.2/24
```

Selector fields: `hardwareAddr`, `busPath`, `pciID`, `driver`, `physical` (bool).

### Static Addressing
```yaml
machine:
  network:
    interfaces:
      - deviceSelector:
          hardwareAddr: "aa:bb:cc:*"
        addresses:
          - 10.0.0.2/24
          - fd00::2/64
        routes:
          - network: 0.0.0.0/0
            gateway: 10.0.0.1
          - network: ::/0
            gateway: fd00::1
        mtu: 9000
```

### DHCP
```yaml
machine:
  network:
    interfaces:
      - deviceSelector:
          hardwareAddr: "aa:bb:cc:*"
        dhcp: true
        dhcpOptions:
          routeMetric: 100
```

## Logical Interfaces

### Bond
```yaml
machine:
  network:
    interfaces:
      - interface: bond0
        bond:
          mode: 802.3ad
          lacpRate: fast
          xmitHashPolicy: layer3+4
          interfaces:
            - enp1s0
            - enp2s0
        addresses:
          - 10.0.0.2/24
```

Bond modes: `balance-rr`, `active-backup`, `balance-xor`, `broadcast`, `802.3ad`, `balance-tlb`, `balance-alb`.

### Bridge
```yaml
machine:
  network:
    interfaces:
      - interface: br0
        bridge:
          stp:
            enabled: true
          interfaces:
            - enp1s0
            - enp2s0
        addresses:
          - 10.0.0.2/24
```

### VLAN
```yaml
machine:
  network:
    interfaces:
      - deviceSelector:
          hardwareAddr: "aa:bb:cc:*"
        vlans:
          - vlanId: 100
            addresses:
              - 10.100.0.2/24
            routes:
              - network: 10.100.0.0/24
                gateway: 10.100.0.1
            dhcp: false
```

## Advanced Networking

### Virtual IP (VIP)
For HA control plane — shared IP that floats between CP nodes:
```yaml
machine:
  network:
    interfaces:
      - deviceSelector:
          hardwareAddr: "aa:bb:cc:*"
        addresses:
          - 10.0.0.2/24
        vip:
          ip: 10.0.0.100
```

### WireGuard
```yaml
machine:
  network:
    interfaces:
      - interface: wg0
        mtu: 1420
        wireguard:
          privateKey: <base64-key>
          listenPort: 51820
          peers:
            - publicKey: <peer-public-key>
              endpoint: peer.example.com:51820
              allowedIPs:
                - 10.10.0.0/24
              persistentKeepalive: 25
        addresses:
          - 10.10.0.1/24
```

### KubeSpan
Mesh networking across sites. **As of v1.13** the preferred configuration is the standalone `KubeSpanConfig` document; the legacy `.machine.network.kubespan` field still works for backward compatibility.

Legacy form (still works):
```yaml
machine:
  network:
    kubespan:
      enabled: true
      advertiseKubernetesNetworks: false
      mtu: 1420
```

v1.13 form (`KubeSpanConfig` document — apply alongside the v1alpha1 Config):
```yaml
apiVersion: v1alpha1
kind: KubeSpanConfig
# enabled, mtu, advertiseKubernetesNetworks, ...
# excludeAdvertisedNetworks: list of CIDRs to omit from advertisement (v1.13)
```

`excludeAdvertisedNetworks` filters which CIDRs are advertised to peers. Routing must be symmetric for any pair of peers — if one peer excludes a network, the other must too. See https://docs.siderolabs.com/talos/v1.13/reference/configuration/network/kubespanconfig/ for the full schema.

### Ingress Firewall
```yaml
machine:
  network:
    interfaces:
      - deviceSelector:
          hardwareAddr: "aa:bb:cc:*"
        addresses:
          - 10.0.0.2/24
# Firewall rules are configured via NetworkRuleConfig resources
# Applied via machine config patches or inline manifests
```

### DNS & Resolvers
```yaml
machine:
  network:
    nameservers:
      - 8.8.8.8
      - 1.1.1.1
      - 2001:4860:4860::8888
```

Host DNS runs on `169.254.116.108` — all pod DNS queries route through it.

**v1.13 behavior change**: `machine.network.nameservers` now overwrites lower-layer nameservers (defaults, platform) when set. Previously, a smart merge preserved IPv4 or IPv6 entries from lower layers if the machine config specified only one type. If you specify only IPv4 nameservers, IPv6 entries from platform defaults are dropped — configure both families explicitly if needed. The standalone `ResolverConfig` document offers the same control.

### Time Servers
```yaml
machine:
  time:
    servers:
      - time.cloudflare.com
      - pool.ntp.org
    bootTimeout: 2m0s
```

## v1.13 Network Configuration Documents

These are standalone documents applied alongside the v1alpha1 `Config` (separated by `---`). See the docs URL at the top of this file for full schemas.

- **`RoutingRuleConfig`** — policy routing rules (`ip rule`-style: source/dest CIDR, FW mark, tos → table lookup).
- **`VRFConfig`** — Virtual Routing and Forwarding. Define VRF instances and associate interfaces.
- **`LinkAliasConfig`** — stable alias names for interfaces. As of v1.13 supports the `%d` format verb (e.g., `net%d`): when the alias contains `%d`, the selector can match multiple links and each matched link receives a sequential alias (`net0`, `net1`, …) ordered by hardware address. Links already aliased by an earlier config are skipped. Useful for `BondConfig` and `BridgeConfig` member interfaces on varying hardware.
- **`TCPProbeConfig`** — TCP probes for network-reachability checks. Lets you define a custom connectivity condition that Talos can wait on.
- **`KubeSpanConfig`** — see KubeSpan section above (replaces the legacy `.machine.network.kubespan`).
- **`ResolverConfig`** — DNS resolvers (overwrites lower layers, like the legacy field; see DNS & Resolvers above).

## Flannel CNI with Network Policies (v1.13)

Talos v1.13 optionally deploys Flannel with [kube-network-policies](https://github.com/kubernetes-sigs/kube-network-policies) for NetworkPolicy enforcement:

```yaml
cluster:
  network:
    cni:
      name: flannel
      flannel:
        kubeNetworkPoliciesEnabled: true
```

If the cluster is already running, sync the bootstrap manifests after applying the patch to deploy the new CNI configuration.

## Diagnostics

Check network state with MCP tools:
- `talos_addresses` — assigned IP addresses
- `talos_routes` — routing table
- `talos_interfaces` — interface status (up/down, MTU, etc.)
- `talos_netstat` — active connections and listeners
- `talos_resolvers` — configured DNS resolvers
- `talos_hostname` — node hostname
- `talos_time` — NTP sync status
