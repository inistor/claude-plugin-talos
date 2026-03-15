# Networking Reference

Docs: https://docs.siderolabs.com/talos/v1.12/talos-guides/network/

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
Mesh networking across sites — enabled in machine config:
```yaml
machine:
  network:
    kubespan:
      enabled: true
      advertiseKubernetesNetworks: false
      mtu: 1420
```

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

### Time Servers
```yaml
machine:
  time:
    servers:
      - time.cloudflare.com
      - pool.ntp.org
    bootTimeout: 2m0s
```

## Diagnostics

Check network state with MCP tools:
- `talos_addresses` — assigned IP addresses
- `talos_routes` — routing table
- `talos_interfaces` — interface status (up/down, MTU, etc.)
- `talos_netstat` — active connections and listeners
- `talos_resolvers` — configured DNS resolvers
- `talos_hostname` — node hostname
- `talos_time` — NTP sync status
