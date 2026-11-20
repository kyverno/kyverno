# AuthorizingPolicy Design

This document is the single source of truth for the `AuthorizingPolicy` feature in Kyverno — covering implementation status, runtime architecture, endpoint behaviour, CEL evaluation, and conformance coverage.

## Upstream Dependency and Scope

This feature is designed to align with Kubernetes KEP-5681 (Conditional Authorization).

- Upstream dependency: `ConditionalAuthorization` feature gate in kube-apiserver.
- Upstream milestone: Kubernetes v1.36 alpha (per KEP-5681 / kyverno issue #15778 tracking).
- Initial scope in kube-apiserver: requests that support conditional enforcement in the authorization + admission flow (write/connect paths as described by KEP-5681), not general read authorization.
- Future scope: conditional reads are explicitly a future direction in the KEP, not part of current implementation.

---

## Implementation Status

All core components are implemented and validated in local build/unit coverage. Live conformance should be rerun after deploying the current branch to a KEP-enabled cluster.

> Note
> Kyverno is still pinned to a `controller-runtime` main-branch pseudo-version in [go.mod](../../../go.mod) because the latest tagged `v0.23.3` release does not yet implement `HasSyncedChecker()`, which is now required by the `client-go` 1.36 `ResourceEventHandlerRegistration` interface. Once `controller-runtime` ships a tagged release with that method, the follow-up change is to remove the temporary `replace sigs.k8s.io/controller-runtime => ...` override and update the direct `sigs.k8s.io/controller-runtime` requirement to that tagged version.

| Component | Status |
|---|---|
| API types (`api/policies/v1alpha1/`) | ✅ Complete |
| CEL compiler (`pkg/cel/policies/apol/compiler/`) | ✅ Complete |
| CEL engine (`pkg/cel/policies/apol/engine/`) | ✅ Complete |
| Provider / policy cache (`pkg/cel/policies/apol/provider/`) | ✅ Complete |
| Webhook handler (`pkg/webhooks/resource/apol/handler.go`) | ✅ Complete |
| Server route wiring (`pkg/webhooks/server.go`) | ✅ Complete |
| Main wiring (`cmd/kyverno/main.go`) | ✅ Complete |
| Policy status controller (`pkg/controllers/policystatus/`) | ✅ Complete |
| Chainsaw conformance tests | ✅ Complete |

---

## Runtime Flow

1. `cmd/kyverno/main.go` registers `policies.kyverno.io/v1alpha1` in the shared scheme, constructs an `apolprovider.KubeProvider` backed by controller-runtime, and passes it to the APOL handler.
2. `pkg/cel/policies/apol/provider/provider.go` watches cluster-scoped `AuthorizingPolicy` objects, compiles them to CEL programs via `apolcompiler.Compiler`, caches the compiled form, and reconciles `status.conditionStatus` with compile/cache readiness.
3. `pkg/webhooks/resource/apol/handler.go` exposes two HTTP endpoints:
  - `/authz/subjectaccessreview` — handles authorization-phase `SubjectAccessReview` requests and returns decisive outcomes or a native conditional `conditionSetChain` response.
  - `/authz/conditions` — evaluates a supplied `conditionSetChain` in KEP-shaped `AuthorizationConditionsReview` payloads and returns a concrete allow/deny/no-opinion response.
4. `pkg/controllers/policystatus/apol.go` consumes webhook recorder notifications and merges webhook readiness into `status.conditionStatus` for `AuthorizingPolicy`.

---

## Implemented Files

### Runtime and Wiring

- `cmd/kyverno/main.go`
  - Registers `policies.kyverno.io/v1alpha1` scheme.
  - Calls `apolprovider.NewKubeProvider(apolCompiler, mgr)`.
  - Passes provider to `apol.New(compiledApolProvider)` and registers with the webhook server.

- `pkg/cel/policies/apol/provider/provider.go`
  - Implements `controller-runtime` reconciler for `AuthorizingPolicy`.
  - Compiles and caches policies on create/update; removes from cache on delete.
  - Updates `status.conditionStatus.PolicyCached` on each reconcile.

- `pkg/webhooks/resource/apol/handler.go`
  - Parses `SubjectAccessReview` from the request body.
  - Calls `provider.Fetch(ctx)` to get all compiled policies.
  - Iterates policies; first `Allow` or `Deny` match wins; returns `NoOpinion` if none match.
  - For `/authz/conditions`, evaluates only caller-supplied `conditionSetChain` entries (no policy re-fetch in this phase).

- `pkg/webhooks/server.go`
  - Registers `POST /authz/subjectaccessreview` → `HandleSubjectAccessReview`.
  - Registers `POST /authz/conditions` → `HandleConditionsReview`.

### CEL Compiler and Engine

- `pkg/cel/policies/apol/compiler/compiler.go`
  - Compiles `spec.matchConditions`, `spec.variables`, `spec.rules[].matchConditions`, `spec.rules[].expression`, and `spec.rules[].conditions`.
  - Uses dynamic request activation under the `request` key.

- `pkg/cel/policies/apol/engine/engine.go`
  - Evaluates policy-level match conditions first; skips policy if not satisfied.
  - Evaluates rules in order; first matching decisive rule (`Allow`/`Deny`) returns immediately.
  - For `Conditional` rules, evaluates all `conditions` expressions and returns `ConditionResult` list.
  - Respects `spec.failurePolicy` for evaluation errors (`Ignore` → `NoOpinion`, `Deny` → `Deny`).
  - Variables are lazily evaluated via `lazyVariables`.

### Policy Status

- `pkg/controllers/policystatus/apol.go`
  - Compiles the policy with a fresh compiler instance to check for syntax errors.
  - Sets `PolicyCached` condition from compile result.
  - Sets `WebhookConfigured` condition from the webhook recorder state.
  - Recalculates `status.conditionStatus.ready` from all conditions.
  - Persists via `dclient.UpdateStatusResource`.

- `pkg/controllers/policystatus/controller.go`
  - Routes `AuthorizingPolicyType` reconcile requests to `updateApolStatus`.

---

## CEL Activation Shape

The authorization handler converts a `SubjectAccessReview` into a dynamic CEL activation under `request`.

Supported fields:

| Field | Source |
|---|---|
| `request.user` | `spec.user` |
| `request.uid` | `spec.uid` |
| `request.groups` | `spec.groups` |
| `request.extra` | `spec.extra` |
| `request.verb` | `spec.resourceAttributes.verb` or `spec.nonResourceAttributes.verb` |
| `request.namespace` | `spec.resourceAttributes.namespace` |
| `request.name` | `spec.resourceAttributes.name` |
| `request.resource` | `spec.resourceAttributes.resource` |
| `request.subresource` | `spec.resourceAttributes.subresource` |
| `request.apiGroup` | `spec.resourceAttributes.group` |
| `request.apiVersion` | `spec.resourceAttributes.version` |
| `request.path` | `spec.nonResourceAttributes.path` |

Authorization phase (`/authz/subjectaccessreview`) uses SAR fields only. Target-object labels are not injected in this phase.

Conditions phase (`/authz/conditions`) additionally supports these request fields when provided via `AuthorizationConditionsReview.request`:

| Field | Source |
|---|---|
| `request.object` | `object` payload field |
| `request.oldObject` | `oldObject` payload field |
| `request.resourceLabels` | `object.metadata.labels` |
| `request.oldResourceLabels` | `oldObject.metadata.labels` |

This allows prototype CEL conditions like `request.resourceLabels['env'] == 'prod'` during condition evaluation.

Example expressions:

```
request.groups.exists(g, g == 'developers')
request.namespace == 'team-a'
request.verb in ['get', 'list', 'watch']
```

---

## Evaluated Policy Fields

The runtime path evaluates these `spec` fields:

- `spec.matchConditions`
- `spec.variables`
- `spec.rules[].matchConditions`
- `spec.rules[].expression`
- `spec.rules[].conditions`

`spec.subjects` and `spec.matchConstraints` exist in the API/CRD schema but are not part of the evaluation path.

---

## Endpoints

### `POST /authz/subjectaccessreview`

The decisive authorization endpoint.

- Input: `authorization.k8s.io/v1 SubjectAccessReview` JSON body.
- Output: same type with either concrete status (`allowed`/`denied`/`reason`) or a conditional `status.conditionSetChain` response.
- `Allow` rules set `status.allowed=true`.
- `Deny` rules set `status.denied=true`.
- `Conditional` rules only return `status.conditionSetChain` when the request advertises conditional support mode; otherwise Kyverno folds the result to deny or no-opinion.
- If no decisive rule matches, `status.reason` is `"no policy matched"` and both booleans are false.

### `POST /authz/conditions`

The conditional-evaluation endpoint.

- Input: KEP-shaped `AuthorizationConditionsReview` payload:

```json
{
  "apiVersion": "authorization.k8s.io/v1alpha1",
  "kind": "AuthorizationConditionsReview",
  "request": {
    "conditionSetChain": [
      {
        "authorizerName": "kyverno",
        "conditionsType": "k8s.io/cel",
        "conditions": [
          {
            "id": "allow-prod",
            "effect": "Allow",
            "condition": "request.resourceLabels['env'] == 'prod'"
          }
        ]
      }
    ],
    "spec": {
      "user": "alice",
      "groups": ["developers"],
      "resourceAttributes": {
        "verb": "update",
        "resource": "deployments",
        "namespace": "tenant-a-prod"
      }
    },
    "object": {
      "apiVersion": "apps/v1",
      "kind": "Deployment",
      "metadata": {
        "labels": {
          "env": "prod"
        }
      }
    }
  }
}
```

- Output: `AuthorizationConditionsReview.response` with concrete `allowed`/`denied` decision and optional `reason` / `evaluationError`.
- Evaluates condition sets in chain order; later sets are evaluated only if prior sets resolve to `NoOpinion`.

---

## Prototype Gap Closure (Implemented)

To make this a valid ConditionalAuthorization prototype for object-aware conditions, the following steps were required and are now implemented:

1. Introduce a dedicated conditions-review payload carrying `request`, `object`, and `oldObject`.
2. Keep SAR-shaped payload support for compatibility during transition.
3. Extend condition-phase activation with `request.resourceLabels` and `request.oldResourceLabels` from object metadata.
4. Add tests proving label-aware condition evaluation and compatibility behavior.

---

## Status Reporting

Two conditions are surfaced in `status.conditionStatus`:

- `PolicyCached` — set `True` when the policy compiles and is cached; set `False` with the error message on compile failure.
- `WebhookConfigured` — set `True` when the webhook recorder confirms the policy is registered; `False` otherwise.

`status.conditionStatus.ready` is `true` only when all conditions are `True`.

---

## Conformance Coverage

Chainsaw suites live under `test/conformance/chainsaw/authorize/`:

| Suite | What it verifies |
|---|---|
| `basic-endpoints` | Allow/deny decisions and conditions endpoint response shape |
| `conditional-effects` | `Conditional` rules return expected condition IDs and effects |
| `conditions-payload-labels` | Conditions payload supports object metadata labels (`request.resourceLabels`) |
| `match-conditions` | Policy-level and rule-level `matchConditions` gate evaluation |

Baseline suites validated on a clean kind cluster; rerun the current branch after deployment for final confirmation:

```bash
KIND_NAME=kind-apol make kind-delete-cluster
KIND_NAME=kind-apol make kind-create-cluster
KIND_NAME=kind-apol make kind-deploy-kyverno
chainsaw test \
  test/conformance/chainsaw/authorize/basic-endpoints \
  test/conformance/chainsaw/authorize/conditional-effects \
  test/conformance/chainsaw/authorize/match-conditions
```

Result: **3 passed, 0 failed, 0 skipped.**

Label-aware conditions suite command:

```bash
chainsaw test test/conformance/chainsaw/authorize/conditions-payload-labels
```

Status: **implemented and validated in test fixtures; run this suite in a live kind environment as part of pre-merge e2e validation**.

---

## References

- [Kubernetes Authorization API](https://kubernetes.io/docs/reference/access-authn-authz/authorization/)
- [CEL Language specification](https://github.com/google/cel-spec)
- [Kyverno repo overview](../../../AGENTS.md)
- [Example policies](examples/)
