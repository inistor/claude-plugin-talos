# Kubernetes config documents (`Kube*Config`)

24 documents, all new in v1.14, all `apiVersion: v1alpha1`, all singletons except where a `name` is
shown. Field-by-field reference:
https://docs.siderolabs.com/talos/v1.14/reference/configuration/ — this file carries only the
examples worth hand-editing and the rules that are not obvious from the schema.

## Kubelet

```yaml
apiVersion: v1alpha1
kind: KubeletConfig
image: ghcr.io/siderolabs/kubelet:v1.37.0
clusterDNS:
    - 10.96.0.10
extraArgs:
    feature-gates: AllBeta=true
config:                       # raw upstream KubeletConfiguration
    serverTLSBootstrap: true
    systemReserved:
        cpu: 500m
        memory: 1Gi
defaultRuntimeSeccompProfileEnabled: true
```

`config` is the upstream `KubeletConfiguration` verbatim. There is **no `extraMounts`** — a config
that needs one must stay on legacy `.machine.kubelet` entirely (`config-migration.md`) — and
`disableManifestsDirectory` is locked to `true`.

## Node registration, labels and taints

```yaml
apiVersion: v1alpha1
kind: KubeNodeConfig
registerWithFQDN: true
nodeIP:
    validSubnets:
        - 10.0.0.0/8
        - '!10.0.0.3/32'      # exclusion
        - fdc7::/16
labels:
    topology.kubernetes.io/zone: rack-13
annotations:
    customer.io/rack: r13a25
taints:
    node-role.kubernetes.io/control-plane: NoSchedule
# skipNodeRegistration: false
```

To schedule workloads on control planes (the old `allowSchedulingOnControlPlanes`), remove the
`node-role.kubernetes.io/control-plane: NoSchedule` entry — there is no boolean.

## API server

```yaml
apiVersion: v1alpha1
kind: KubeAPIServerConfig
image: registry.k8s.io/kube-apiserver:v1.37.0
apiPort: 6443
certExtraSANs:
    - k8s.example.com
extraArgs:
    feature-gates: ServerSideApply=true
env:
    HTTPS_PROXY: http://proxy:8080
resources:
    requests: {cpu: 2, memory: 2Gi}
startupProbes: false
```

`KubeControllerManagerConfig` and `KubeSchedulerConfig` take the same `image` / `extraArgs` / `env`
/ `resources` / `enabled` shape; `KubeSchedulerConfig` adds `config` (a `KubeSchedulerConfiguration`).

## Manifests

One **named document per manifest** — the old `.cluster.inlineManifests` list is gone. Inline
manifests may themselves contain several `---`-separated objects.

```yaml
apiVersion: v1alpha1
kind: KubeInlineManifestConfig
name: namespace-ci
manifest: |-
    apiVersion: v1
    kind: Namespace
    metadata:
      name: ci
---
apiVersion: v1alpha1
kind: KubeExternalManifestConfig
name: example-cni
url: https://www.example.com/v1.2.3/manifest.yaml
headers:
    Authorization: Bearer token
```

Pin external URLs to a release tag: every node fetches them at bootstrap, so a moving `latest`
produces a split-brain cluster.

## Networking documents

```yaml
apiVersion: v1alpha1
kind: KubeNetworkConfig
dnsDomain: cluster.local
podSubnets:
    - 10.244.0.0/16
serviceSubnets:
    - 10.96.0.0/12
# nodeCIDRMaskSizeIPv4: 24
# nodeCIDRMaskSizeIPv6: 112
---
apiVersion: v1alpha1
kind: KubePrismConfig
port: 7445
tlsServerName: api.cluster.local
```

`podSubnets` and `serviceSubnets` are `merge:"replace"` — a patch **overwrites** the list rather
than appending, so a patch adding an IPv6 subnet must repeat the IPv4 one.

```yaml
apiVersion: v1alpha1
kind: KubeFlannelCNIConfig
backendType: vxlan
backendPort: 4789
backendMTU: 1420
extraArgs:
    - --iface-can-reach=10.0.0.1
kubeNetworkPoliciesEnabled: true
```

**For a custom CNI, simply omit `KubeFlannelCNIConfig`** — no document means no Flannel. Install
the CNI through a manifest document or out of band with Helm. With Cilium's kube-proxy replacement,
also set `KubeProxyConfig` `enabled: false`.

## Pod Security admission

```yaml
apiVersion: v1alpha1
kind: KubeAdmissionControlConfig
name: PodSecurity
configuration:
    apiVersion: pod-security.admission.config.k8s.io/v1
    kind: PodSecurityConfiguration
    defaults: {enforce: baseline, enforce-version: latest}
    exemptions:
        namespaces: [kube-system]
```

Use `pod-security.admission.config.k8s.io/v1`. The `v1alpha1` group version still present in some
upstream fixtures predates Kubernetes 1.25.

## The remaining documents

| Kind | Purpose |
|---|---|
| `KubeClusterConfig` | cluster name and control-plane endpoint |
| `KubeProxyConfig` | kube-proxy: `enabled`, `image`, `mode`, `config`, `extraArgs` |
| `KubeCoreDNSConfig` | CoreDNS: `enabled`, `image` |
| `KubeControllerManagerConfig` / `KubeSchedulerConfig` | control-plane components |
| `KubeAuditPolicyConfig` | apiserver audit `Policy` (`merge:"replace"`) |
| `KubeAuthenticationConfig` | structured OIDC/JWT `AuthenticationConfiguration` (`merge:"replace"`) |
| `KubeAuthorizerConfig` | named, one authorizer each: `type` `Node` / `RBAC` / `Webhook`; the generated default is `node` + `rbac`, and replacing it means shipping every authorizer wanted |
| `KubeEtcdEncryptionConfig` | `config` is an upstream `EncryptionConfiguration` (aescbc or secretbox) |
| `KubeAPIServerCAConfig` / `KubeAggregatorCAConfig` | `issuingCA.cert` / `.key` |
| `KubeServiceAccountConfig` | SA token signing key and issuer URL |
| `KubeStaticPodConfig` | named; `pod` is a raw Pod spec (`merge:"replace"`) |
| `KubeCredentialProviderConfig` | kubelet image credential providers (`merge:"replace"`) |
| `KubeTalosAPIAccessConfig` | `allowedRoles`, `allowedKubernetesNamespaces` for Talos API from pods |
