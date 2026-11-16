# AuthorizingPolicy end-to-end tests

These Chainsaw tests send real requests through kube-apiserver, with
conditional authorization enabled and Kyverno configured as a webhook
authorizer. Each test impersonates a user that has no RBAC permissions
(`kubectl --as`) and asserts the decision kube-apiserver enforced: whether the
object was created, changed, or deleted, and whether the request was
forbidden. They do not post payloads to Kyverno endpoints directly.

The tests need a dedicated cluster. They do not run in the standard
conformance cluster.

## Requirements

- Kubernetes 1.38 with the `ConditionalAuthorization` feature gate. The tests
  were run against `v1.38.0-beta.0`, which is a pre-release, not a final
  1.38 release.
- The `AuthorizationConditionsEnforcer` admission plugin. It is on by default
  in kube-apiserver 1.38 and runs after `ValidatingAdmissionPolicy` and
  mutating admission, so conditions see the mutated object.
- kind v0.33.0 or later, the same version for building the node image and for
  loading images.

## Create the cluster

From the repository root:

```sh
kind build node-image --type release v1.38.0-beta.0 --image kindest/node:v1.38.0-beta.0
make image-build-all
./test/conformance/chainsaw/authorize/_setup/setup-cluster.sh
```

The setup script does the following:

1. Creates the kind cluster `authz-138` with the feature gate on and
   kube-apiserver started with `--authorization-config`, initially
   [`authorization-config-bootstrap.yaml`](_setup/authorization-config-bootstrap.yaml)
   (Node and RBAC only).
2. Loads the Kyverno images, installs the `AuthorizingPolicy` CRD, and installs
   the chart with `admissionController.container.extraArgs.enableAuthorizingPolicies=true`.
3. Writes the webhook kubeconfig. kube-apiserver uses the host network and
   cannot resolve cluster DNS names, so the kubeconfig uses the `kyverno-svc`
   ClusterIP with `tls-server-name: kyverno-svc.kyverno.svc` and Kyverno's CA.
4. Replaces the authorization configuration with
   [`authorization-config.yaml`](_setup/authorization-config.yaml).
   kube-apiserver reloads it without a restart. The script waits until a probe
   policy takes effect.

## Authorizer order and recovery

The order is Node, RBAC, then Kyverno. Kyverno only sees requests that Node and
RBAC did not allow, so a Kyverno failure cannot lock out system components or
cluster administrators. Every Kyverno webhook uses `failurePolicy: NoOpinion`
and skips `system:` users.

To recover, copy `authorization-config-bootstrap.yaml` over
`/etc/kubernetes/authz/authorization-config.yaml` on the control-plane node
(the `AUTHZ_DIR` the script printed). kube-apiserver reloads the file within
about a minute (25s and 59s in the recorded run). If kube-apiserver does not
start, edit the file the same way; the static pod reads it at startup.

When the Kyverno admission controller is down, requests that only Kyverno would
allow fail with HTTP 500 (the webhook call fails and kube-apiserver does not
fall back to an allow), and requests that Node or RBAC allow are unaffected.

The configuration has several Kyverno webhooks. Dedicated test users select
them through `matchConditions`, so tests never change kube-apiserver
configuration:

| Webhook | Users | Purpose |
|---|---|---|
| `kyverno-cached` | `chainsaw-ttl-user` | Caches decisions for 120s |
| `kyverno-callback-rejected` | `chainsaw-cb-rejected` | Conditions reviews get HTTP 400 |
| `kyverno-callback-down` | `chainsaw-cb-down` | Conditions reviews get connection refused |
| `kyverno-unconditional` | `chainsaw-legacy-user` | No `conditionsReview`, so no conditional decisions |
| `kyverno` | all other users, including service accounts outside kube-system and kyverno | Default, no caching |

A webhook `authorizedTTL` or `unauthorizedTTL` of `0s` is defaulted to 5m and
30s. Use `cacheAuthorizedRequests: false` and `cacheUnauthorizedRequests: false`
to turn caching off.

## Run the tests

```sh
chainsaw test --test-dir ./test/conformance/chainsaw/authorize --parallel 1
```

Run the tests one at a time: AuthorizingPolicy objects are cluster-scoped.

| Test | Covers |
|---|---|
| `metadata-only` | Unconditional Allow and Deny from metadata for get, list, watch, create, and delete; deletion revokes access |
| `conditional-writes` | Create, update, patch, and delete decided from `object` and `oldObject`; dry-run; SubjectAccessReview API callers |
| `pods-containers` | Containers, init containers, and ephemeral containers (`pods/ephemeralcontainers`) |
| `residual-deny` | A satisfied Allow does not grant while a Deny or NoOpinion condition is unresolved |
| `service-accounts` | `serviceAccount` subject selectors apply to service accounts; kube-system service accounts are excluded |
| `unconditional-caller` | A webhook without `conditionsReview` gets unconditional, fail-closed answers |
| `reads` | Metadata-only reads, label-selector reads, and no grant from object-dependent read rules |
| `callback-failure` | Rejected and unreachable conditions reviews, and conditions that error |
| `policy-lifecycle` | Policy update and deletion between the authorization phase and the callback |
| `mutation-stage` | Conditions see the object after mutating admission (a MutatingAdmissionPolicy) |

## Not covered

- Feature gate off and a webhook without a conditions reviewer are covered by
  unit tests with the upstream webhook authorizer
  (`pkg/webhooks/resource/apol`), not in this cluster.
- Callback timeouts, replica failover, restarts, rolling upgrades, and version
  skew.
- `pods/exec` and other connect requests.
