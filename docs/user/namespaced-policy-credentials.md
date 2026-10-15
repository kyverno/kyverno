# Namespaced Policy credentials and CEL parameters

Namespaced `kyverno.io/v1` Policies resolve registry credential Secret names in
the policy namespace. Explicit `namespace/name` references must use that same
namespace. Admission rejects foreign references; runtime checks also protect
policies stored before the upgrade.

This applies to `context[].imageRegistry.imageRegistryCredentials.secrets` and
`verifyImages[].imageRegistryCredentials.secrets`, including nested context
entries. Context credential references remain literal names. `verifyImages`
continues to substitute rule variables, then checks the resulting references.

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
the installation namespace. Existing administrator-configured registry Secrets,
ClusterPolicy defaults, and resource imagePullSecrets retain their lookup behavior.

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

The namespace boundary takes effect on upgrade. Stored Policies using forbidden
credentials or parameters report an error on evaluation and must be corrected
before their next update. The conformance fixtures under
`test/conformance/chainsaw/validate/namespaced-admission-scope` and
`test/conformance/chainsaw/verify-images/namespaced-credential-scope` exercise
admission; unit tests cover runtime confinement and compatibility.
