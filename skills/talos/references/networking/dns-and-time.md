# DNS, hostname, time sync and probes (Talos v1.14)

Field reference: https://docs.siderolabs.com/talos/v1.14/reference/configuration/network/

## ResolverConfig

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

Defaults are `1.1.1.1` and `8.8.8.8`. Setting `nameservers` **overwrites** the lower layers (DHCP,
platform, defaults) rather than merging, so listing only IPv4 servers drops the platform's IPv6 ones
— configure both families explicitly when both are needed.

`searchDomains.domains: []`, an explicit empty list, clears DHCP and platform search domains;
leaving the field unset inherits them. `disableDefault: true` stops deriving a search domain from
the hostname FQDN.

### Encrypted DNS (new in 1.14)

`protocol: DoT` is DNS over TLS (RFC 7858, TCP/853); `protocol: DoH` is DNS over HTTPS (RFC 8484,
TCP/443, path `/dns-query`). `tlsServerName` is **required** for both and must be **empty** for
`Do53`; it is the SNI, the name verified against the certificate, and for DoH the host portion of
the request URL (`https://<tlsServerName>/dns-query`), while the connection still goes to `address`.

> **Boot-stall deadlock.** Certificate validation needs a correct clock, NTP servers given as
> hostnames need DNS, and NTS (below) *requires* hostnames. If every nameserver is DoT/DoH the boot
> can stall — Talos emits a validation warning for exactly this case. Keep one plain-DNS fallback
> nameserver, configure NTP servers by IP, or rely on a working hardware clock.

### Host DNS

Moved in 1.14 from `.machine.features.hostDNS`. A caching resolver on the node at
`169.254.116.108` (IPv6 `fd54:616c:6f73::204f:5320:444e:531`), using the `nameservers` above as
upstreams. `forwardKubeDNSToHost` points CoreDNS at it instead of at the upstreams directly;
`resolveMemberNames` resolves cluster member hostnames locally and needs cluster discovery. Both are
rejected when `enabled: false`.

## StaticHostConfig and HostnameConfig

`StaticHostConfig` writes `/etc/hosts` entries; the document name is the IP address and `hostnames`
is the list of names for it.

```yaml
apiVersion: v1alpha1
kind: HostnameConfig
hostname: controlplane1.example.org
---
apiVersion: v1alpha1
kind: HostnameConfig
auto: stable        # stable | off
```

`hostname` outranks DHCP and cloud-init; `auto` has the lowest priority. `stable` derives a hostname
from machine identity, `off` waits for an external source. The two fields conflict — set one.

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

`ntp.useNTS` (new in 1.14) enables Network Time Security — authenticated, encrypted NTP over TLS. It
defaults to true when no `TimeSyncConfig` is supplied at all, with the default server
`time.cloudflare.com`. With NTS every NTP server must be given as a **hostname**, not an IP, which
is the other half of the DoT/DoH deadlock above.

PTP is mutually exclusive with NTP: use `ptp.devices: [/dev/ptp_kvm]` instead of an `ntp:` block.

## Network probes

Probes gate readiness on a reachability condition.

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

HTTP probes succeed on 2xx and 3xx and do **not** follow redirects; connection and transport errors
count as failures. `interval` defaults to 1s, `timeout` to 10s, `failureThreshold` to 0 (fail on the
first failure). Results: `talosctl get probestatuses`.

## Diagnostics

Useful resources (`talosctl get <type>` / the `talos_get` MCP tool):

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
