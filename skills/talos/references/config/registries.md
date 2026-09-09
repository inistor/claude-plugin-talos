# Registry documents

Three named documents replace `.machine.registries`. They are **not** new in v1.14 — they exist
since v1.12, so the same YAML works on a v1.13 cluster. Field reference:
https://docs.siderolabs.com/talos/v1.14/reference/configuration/

`name` is the **first segment of the image reference** (`docker.io` for unqualified images), and
`name: "*"` is a catch-all applied to every registry.

## RegistryMirrorConfig

```yaml
apiVersion: v1alpha1
kind: RegistryMirrorConfig
name: ghcr.io
endpoints:
    - url: https://my-private-registry.local:5000
    - url: http://my-harbor/v2/registry-k8s.io/
      overridePath: true
skipFallback: true
```

- Endpoints are tried in order. Without `skipFallback: true`, Talos falls back to the upstream
  registry when no mirror serves the image — which is exactly what an **air-gapped** node must not
  do, since the fallback turns a clear pull failure into a long DNS/TCP timeout.
- `overridePath: true` uses the endpoint path verbatim instead of appending `/v2/`. Required for
  Harbor-style proxy-cache projects, where the project name is already part of the path.

A single pull-through cache for everything, the usual air-gapped or bandwidth-saving setup:

```yaml
apiVersion: v1alpha1
kind: RegistryMirrorConfig
name: "*"
endpoints:
    - url: https://registry-cache.internal:5000
```

Resolution is exact-host first, then `"*"`, then the default host — the two never combine. A
`name: ghcr.io` document therefore fully replaces the catch-all for ghcr.io images, including its
`skipFallback`, so an air-gapped setup must repeat that flag on every specific entry. Mirroring
also applies to images Talos pulls itself (installer, kubelet), so an air-gapped cache needs those
present before an upgrade, not only workload images.

## RegistryAuthConfig

```yaml
apiVersion: v1alpha1
kind: RegistryAuthConfig
name: my-private-registry.io
username: agent007
password: topsecret
# auth: <base64 user:pass>
# identityToken: <token>
```

Credentials are stored verbatim in the machine config — treat the whole config as a secret, and
prefer `KubeCredentialProviderConfig` for workload image pulls that can use a provider binary.

## RegistryTLSConfig

```yaml
apiVersion: v1alpha1
kind: RegistryTLSConfig
name: my-tls-registry.io
clientIdentity:
    cert: |-
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
    key: |-
        -----BEGIN PRIVATE KEY-----
        ...
        -----END PRIVATE KEY-----
ca: |-
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
insecureSkipVerify: true
```

`ca` adds a CA trusted for this registry only. For a trust anchor the whole host should honour
(including non-registry traffic), use `TrustedRootsConfig` instead.
