# Machine Configuration Reference (v1alpha1)

Docs: https://docs.siderolabs.com/talos/v1.12/reference/configuration/v1alpha1/config/

## Top-Level Structure

```yaml
version: v1alpha1
debug: false
machine:
  type: controlplane  # or "worker"
  # ... machine config
cluster:
  # ... cluster config
```

## Machine Section

### Core Fields
- `type`: `controlplane` or `worker`
- `token`: machine token for node authentication
- `ca`: machine CA certificate and key
- `certSANs`: additional SANs for the API certificate

### Install
```yaml
machine:
  install:
    disk: /dev/sda           # install target disk
    image: ghcr.io/siderolabs/installer:v1.12.0
    bootloader: true
    wipe: false
    extensions:
      - image: ghcr.io/siderolabs/iscsi-tools:latest
```

### Network
```yaml
machine:
  network:
    hostname: node1
    interfaces:
      - deviceSelector:
          hardwareAddr: "00:11:22:*"
        addresses:
          - 10.0.0.2/24
        routes:
          - network: 0.0.0.0/0
            gateway: 10.0.0.1
        dhcp: false
        vip:
          ip: 10.0.0.100
    nameservers:
      - 8.8.8.8
      - 1.1.1.1
```

### Kubelet
```yaml
machine:
  kubelet:
    image: ghcr.io/siderolabs/kubelet:v1.35.0
    extraArgs:
      rotate-server-certificates: "true"
    extraMounts:
      - destination: /var/local
        type: bind
        source: /var/local
        options: [bind, rshared, rw]
    nodeIP:
      validSubnets:
        - 10.0.0.0/24
```

### Files
```yaml
machine:
  files:
    - content: |
        [plugins."io.containerd.grpc.v1.cri"]
          enable_unprivileged_ports = true
      permissions: 0o644
      path: /etc/cri/conf.d/20-customization.part
      op: create
```

### Features
```yaml
machine:
  features:
    rbac: true
    stableHostname: true
    kubernetesTalosAPIAccess:
      enabled: true
      allowedRoles:
        - os:reader
      allowedKubernetesNamespaces:
        - kube-system
```

### Kernel
```yaml
machine:
  kernel:
    modules:
      - name: br_netfilter
      - name: nf_conntrack
```

### Sysctls
```yaml
machine:
  sysctls:
    net.core.somaxconn: "65535"
    net.ipv4.ip_forward: "1"
    vm.overcommit_memory: "1"
```

## Cluster Section

### Core Fields
```yaml
cluster:
  id: <cluster-id>
  secret: <cluster-secret>
  controlPlane:
    endpoint: https://10.0.0.100:6443
  clusterName: my-cluster
  network:
    cni:
      name: custom
      urls:
        - https://raw.githubusercontent.com/projectcalico/calico/v3.27.0/manifests/calico.yaml
    dnsDomain: cluster.local
    podSubnets:
      - 10.244.0.0/16
    serviceSubnets:
      - 10.96.0.0/12
```

### API Server
```yaml
cluster:
  apiServer:
    image: registry.k8s.io/kube-apiserver:v1.35.0
    certSANs:
      - 10.0.0.100
    extraArgs:
      feature-gates: GracefulNodeShutdown=true
    admissionControl:
      - name: PodSecurity
        configuration:
          apiVersion: pod-security.admission.config.k8s.io/v1alpha1
          kind: PodSecurityConfiguration
          defaults:
            enforce: baseline
```

### etcd
```yaml
cluster:
  etcd:
    ca:
      crt: <base64>
      key: <base64>
    extraArgs:
      election-timeout: "5000"
    advertisedSubnets:
      - 10.0.0.0/24
```

### Discovery
```yaml
cluster:
  discovery:
    enabled: true
    registries:
      kubernetes:
        disabled: false
      service:
        disabled: false
```

### Inline Manifests
```yaml
cluster:
  inlineManifests:
    - name: cilium
      contents: |
        apiVersion: v1
        kind: Namespace
        metadata:
          name: cilium
```

## Strategic Merge Patching

Patches modify the config without replacing it entirely:

```yaml
# Add a kernel module
machine:
  kernel:
    modules:
      - name: br_netfilter

# Delete a field
machine:
  network:
    interfaces:
      - deviceSelector:
          hardwareAddr: "00:11:22:*"
        $patch: delete
```

Multi-document patches (separate with `---`) apply in order.
