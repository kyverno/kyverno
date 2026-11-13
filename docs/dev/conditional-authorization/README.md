# Conditional authorization with AuthorizingPolicy

`AuthorizingPolicy` (`policies.kyverno.io/v1alpha1`) lets Kyverno act as a
Kubernetes authorization webhook. With Kubernetes conditional authorization
([KEP-5681](https://github.com/kubernetes/enhancements/tree/master/keps/sig-auth/5681-conditional-authorization)),
Kyverno can return a decision that depends on the object being written, and
kube-apiserver asks Kyverno to resolve it during admission.

This feature is alpha. It is built against the Kubernetes `v0.38.0-beta.0`
libraries, which are a pre-release of Kubernetes 1.38, not a final release.
Conditional decisions need a kube-apiserver at 1.38 or later with the
`ConditionalAuthorization` feature gate on.

The canonical API types live in `github.com/kyverno/api`
(`api/policies.kyverno.io/v1alpha1/authorizing_policy.go`). The authoring and
evaluation contract is tracked in kyverno/kyverno#15778.

## How it works

1. kube-apiserver sends an `authorization.k8s.io/v1` `SubjectAccessReview`.
   When `webhook.conditionsReview` is configured and the gate is on, the
   request carries `spec.authorizationOptions.handledDecisionTypes` with
   `ConditionsMap` and `Union`.
2. Kyverno selects the policies whose `matchSubjects` and `matchResources`
   match, and partially evaluates every rule with `request` known and
   `object`/`oldObject` unknown. Precedence is Deny, then NoOpinion, then
   Allow, across all matching policies.
3. Kyverno answers with `status.allowed`, `status.denied`, or neither
   (NoOpinion). If the answer depends on the object, and the caller opted in,
   and the verb is `create`, `update`, `patch`, or `delete`, Kyverno returns
   `status.conditionalDecision` of type `ConditionsMap`. In that case
   `allowed`, `denied`, `reason`, and `evaluationError` are all zero.
   Each condition has `id` `<policy>/<rule>`, `type` `policies.kyverno.io/cel`,
   and a residual CEL expression over `object` and `oldObject` only.
4. During admission, kube-apiserver sends an `authorization.k8s.io/v1alpha1`
   `AuthorizationConditionsReview` with `request.decision` (the snapshot from
   step 3) and `request.admissionRequest`. Kyverno evaluates only that snapshot,
   never the current policies, echoes `request.admissionRequest.uid` in
   `response.uid`, and returns an unconditional `response.decision`.

Kyverno fails closed whenever it cannot return a conditional decision: callers
that did not opt in, reads and other non-conditional verbs, non-resource
requests, and more than 128 conditions. It returns Deny when a Deny condition
remains, and NoOpinion otherwise. Malformed SubjectAccessReview requests are
denied. A malformed conditions review fails closed to Deny when the decision
could deny, and to NoOpinion otherwise. Requests that cannot be decoded, or a
conditions review without `request.admissionRequest.uid`, get HTTP 400, and
kube-apiserver then applies its own failure handling.

## Endpoints

The Kyverno admission controller serves these endpoints on its webhook
service when it starts with `--enableAuthorizingPolicies=true`. The
`AuthorizingPolicy` CRD must be installed first; the `kyverno-api` chart that
Kyverno depends on does not ship it yet, so apply
`config/crds/policies.kyverno.io/policies.kyverno.io_authorizingpolicies.yaml`.

| Path | Accepts |
|---|---|
| `/authz/subjectaccessreview` | `authorization.k8s.io/v1` `SubjectAccessReview` |
| `/authz/conditionsreview` | `authorization.k8s.io/v1alpha1` `AuthorizationConditionsReview` |
| `/authz` | Both; dispatches on `apiVersion` and `kind` |

Enable the flag through the chart with:

```yaml
admissionController:
  container:
    extraArgs:
      enableAuthorizingPolicies: true
```

## Configure kube-apiserver

Start kube-apiserver with `--feature-gates=ConditionalAuthorization=true` and
`--authorization-config=<file>`. See
[`examples/authorization-config.yaml`](examples/authorization-config.yaml) and
[`examples/kyverno-authz-kubeconfig.yaml`](examples/kyverno-authz-kubeconfig.yaml).

Key settings:

- `webhook.subjectAccessReviewVersion: v1`. kube-apiserver accepts
  `conditionsReview` only with `v1`, and Kyverno serves only `v1`.
- `webhook.conditionsReview.version: v1alpha1`. Without `conditionsReview`,
  kube-apiserver never advertises conditional support and Kyverno answers
  unconditionally.
- `webhook.conditionsReview.kubeConfigContextName` names a context in the
  webhook kubeconfig whose server is `/authz/conditionsreview`. If you leave it
  unset, kube-apiserver sends both kinds to the current context, which must
  then point at `/authz`.
- `failurePolicy: NoOpinion` keeps Kyverno one authorizer among others: a
  Kyverno NoOpinion lets the next authorizer decide.

`pkg/webhooks/resource/apol/docs_examples_test.go` validates both example files
with the kube-apiserver `v0.38.0-beta.0` loader and validation.

## Example policy

[`examples/owner-writes.yaml`](examples/owner-writes.yaml) lets members of the
`dev` group write ConfigMaps whose `owner` label matches their user name, and
denies writes to ConfigMaps labelled `protected`.

Scope object-dependent Deny rules to write verbs. On reads, `object` is never
known, so an object-dependent Deny residual fails closed to Deny.

## Limitations

- Writing an `AuthorizingPolicy` is equivalent to cluster-admin: an Allow rule
  can grant anything. The Helm chart gives Kyverno read access and status
  updates only, and does not aggregate write access into the `admin`, `edit`,
  or `view` roles.
- Variables cannot reference `object` or `oldObject`.
- Only pure CEL libraries are available. Kyverno libraries that do I/O, such as
  `resource`, `http`, and `globalContext`, are not.
- Kyverno does not analyze privilege escalation, restrict CEL to an
  SMT-decidable subset, or infer label or field selectors for reads.
- The webhook endpoints do not authenticate the caller. Any client that can
  reach the Kyverno service can ask for decisions, but cannot change what
  kube-apiserver enforces.
