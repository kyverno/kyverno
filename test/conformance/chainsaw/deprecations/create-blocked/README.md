# create-blocked

Verifies the 1.20 write-time hard block on legacy `kyverno.io` policy APIs
(https://github.com/kyverno/kyverno/issues/17483): creating a `ClusterPolicy`, `Policy`,
`ClusterCleanupPolicy`, `CleanupPolicy`, or legacy `PolicyException` — in any served version — is
denied with a migration-guiding error, under the default `features.blockLegacyPolicyAPIs.enabled:
true` setting.

This specifically covers `kyverno.io/v2` for `ClusterCleanupPolicy`/`CleanupPolicy`/
`PolicyException`, which is each resource's storage version, to prove the admission webhook rules
actually intercept the version most manifests use (previously only `v2beta1`/`v2alpha1` were
registered, so a `v2` write bypassed the webhook entirely).
