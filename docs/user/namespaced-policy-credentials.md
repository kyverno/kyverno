# Namespaced Policy credentials and CEL parameters

Namespaced `kyverno.io/v1` Policies resolve registry credential Secret names in
the policy namespace. Explicit `namespace/name` references must use that same
namespace. Admission rejects foreign references; runtime checks also protect
policies stored before the upgrade.

This applies to `context[].imageRegistry.imageRegistryCredentials.secrets` and
`verifyImages[].imageRegistryCredentials.secrets`, including nested context
entries. Context credential references remain literal names. `verifyImages`
continues to substitute rule variables, then checks the resulting references.
Validation checks those references even when `required` is false or an image is
excluded by `skipImageReferences`; neither path needs to load the Secret.

For a Policy in `tenant-a`, both `regcred` and `tenant-a/regcred` select the Secret
in `tenant-a`. A reference to `kyverno/regcred` or `tenant-b/regcred` is rejected.
Before upgrading, replace dependencies on installation-namespace credentials with
credentials appropriate for that tenant. Do not copy administrator credentials
into tenant namespaces. Missing credentials can cause private registry access to
fall back to anonymous access and fail.

The controllers resolve tenant credentials with bounded, single-Secret API GETs.
They do not start tenant Secret informers or require list/watch access. Give only
the required controllers access to the particular Secret, for example:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kyverno-registry-credential
  namespace: tenant-a
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["regcred"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kyverno-registry-credential
  namespace: tenant-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kyverno-registry-credential
subjects:
- kind: ServiceAccount
  name: kyverno-admission-controller
  namespace: kyverno
```

Use your deployment's actual ServiceAccount names. Add the background and reports
controller ServiceAccounts only if those controllers evaluate the policy.
An API authorization failure is returned; it never falls back to credentials in
the installation namespace. Existing administrator-configured registry Secrets
and ClusterPolicy defaults retain their lookup behavior.

Resource `imagePullSecrets` are controlled by the resource author, not the policy
author, so for every policy kind Kyverno resolves them as the kubelet does: only in
the resource namespace, using Kyverno's existing Secret cache. A name that is not a
valid Secret name, such as `kyverno/regcred`, is ignored rather than read as
`namespace/name`.

The same boundary applies to `verifyImages` and `validate.manifests` attestor keys
in a namespaced Policy: `keys.secret`, and `k8s://<namespace>/<name>` references
in `keys.publicKeys` or `keys.kms`. An empty `keys.secret.namespace` resolves to the Policy namespace, and
a reference to any other namespace is forbidden. Admission rejects statically
foreign references; references containing variables are admitted and checked
again at runtime after substitution. ClusterPolicy key references are unchanged.

Policies that read a key from the installation namespace, for example
`keys.secret: {name: cosign-pub, namespace: kyverno}` or
`publicKeys: k8s://kyverno/cosign-pub`, must move to a key Secret in the Policy
namespace or to a ClusterPolicy. A public key can also be inlined in
`keys.publicKeys`.

For `Policy.spec.rules[].validate.cel`, both named and selector-based parameter
lookups stay in the policy namespace. An omitted `paramRef.namespace` defaults
there; an explicit namespace must match. Cluster-scoped `paramKind` is rejected
at admission and runtime. Install the parameter CRD before creating the Policy:
admission checks the exact requested served API version and fails if it cannot
establish that the parameter kind is namespaced.

Move tenant parameters to an appropriate namespaced resource such as a ConfigMap.
If cluster-wide parameters are required, use an administrator-managed
ClusterPolicy. Native Kubernetes ValidatingAdmissionPolicy and
MutatingAdmissionPolicy parameter behavior is unchanged.
Context entries in a namespaced CleanupPolicy are confined to the policy
namespace, in both the cleanup controller and `kyverno apply`: `configMap` and
`apiCall` entries must target that namespace, and `globalReference` is denied.
ClusterCleanupPolicy context lookups are not restricted. Cleanup resource
selection still respects policy namespace.

The namespace boundary takes effect on upgrade. Stored Policies using forbidden
credentials or parameters report an error on evaluation and must be corrected
before their next update. The conformance fixtures under
`test/conformance/chainsaw/validate/namespaced-admission-scope` and
`test/conformance/chainsaw/verify-images/namespaced-credential-scope` exercise
admission; unit tests cover runtime confinement and compatibility.
