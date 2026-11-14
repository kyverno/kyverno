# Conditional authorization with AuthorizingPolicy

`AuthorizingPolicy` (`policies.kyverno.io/v1alpha1`) lets Kyverno act as a
Kubernetes authorization webhook. With Kubernetes conditional authorization
([KEP-5681](https://github.com/kubernetes/enhancements/tree/master/keps/sig-auth/5681-conditional-authorization)),
Kyverno can return a decision that depends on the object being written, and
kube-apiserver asks Kyverno to resolve it during admission.

This page gets you started. For semantics, operational guidance, migration, and
what is not delivered, see [DESIGN.md](DESIGN.md).

> **Alpha.** Kyverno builds against the Kubernetes `v0.38.0-beta.0` libraries.
> That tag is a pre-release of Kubernetes 1.38, not a final release, and every
> test so far ran against kube-apiserver `v1.38.0-beta.0`. The API, the wire
> protocol, and this page can change without notice.

## Requirements

- kube-apiserver 1.38 (tested with `v1.38.0-beta.0`) started with:
  - `--feature-gates=ConditionalAuthorization=true` (alpha, off by default).
  - `--authorization-config=<file>`. Do not also set `--authorization-mode`.
- The `AuthorizationConditionsEnforcer` admission plugin. It is on by default
  in 1.38 and runs after mutating admission and `ValidatingAdmissionPolicy`, so
  conditions see the mutated object.
- The `AuthorizingPolicy` CRD. The `kyverno-api` chart that Kyverno depends on
  does not ship it yet, so apply
  `config/crds/policies.kyverno.io/policies.kyverno.io_authorizingpolicies.yaml`.
- The Kyverno admission controller started with
  `--enableAuthorizingPolicies=true` (default `false`).

## Enable Kyverno

Install the CRD, then set the flag through the Helm chart:

```yaml
admissionController:
  container:
    extraArgs:
      enableAuthorizingPolicies: true
```

Kyverno then serves these endpoints on its webhook service. It registers them
only after it has loaded and compiled all existing policies, so it never answers
from an empty policy set.

| Path | Accepts |
|---|---|
| `/authz/subjectaccessreview` | `authorization.k8s.io/v1` `SubjectAccessReview` |
| `/authz/conditionsreview` | `authorization.k8s.io/v1alpha1` `AuthorizationConditionsReview` |
| `/authz` | Both; dispatches on `apiVersion` and `kind` |

## Configure kube-apiserver

Use these files as a starting point:

- [`examples/authorization-config.yaml`](examples/authorization-config.yaml):
  Node, then RBAC, then Kyverno, with `conditionsReview` and caching turned off.
- [`examples/kyverno-authz-kubeconfig.yaml`](examples/kyverno-authz-kubeconfig.yaml):
  one context per endpoint.
- [`examples/authorization-config-bootstrap.yaml`](examples/authorization-config-bootstrap.yaml):
  the break-glass configuration with Node and RBAC only.

Key settings:

- `webhook.subjectAccessReviewVersion: v1`. kube-apiserver accepts
  `conditionsReview` only with `v1`, and Kyverno serves only `v1`.
- `webhook.conditionsReview.version: v1alpha1`. Without `conditionsReview`,
  kube-apiserver never offers conditional decisions, and Kyverno answers every
  request unconditionally and fails closed.
- `webhook.conditionsReview.kubeConfigContextName` names the kubeconfig context
  whose server is `/authz/conditionsreview`. If you leave it unset,
  kube-apiserver sends both kinds to the current context, which must then point
  at `/authz`.
- `failurePolicy: NoOpinion` keeps Kyverno one authorizer among others.
- `cacheAuthorizedRequests: false` and `cacheUnauthorizedRequests: false` make
  policy changes take effect on the next request. A TTL of `0s` does **not**
  turn caching off; kube-apiserver defaults it to 5m and 30s.
- kube-apiserver runs on the host network on most control planes and cannot
  resolve cluster DNS names. Point the kubeconfig at the `kyverno-svc`
  ClusterIP and set `tls-server-name: kyverno-svc.kyverno.svc`, as the e2e
  setup does.

`pkg/webhooks/resource/apol/docs_examples_test.go` validates both
authorization configuration files with the kube-apiserver `v0.38.0-beta.0`
loader, with the feature gate on and off, and validates every example policy
against the generated CRD schema.

## Write a policy

[`examples/owner-writes.yaml`](examples/owner-writes.yaml) lets members of the
`dev` group write ConfigMaps whose `owner` label matches their user name, and
denies writes to ConfigMaps labeled `protected`:

```yaml
apiVersion: policies.kyverno.io/v1alpha1
kind: AuthorizingPolicy
metadata:
  name: owner-writes
spec:
  matchSubjects:                 # required; terms are ORed
    - matchExpressions:          # requirements are ANDed
        - key: groups
          operator: In
          values: ["dev"]
  matchResources:                # required
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["*"]
        resources: ["configmaps"]
        verbs: ["*"]
  rules:                         # unordered; Deny > NoOpinion > Allow
    - name: owner-writes
      effect: Allow
      expression: >-
        has(object.metadata.labels) && 'owner' in object.metadata.labels &&
        object.metadata.labels.owner == request.user
    - name: protected
      effect: Deny
      expression: >-
        request.resourceAttributes.verb in ['create', 'update', 'patch', 'delete'] &&
        has(object.metadata.labels) && 'protected' in object.metadata.labels
    - name: reads
      effect: Allow
      expression: "request.resourceAttributes.verb in ['get', 'list', 'watch']"
```

You don't write anything stage-specific. Kyverno evaluates `request` when
kube-apiserver asks for authorization, and turns any part that needs `object`
or `oldObject` into a condition that kube-apiserver checks during admission.

Scope object-dependent Deny rules to write verbs, as the `protected` rule does.
On reads `object` is never known, so an unscoped object-dependent Deny denies
every read.

[`examples/label-selector-reads.yaml`](examples/label-selector-reads.yaml)
shows a read rule that uses the label selector of a list or watch request.

## Restrict who can write policies

Writing an `AuthorizingPolicy` is equivalent to cluster-admin. Grant write
access only to identities that already hold cluster-admin, as in
[`examples/policy-author-rbac.yaml`](examples/policy-author-rbac.yaml). The
Kyverno chart gives Kyverno read access and status updates only, and does not
aggregate write access into the `admin`, `edit`, or `view` roles.

## Try it

Check a decision without changing anything:

```sh
kubectl auth can-i update configmaps --as alice --as-group dev -n team-a
```

`kubectl auth can-i` sends a `SubjectAccessReview` without conditional support,
so an object-dependent rule answers `no`. Use a real or dry-run write to see the
enforced decision:

```sh
kubectl create configmap mine --from-literal=a=b -n team-a \
  --as alice --as-group dev --dry-run=server -o yaml   # forbidden: no owner label
```

The Chainsaw tests in
[`test/conformance/chainsaw/authorize`](../../../test/conformance/chainsaw/authorize/README.md)
create a kind cluster with these settings and assert enforced decisions.

## CLI

`kyverno apply` and `kyverno test` do not evaluate `AuthorizingPolicy`. Both
commands fail with the error `policy kind is not supported by the Kyverno CLI`
when the input contains one, instead of skipping it.

## Recover

If Kyverno blocks access you need, copy
[`examples/authorization-config-bootstrap.yaml`](examples/authorization-config-bootstrap.yaml)
over the active authorization configuration file on each control-plane node.
kube-apiserver reloads it without a restart, in about a minute in testing. See
[Break-glass and recovery](DESIGN.md#break-glass-and-recovery).
