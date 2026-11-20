# Conditional Authorization (AuthorizingPolicy) Development

This directory contains all documentation and examples for the AuthorizingPolicy feature — Kyverno's Kubernetes-native policy engine for authorization and access control.

## Overview

`AuthorizingPolicy` extends Kyverno into the Kubernetes authorization layer. It evaluates authorization metadata in the SAR path and supports conditional rule evaluation via a dedicated conditions endpoint. This aligns with the KEP-5681 two-phase model direction.

### How it works

1. In the authorization phase, the Kubernetes API server sends a `SubjectAccessReview` to Kyverno at `/authz/subjectaccessreview`.
2. Kyverno evaluates compiled `AuthorizingPolicy` rules and returns either a decisive SAR result (`Allow`/`Deny`) or a native conditional SAR response (`status.conditionSetChain`).
3. For condition evaluation callbacks, the Kubernetes API server uses `/authz/conditions`; this maps to the KEP-5681 condition-enforcement flow.

Conditional SAR responses are returned only when the caller advertises conditional support via the request mode fields. If that mode is absent, Kyverno folds conditional outcomes back into a concrete deny or no-opinion response instead of returning `status.conditionSetChain`.

### API server configuration

Kyverno must be registered as an authorization webhook in the API server. For Conditional Authorization, this feature tracks [KEP-5681](https://github.com/kubernetes/enhancements/blob/master/keps/sig-auth/5681-conditional-authorization/README.md).

> **Note:** Per issue #15778, this feature depends on KEP-5681 and should be tested on a Kubernetes version that includes it (v1.36 alpha milestone) with the `ConditionalAuthorization` feature gate enabled.

#### AuthorizationConfiguration for Conditional Authorization (KEP-5681)

Create an `AuthorizationConfiguration` manifest and pass it to the API server with `--authorization-config=<path>`:

```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: AuthorizationConfiguration
authorizers:
- type: Node
- type: RBAC
- type: Webhook
  name: kyverno
  webhook:
    timeout: 5s
    failurePolicy: NoOpinion
    connectionInfo:
      type: KubeConfigFile
      kubeConfigFile: /etc/kubernetes/kyverno-authz-kubeconfig.yaml
    authorizationReviewVersions: ["v1"]
    # KEP-5681 Conditional Authorization extension:
    conditionsEndpointKubeConfigContext: kyverno-conditions
    authorizationConditionsReviewVersion: v1alpha1
```

The `conditionsEndpointKubeConfigContext` should target the Kyverno endpoint that evaluates `AuthorizationConditionsReview` requests.

`conditionsEndpointKubeConfigContext` is a **kubeconfig context name**, not an HTTP path. The context points to a server URL that should route to Kyverno's conditions endpoint.

Minimal example (single host, different paths):

```yaml
apiVersion: v1
kind: Config
clusters:
- name: kyverno-authz
  cluster:
    server: https://kyverno-svc.kyverno.svc:443/authz/subjectaccessreview
- name: kyverno-conditions
  cluster:
    server: https://kyverno-svc.kyverno.svc:443/authz/conditions
contexts:
- name: kyverno-authz
  context:
    cluster: kyverno-authz
    user: apiserver
- name: kyverno-conditions
  context:
    cluster: kyverno-conditions
    user: apiserver
current-context: kyverno-authz
users:
- name: apiserver
  user:
    token: "<redacted>"
```

#### kind cluster configuration for local testing

To test with kind, pass a custom API server config that mounts the `AuthorizationConfiguration` file:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: ClusterConfiguration
        apiServer:
          extraArgs:
            authorization-config: /etc/kubernetes/authz-config.yaml
          extraVolumes:
            - name: authz-config
              hostPath: /etc/kubernetes/authz-config.yaml
              mountPath: /etc/kubernetes/authz-config.yaml
              readOnly: true
    extraMounts:
      - hostPath: /path/to/authz-config.yaml
        containerPath: /etc/kubernetes/authz-config.yaml
```

---

### Benefits over Kubernetes RBAC

| Capability | RBAC | AuthorizingPolicy |
|---|---|---|
| Expression language | Role/binding graph | Arbitrary CEL |
| Object-aware constraints at authorization time | No | Yes (via conditional flow) |
| Reusable variables | No | `spec.variables` |
| Scoped match conditions per rule | No | `spec.rules[].matchConditions` |
| Conditional / break-glass effects | No | `Conditional` rule effect |
| Explainability through SAR condition output | Limited | Yes |

RBAC remains the core allow model for metadata-only authorization decisions. `AuthorizingPolicy` complements RBAC by adding conditional, data-aware constraints that can be enforced at admission while still being visible through authorization APIs.

### Benefits over admission webhooks

Admission webhooks fire **after** authorization succeeds, giving them no ability to influence whether a request is allowed at all. `AuthorizingPolicy` operates at the authorization step, so:

- Requests can be denied before they reach any admission webhook.
- There is no need for an extra custom admission webhook just to compensate for RBAC over-grants; Kyverno handles both authorization-time decisions and condition evaluation callbacks.
- CEL programs are compiled once and cached; evaluation adds minimal latency.
- Policies use the same CRD/status lifecycle as other Kyverno policies (`PolicyCached`, `WebhookConfigured` conditions).

---

## Documentation

- **[DESIGN.md](DESIGN.md)** — Implementation status, runtime architecture, endpoints, CEL activation, and conformance coverage

## Examples

- **[examples/](examples/)** — Sample AuthorizingPolicy resources demonstrating conditional authorization:
  - `tenant-governance-breakglass.yaml` — Tenant namespace governance with conditional break-glass for admin writes
  - `role-based-pod-access.yaml` — Pod and subresource (`pods/exec`) controls for production operations
  - `api-surface-control.yaml` — Non-resource URL and API-surface authorization controls
  - See [examples/README.md](examples/README.md) for full documentation

## Architecture

### Current State (Completed)

- ✅ API types (`api/policies/v1alpha1/authorizingpolicy_types.go`)
- ✅ CEL compiler and engine (`pkg/cel/policies/apol/`)
- ✅ Webhook handlers (`pkg/webhooks/resource/apol/`)
- ✅ Handler registration in main (`cmd/kyverno/main.go`)
- ✅ HTTP endpoints (`/authz/subjectaccessreview`, `/authz/conditions`)
- ✅ Basic Chainsaw conformance tests

## Quick Links

- [Kyverno CEL Libraries](../../dev/cel/)
- [ValidatingPolicy Reference Implementation](../../pkg/cel/policies/vpol/engine/)
- [MutatingPolicy Reference Implementation](../../pkg/cel/policies/mpol/engine/)
- [Kubernetes Authorization API](https://kubernetes.io/docs/reference/access-authn-authz/authorization/)

## Testing

### Chainsaw Tests
```bash
# Run endpoint connectivity tests
chainsaw test test/conformance/chainsaw/authorize/basic-endpoints/

# Run all authorize tests (once e2e tests are added)
chainsaw test test/conformance/chainsaw/authorize/
```

### Manual Testing
```bash
# Deploy sample policy
kubectl apply -f docs/dev/conditional-authorization/examples/tenant-governance-breakglass.yaml

# Optionally deploy additional examples
kubectl apply -f docs/dev/conditional-authorization/examples/role-based-pod-access.yaml
kubectl apply -f docs/dev/conditional-authorization/examples/api-surface-control.yaml

# Test with curl via port-forward
kubectl -n kyverno port-forward svc/kyverno-svc 9443:443 &
curl -sk https://127.0.0.1:9443/authz/subjectaccessreview -H 'Content-Type: application/json' \
  -d '{"spec":{"conditionalAuthorization":{"mode":"HumanReadable"},"user":"alice","groups":["developers"],"resourceAttributes":{"verb":"get","resource":"pods"}}}'
```

## Status

**Feature Status:** Complete for build/unit coverage — endpoints, provider, CEL engine, and status controller are implemented; live conformance should be rerun after deployment on a KEP-enabled cluster.

**Last Updated:** April 14, 2026
