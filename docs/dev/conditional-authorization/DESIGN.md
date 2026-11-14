# AuthorizingPolicy design

This page describes how Kyverno implements Kubernetes conditional authorization
with `AuthorizingPolicy`, how to operate it safely, and what it does not do.
For setup steps, see [README.md](README.md). The parent issue is
[kyverno/kyverno#15778](https://github.com/kyverno/kyverno/issues/15778).

Status: alpha. Kyverno builds against Kubernetes `v0.38.0-beta.0`, a
pre-release of Kubernetes 1.38. All end-to-end evidence comes from
kube-apiserver `v1.38.0-beta.0`. Nothing on this page is a promise about the
final 1.38 release.

## Kubernetes baseline

| Item | Value |
|---|---|
| Kubernetes libraries | `k8s.io/*` `v0.38.0-beta.0` (beta) |
| kube-apiserver tested | `v1.38.0-beta.0`, kind node image built with kind v0.33.0 |
| Feature gate | `ConditionalAuthorization` (alpha, off by default) |
| Admission plugin | `AuthorizationConditionsEnforcer` (on by default in 1.38) |
| Authorization request | `authorization.k8s.io/v1` `SubjectAccessReview` with `spec.authorizationOptions.handledDecisionTypes` |
| Conditions callback | `authorization.k8s.io/v1alpha1` `AuthorizationConditionsReview` |
| Authorizer configuration | `apiserver.config.k8s.io/v1` `AuthorizationConfiguration`, `webhook.conditionsReview.version: v1alpha1` |

kube-apiserver rejects an `AuthorizationConfiguration` that sets
`conditionsReview` when the feature gate is off.

## API

The canonical Go types live in `github.com/kyverno/api`, file
`api/policies.kyverno.io/v1alpha1/authorizing_policy.go`. The resource is
cluster-scoped, with plural `authorizingpolicies` and short name `apol`.

```yaml
apiVersion: policies.kyverno.io/v1alpha1
kind: AuthorizingPolicy
metadata:
  name: deny-exec-outside-ops
spec:
  matchSubjects:                     # required, 1-8 terms, ORed
    - matchExpressions:              # required, 1-8 requirements, ANDed
        - key: groups                # user | groups | serviceAccount
          operator: NotIn            # In | NotIn | Exists | DoesNotExist
          values: ["ops"]
  matchResources:                    # required
    resourceRules:                   # ORed
      - apiGroups: [""]
        apiVersions: ["*"]
        resources: ["pods/exec"]
        verbs: ["create"]
        namespaces: ["prod"]         # optional
        resourceNames: []            # optional
    nonResourceRules:                # ORed, RBAC path semantics
      - paths: ["/metrics"]
        verbs: ["get"]
  variables:                         # optional; cannot use object or oldObject
    - name: isOps
      expression: "'ops' in request.groups"
  rules:                             # required, 1-64, unordered, unique names
    - name: deny-exec
      effect: Deny                   # Allow | Deny | NoOpinion
      expression: "!variables.isOps" # one CEL expression returning bool
      description: ""                # optional, copied into the condition
```

### Subject selectors

| Key | `In` | `NotIn` | `Exists` | `DoesNotExist` |
|---|---|---|---|---|
| `user` | user is listed | user is not listed | invalid | invalid |
| `groups` | member of any listed group | member of none of the listed groups | has a group | has no group |
| `serviceAccount` (`<namespace>/<name>`) | is a listed service account | is not a service account, or is not listed | is a service account | is not a service account |

- `matchSubjects` is required. There is no implicit "everyone".
- The value `"*"` selects every value. It is valid only with `In` and as the
  only value. `user In ["*"]` selects every subject, including
  `system:anonymous`.
- `groups NotIn [a, b]` matches only subjects in neither group.
- The API module defines broad-selector warnings for a term that selects
  almost everyone (it has `user In ["*"]` or only negative requirements), a
  resource rule that uses `*` for resources, API groups, and verbs, and a
  non-resource rule with path `*` or `/*`. Kyverno does not surface these
  warnings yet: there is no policy validation webhook for this kind.

### Resource selectors

`matchResources` uses `admissionregistration`-style resource matching:
`pods` matches only the resource, `pods/exec` one subresource, `pods/*` the
resource and all of its subresources, `*/scale` one subresource of every
resource, and `*/*` everything. An omitted `namespaces` field matches any
namespace, cluster-scoped requests, and all-namespace lists. A set `namespaces`
never matches cluster-scoped or all-namespace requests. `resourceNames` follows
RBAC: requests without a name, such as `create` and `list`, never match a set
`resourceNames`.

### CEL environment

| Variable | Known when Kyverno authorizes | Known in the conditions review |
|---|---|---|
| `request` (the `SubjectAccessReview` spec: `user`, `groups`, `uid`, `extra`, `resourceAttributes`, `nonResourceAttributes`) | Yes | Captured into the condition |
| `variables` | Yes | Captured into the condition |
| `object` (null for delete) | No | Yes |
| `oldObject` (null for create) | No | Yes |

Only pure libraries are available: the CEL standard library and the
Kubernetes CEL libraries. Kyverno libraries that do I/O or read cluster state,
such as `resource`, `http`, `image`, and `globalContext`, are not.

## Evaluation

1. A policy applies when the request matches `matchSubjects` and
   `matchResources`. All rules of all applying policies form one flat,
   unordered set. Policy names, rule order, and the order in which Kyverno
   loaded policies never change the decision. Permutation property tests in
   `pkg/cel/policies/apol/engine` enforce this.
2. Kyverno partially evaluates each rule with `request` and `variables` known
   and `object` and `oldObject` unknown. A rule is `true`, `false`, an error,
   or a residual expression that depends on the unknowns.
3. Errors depend on the effect. A Deny or NoOpinion rule that errors applies.
   An Allow rule that errors does not apply, and Kyverno reports the error in
   `evaluationError`. A policy that fails to compile follows the same rules.
4. Precedence is Deny, then NoOpinion, then Allow.
5. If residuals remain, the caller handles conditional decisions, and the verb
   is `create`, `update`, `patch`, or `delete`, Kyverno returns a
   `ConditionsMap`. Otherwise it fails closed: Deny if a Deny residual
   remains, NoOpinion otherwise.
6. No applying policy or rule means NoOpinion. Kyverno never denies by default.

Each condition has `id` `<policy>/<rule>`, `type` `policies.kyverno.io/cel`,
and a CEL expression over `object` and `oldObject` only. Kyverno captures every
known value the expression still needs, so a condition does not depend on later
policy changes. Evaluating in two phases gives the same result as evaluating
the complete request, including errors. One conservative exception: a variable
that errors when Kyverno authorizes makes the referencing rule an error then.

In the conditions review, Kyverno evaluates only the conditions it receives and
never reads the current policies. Any replica can answer, and an in-flight
request is not affected by policy edits.

### Limits

Kyverno fails closed when a request exceeds these limits:

- More than 128 conditions in one decision.
- A `SubjectAccessReview` spec larger than 64 KiB.
- `object` plus `oldObject` larger than 8 MiB, or nested deeper than 64.
- A condition larger than 10240 bytes.
- The CEL per-expression cost limit, or the per-request runtime budget.
- More than 256 concurrent evaluations when the request context ends first.

## Supported requests

| Request | Behavior |
|---|---|
| `create`, `update`, `patch`, `delete` on resources and subresources | Unconditional or conditional decisions. Covered end to end, including dry-run, `pods/ephemeralcontainers`, and objects changed by mutating admission. |
| `get`, `list`, `watch` | Decided from request metadata only, including the label and field selectors of the request. Object-dependent residuals fail closed. |
| `deletecollection`, custom verbs | Decided from request metadata only. Object-dependent residuals fail closed. |
| Connect requests such as `pods/exec` | Not covered by tests. Use metadata-only rules for them. |
| Non-resource requests | Decided from request metadata only. |
| Callers that do not handle conditional decisions, such as the `SubjectAccessReview` API and `kubectl auth can-i` | Unconditional answers. Object-dependent residuals fail closed. |

## Operating Kyverno as an authorizer

### Default NoOpinion versus explicit restrictions

Kyverno returns NoOpinion unless a rule applies, so it never takes access away
by default. A request that no authorizer allows is denied by kube-apiserver
anyway, so an Allow rule grants access and a Deny rule only blocks access that a
later authorizer or another policy's Allow rule would grant.

Deny rules restrict RBAC grants only when Kyverno runs before RBAC. That is an
explicit choice with a larger blast radius; see the next section.

### Authorizer ordering

The tested order is Node, then RBAC, then Kyverno. Kyverno then sees only
requests that Node and RBAC did not allow, so a Kyverno outage or a bad policy
cannot lock out control-plane components or cluster administrators.

If you place Kyverno before RBAC so that Deny rules restrict RBAC grants:

- Exclude break-glass identities and every `system:` user with
  `matchConditions`.
- Expect a Kyverno outage to affect every request that the webhook matches.
- Test the configuration on a non-production cluster first. This order is not
  covered by Kyverno's end-to-end tests.

### Privileged-user bypass

kube-apiserver allows members of `system:masters` before it consults the
configured authorizer chain. No `AuthorizingPolicy` can restrict them. Other
privileged identities, such as control-plane components, nodes, and the
`kube-system` and Kyverno service accounts, are excluded by the example
configuration's `matchConditions`. Other service accounts are sent to Kyverno,
so `serviceAccount` subject selectors apply to them. Deny rules do not
apply to any request that a `matchConditions` expression excludes or that an
earlier authorizer allows.

### Policy-writer RBAC

Writing an `AuthorizingPolicy` is equivalent to cluster-admin: an Allow rule
can grant any verb on any resource to any subject. The RBAC escalation check
does not apply to custom resources, and Kyverno does not check that a policy
author holds the permissions their policy grants.

- Grant `create`, `update`, `patch`, and `delete` on `authorizingpolicies` only
  to identities that already hold cluster-admin. See
  [`examples/policy-author-rbac.yaml`](examples/policy-author-rbac.yaml).
- The Kyverno chart gives the admission controller `get`, `list`, and `watch` on
  `authorizingpolicies` and `update` and `patch` on `authorizingpolicies/status`
  only. It does not aggregate write access into `admin`, `edit`, or `view`.

### Endpoint exposure

The `/authz` endpoints do not authenticate the caller. A client that can reach
the Kyverno service can learn policy outcomes but cannot change what
kube-apiserver enforces. Restrict access to the service with network policies.

### Break-glass and recovery

1. Copy
   [`examples/authorization-config-bootstrap.yaml`](examples/authorization-config-bootstrap.yaml)
   over the active authorization configuration on each control-plane node.
2. kube-apiserver reloads the file without a restart. In the recorded e2e run
   it took 25 seconds to remove Kyverno and 59 seconds to restore it.
3. If kube-apiserver does not start, edit the same file; the static pod reads it
   at startup.
4. Members of `system:masters` keep access throughout, because kube-apiserver
   allows them before the chain.

While Kyverno is unreachable, requests that only Kyverno would allow fail with
HTTP 500, even with `failurePolicy: NoOpinion`. Requests that Node or RBAC
allow are not affected. An Allow condition that errors in the conditions review
also produces HTTP 500 rather than 403. Both outcomes fail closed.

### Cached decisions versus revocation

Kyverno does not cache decisions. kube-apiserver can:

- With caching on, kube-apiserver reuses a webhook decision, including a
  conditional one, until `authorizedTTL` or `unauthorizedTTL` expires. Policy
  edits and deletions do not revoke a cached decision before then. A TTL of
  `0s` is defaulted to 5 minutes (authorized) and 30 seconds (unauthorized), so
  it does not turn caching off.
- With `cacheAuthorizedRequests: false` and `cacheUnauthorizedRequests: false`,
  as in the example configuration, the next request sees the change.
- A request that already received a conditional decision is always finished
  with that decision's conditions, even if the policy changes before
  admission. The `policy-lifecycle` e2e test covers this.

Kyverno does not offer immediate revocation of decisions that kube-apiserver
has already made or cached.

### Upgrade and rollback

- **Alpha API.** `AuthorizingPolicy` `v1alpha1` has no conversion and no
  compatibility promise.
- **Kyverno upgrades.** Conditions carry everything they need, and the
  condition type `policies.kyverno.io/cel` identifies conditions environment
  version 1. During a rolling upgrade, any replica can evaluate conditions that
  another replica issued. An incompatible change to the conditions environment
  must use a new condition type, and unknown types evaluate as errors (fail
  closed). Rolling upgrades and version skew are not covered by e2e tests.
- **Kyverno restarts.** The admission controller serves the `/authz`
  endpoints only after it compiles every existing `AuthorizingPolicy`. While a
  replica starts, kube-apiserver cannot reach it, and requests that only
  Kyverno would allow fail as described in
  [Break-glass and recovery](#break-glass-and-recovery).
- **Kubernetes upgrades and downgrades.** The `conditionsReview` field, the
  `ConditionalAuthorization` gate, and `AuthorizationConditionsReview` exist
  only in Kubernetes 1.38 and later, and are alpha there. Before you downgrade
  kube-apiserver below 1.38, or move to a release that changes the alpha
  protocol, switch to
  [`examples/authorization-config-bootstrap.yaml`](examples/authorization-config-bootstrap.yaml).
  Kyverno is tested only against `v1.38.0-beta.0`.
- **Rolling back Kyverno.** Remove the Kyverno webhook from the
  `AuthorizationConfiguration` first, then disable `enableAuthorizingPolicies`
  or downgrade Kyverno, then delete the CRD and policies if you no longer need
  them.
- **Turning the feature gate off.** Remove `conditionsReview` from the
  configuration first. kube-apiserver rejects a configuration that sets it while
  the gate is off.

## CLI scope

`kyverno apply` and `kyverno test` reject `AuthorizingPolicy` with the error
`policy kind is not supported by the Kyverno CLI`, for standalone files, multi-document files,
and `List` objects. They never skip one silently. Evaluating a policy against
a `SubjectAccessReview` and an object from the CLI is not implemented.

## Migration from the PR #15866 experimental manifests

The experimental schema in
[kyverno/kyverno#15866](https://github.com/kyverno/kyverno/pull/15866) was never
released and has no conversion. Rewrite each manifest:

| PR #15866 | `v1alpha1` |
|---|---|
| `subjects: [{kind: Group, name: g}]` | `matchSubjects: [{matchExpressions: [{key: groups, operator: In, values: [g]}]}]` |
| `subjects: [{kind: User, name: u}]` | `key: user`, `values: [u]` |
| `subjects: [{kind: ServiceAccount, namespace: n, name: s}]` | `key: serviceAccount`, `values: ["n/s"]` |
| `subjects` omitted | `key: user`, `operator: In`, `values: ["*"]` (a broad selector) |
| `matchConstraints.resourceRules` | `matchResources.resourceRules`, with an explicit `apiVersions` list |
| Ordered rules, first match wins | Unordered rules, Deny > NoOpinion > Allow across all policies |
| `effect: Conditional` with `conditions[]` | One rule per condition: `name: <condition id>`, `effect: <condition effect>`, `expression: <rule expression> && <condition expression>` |
| `matchConditions` | Fold into each rule expression or into a variable |
| `failurePolicy` | Removed; error handling depends on the effect |
| `request.verb`, `request.namespace` | `request.resourceAttributes.verb`, `request.resourceAttributes.namespace` |
| `request.resourceLabels`, `request.oldResourceLabels` | `object.metadata.labels`, `oldObject.metadata.labels` |
| `request.object` | `object` |

Review migrated policies for first-match assumptions: an Allow rule listed
before a Deny rule no longer wins.

## Response to the design feedback

Lucas's feedback on
[kyverno/kyverno#15778](https://github.com/kyverno/kyverno/issues/15778)
([comment](https://github.com/kyverno/kyverno/issues/15778#issuecomment-4352065957))
has six points. The maintainers have not yet confirmed these dispositions.

| # | Feedback | Disposition |
|---|---|---|
| 1 | Label-selector-style subjects, explicit `*` instead of implicit match-all | **Implemented.** `matchSubjects` is required and uses `In`, `NotIn`, `Exists`, and `DoesNotExist` on `user`, `groups`, and `serviceAccount`, with an explicit `"*"`. Differences: requirements are grouped into ORed terms of ANDed requirements (a one-requirement term is the suggested form), single-value `In` and `NotIn` replace `Equals` and `NotEquals`, and broad-selector warnings exist in the API module but are not surfaced yet. Tests: `selectors_test.go` and the kyverno/api match tests. |
| 2 | Top-level `matchResources` instead of nested `matchConstraints` | **Implemented.** |
| 3 | Flat, unordered rules merged across policies, Deny, then NoOpinion, then Allow | **Implemented.** Rules use `name` instead of `id`, following the Kubernetes list-map convention. Tests: `TestNoFirstMatch`, `TestPermutationInvariance`. |
| 4 | Hide request stages from authors; use CEL partial evaluation | **Implemented.** Authors write one expression; Kyverno partially evaluates it and returns self-contained conditions. Tests: `TestPhasedMatchesFullEvaluation`, `TestResidualsAreSelfContained`, and the Chainsaw suite. |
| 5 | SMT decidability for privilege-escalation checks | **Deferred.** Arbitrary CEL, including macros such as `exists` and `map`, is not restricted to a decidable subset, and no escalation analysis exists. Rationale: it needs a restricted language and a solver, which is a separate design. Mitigation: policy authoring is limited to cluster administrators. |
| 6 | Reads, and friendlier label and field selector authorization | **Partly implemented, rest deferred.** Read rules can use request metadata, including `request.resourceAttributes.labelSelector` and `fieldSelector`; object-dependent read rules never grant. Automatic selector inference and read filtering are deferred for the same reason as point 5. |

## Not delivered

These capabilities are out of scope and are not provided:

- **SMT decidability.** Kyverno does not restrict CEL to a decidable subset and
  does not analyze policies with a solver.
- **Privilege-escalation analysis.** Kyverno does not detect that a policy lets
  someone expand their own privileges.
- **Selector inference.** Kyverno does not derive label or field selectors from
  object-dependent read rules, and does not filter list results.
- **Cedar.** No Cedar policy language or Cedar-based authorizer. That work is
  separate.
- **Immediate revocation** of decisions that kube-apiserver has cached.
- **CLI evaluation** of `AuthorizingPolicy`.
- **Packaged CRD.** The `kyverno-api` chart does not ship the
  `AuthorizingPolicy` CRD yet.

## Testing

- Unit tests: `pkg/cel/policies/apol/...` and `pkg/webhooks/resource/apol`,
  including round trips through the upstream kube-apiserver webhook authorizer.
- Example validation: `pkg/webhooks/resource/apol/docs_examples_test.go` loads
  both authorization configuration files and the kubeconfig with the upstream
  loaders, validates every example policy against the generated CRD schema
  (OpenAPI and `x-kubernetes-validations` rules), compiles it, checks its
  decisions, and checks the policy-author role.
- End to end: [`test/conformance/chainsaw/authorize`](../../../test/conformance/chainsaw/authorize/README.md),
  run against kube-apiserver `v1.38.0-beta.0`. This suite is not yet in CI.

## Open questions

- Subject selector shape (terms versus a flat list) and the rule key name
  (`name` versus `id`) need confirmation from Lucas and the maintainers.
- Condition type `policies.kyverno.io/cel` versus a future built-in
  `authorizer.kubernetes.io/cel`. Kubernetes `v0.38.0-beta.0` has no built-in
  evaluator for the latter.
- Whether broad selectors should be rejected or warned about, and where Kyverno
  surfaces the warnings.
- Whether the CLI should gain an identity- and action-aware mode.
- Whether the `/authz` endpoints should require a client certificate.
