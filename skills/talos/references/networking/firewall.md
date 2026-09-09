# Ingress firewall (Talos v1.14)

Field reference: https://docs.siderolabs.com/talos/v1.14/reference/configuration/network/

Two documents. `NetworkDefaultActionConfig` sets the chain policy; `NetworkRuleConfig` — note the
capital `N`, the kind is not `networkRuleConfig` — adds rules. Rules are host-wide, matched on
destination port plus source subnet, never per interface.

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

## The model

The default action decides what a `NetworkRuleConfig` *means*; the same document is read in both
directions (`Invert: defaultAction == accept`):

- `ingress: block` — the chain drops by default and each rule is an **allowlist**: traffic to those
  ports is accepted when it comes from the listed subnets.
- `ingress: accept` — the chain accepts by default and each rule inverts into a **blocklist**:
  traffic to those ports is dropped unless it comes from the listed subnets.

`protocol` is `tcp`, `udp`, `icmp` or `icmpv6`. `ports` accepts bare integers and `lo-hi` strings,
and ranges must not overlap within one rule. `except` carves a prefix out of the `subnet` above it.

## What `ingress: block` still allows implicitly

- established and related conntrack entries (invalid packets are dropped)
- traffic arriving on trusted interfaces: loopback, SideroLink, KubeSpan
- traffic not addressed to the machine itself
- ICMP and ICMPv6, rate-limited to 5 packets per second
- ICMP timestamp and address-mask requests and replies are **dropped** (CVE-1999-0524)
- pod-CIDR and service-CIDR traffic to the host DNS address on port 53, when
  `hostDNS.forwardKubeDNSToHost` is enabled
- pod/service CIDR to pod/service CIDR traffic

Nothing else is opened. Every port the cluster needs requires its own rule:

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

The 1.14 etcd split is firewall-relevant: `2379` now serves gRPC only, and `/metrics`, `/health` and
the gRPC-gateway JSON endpoints moved to `2383`, so an existing ruleset that opened 2379 needs a
matching rule for 2383 (see `references/v1.14-changes.md`).

Inspect the compiled chains with `talosctl get nftableschains -o yaml`.
