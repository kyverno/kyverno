# Namespaced image policy registry credentials

`NamespacedImageValidatingPolicy` resolves bare registry Secret names in the
policy namespace. An explicit `namespace/name` reference must use that same
namespace. This applies to `spec.credentials.secrets` and attestor signature
pull secrets (`spec.attestors[].cosign.source.PullSecrets`), including attestors
passed dynamically to CEL verification functions.

For a policy in `tenant-a`, `registry` and `tenant-a/registry` refer to the
`registry` Secret in `tenant-a`. `tenant-b/registry` and `kyverno/registry` are
rejected at policy admission and at runtime. Existing stored policies with
foreign references report a policy error after upgrade.

Before upgrading, provision tenant-specific credentials in the policy namespace
and update foreign references. A policy that previously relied on a bare name to
select a Kyverno installation Secret must also migrate. Do not copy administrator
credentials to tenant namespaces.

Grant the RBAC before upgrading, not after. A Secret that is simply absent is
skipped, but a Secret the controller is *denied* fails credential resolution
outright, so the policy then errors on every matching image rather than only on
private ones.

The controller uses its existing configured Secret caches, or an authorized GET
for the particular tenant Secret. The change does not create new Secret informers
or grant RBAC permissions. Grant only `get`, and only for the Secrets the policy
names, as in
[the example Role and RoleBinding](namespaced-image-policy-credentials/secret-rbac.yaml).
Adjust the namespace, the controller ServiceAccount names and `resourceNames` for
the installation — `resourceNames` must list every Secret the policy references,
or the ones left out are denied.

`kubectl-kyverno apply` and `kubectl-kyverno test` resolve these Secrets from
their own informer cache and never issue the authorized GET, so they need a
kubeconfig that can list and watch Secrets in the policy namespace.

Cluster-scoped `ImageValidatingPolicy` credentials, operator-configured global
credentials and resource `imagePullSecrets` keep their existing behavior. This
change does not modify legacy `Policy` or `ClusterPolicy` handling.

Admission and runtime tests cover accepted tenant references, denied foreign
references, cluster-scoped compatibility, cached-policy immutability, and
policies stored before the upgrade still reporting a policy error through the
real reconciler. Deployed admission fixtures are in
`test/conformance/chainsaw/namespaced-image-validating-policies/credential-scope`.
