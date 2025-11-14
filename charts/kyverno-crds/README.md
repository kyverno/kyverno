# Kyverno CRDs Helm Chart

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Helm Version](https://img.shields.io/badge/Helm-v3.2%2B-informational)](https://helm.sh/)

This Helm chart contains the Custom Resource Definitions (CRDs) for [Kyverno](https://kyverno.io/), a Kubernetes-native policy engine.

## About This Chart

This chart addresses [Kyverno Issue #13928](https://github.com/kyverno/kyverno/issues/13928) by providing a **standalone Helm chart** containing only Kyverno CRDs.

**Benefits:**
- **Offline Deployments**: OCI format for air-gapped clusters
- **Separate Lifecycle**: Deploy CRDs independently from main app  
- **ArgoCD Integration**: Better separation of concerns
- **Selective Installation**: Choose only needed CRDs

## Supported CRDs (17 Total)

### kyverno.io (7 CRDs)
- `cleanuppolicies` - Resource cleanup policies
- `clustercleanuppolicies` - Cluster-wide cleanup policies
- `clusterpolicies` - Cluster-wide policy definitions  
- `globalcontextentries` - Global context for policies
- `policies` - Namespace-scoped policies
- `policyexceptions` - Policy enforcement exceptions
- `updaterequests` - Background processing requests

### policies.kyverno.io (6 CRDs) 
- `deletingpolicies` - V2 deleting policies
- `generatingpolicies` - V2 generating policies
- `imagevalidatingpolicies` - V2 image validation
- `mutatingpolicies` - V2 mutating policies
- `policyexceptions` - V2 policy exceptions
- `validatingpolicies` - V2 validating policies

### reports.kyverno.io (2 CRDs)
- `clusterephemeralreports` - Cluster ephemeral reports
- `ephemeralreports` - Namespace ephemeral reports

### wgpolicyk8s.io (2 CRDs)
- `clusterpolicyreports` - Cluster policy reports
- `policyreports` - Namespace policy reports

## Installation

### Standard Installation
```bash
# Add repository
helm repo add kyverno https://kyverno.github.io/kyverno/
helm repo update

# Install CRDs
helm install kyverno-crds kyverno/kyverno-crds -n kyverno --create-namespace

# Verify
kubectl get crd | grep kyverno
```

### OCI Installation (Offline)
```bash
# Pull chart
helm pull oci://ghcr.io/kyverno/charts/kyverno-crds --version 0.1.0

# Install from cache
helm install kyverno-crds ./kyverno-crds-0.1.0.tgz -n kyverno --create-namespace
```

### Custom Installation
```bash
# Create values file
cat > custom-values.yaml << YAML_EOF
# Only validation CRDs
groups:
  kyverno:
    clusterpolicies: true
    policies: true
    policyexceptions: true
    # Disable others
    cleanuppolicies: false
    clustercleanuppolicies: false
    globalcontextentries: false
    updaterequests: false
  policies:
    validatingpolicies: true
    imagevalidatingpolicies: true
    # Disable others
    mutatingpolicies: false
    generatingpolicies: false
    deletingpolicies: false
    policyexceptions: true

# ArgoCD annotation
annotations:
  argocd.argoproj.io/sync-options: Replace=true
YAML_EOF

# Install with custom values
helm install kyverno-crds kyverno/kyverno-crds -f custom-values.yaml -n kyverno --create-namespace
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `installCRDs` | Install all CRDs | `true` |
| `groups.kyverno.*` | kyverno.io CRDs | `true` (all) |
| `groups.policies.*` | policies.kyverno.io CRDs | `true` (all) |
| `groups.reports.*` | reports.kyverno.io CRDs | `true` (all) |
| `groups.wgpolicyk8s.*` | wgpolicyk8s.io CRDs | `true` (all) |
| `annotations` | Additional CRD annotations | `{}` |
| `customLabels` | Additional CRD labels | `{}` |

## ArgoCD Example

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kyverno-crds
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://kyverno.github.io/kyverno
    chart: kyverno-crds
    targetRevision: 0.1.0
    helm:
      values: |
        annotations:
          argocd.argoproj.io/sync-options: Replace=true
  destination:
    server: https://kubernetes.default.svc
    namespace: kyverno
  syncPolicy:
    syncOptions:
    - CreateNamespace=true
    - Replace=true
```

## Migration from Integrated CRDs

If using main kyverno chart with `crds.install=true`:

1. Install standalone CRDs chart
2. Update main chart: `--set crds.install=false`

## License

Apache 2.0 License - see [LICENSE](https://www.apache.org/licenses/LICENSE-2.0)
