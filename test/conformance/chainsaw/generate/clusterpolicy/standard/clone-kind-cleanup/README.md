# Kind-only cloneList cleanup

This test generates ConfigMaps from `cloneList.kinds: [ConfigMap]` in a direct
rule and a foreach rule. Removing the direct rule must delete its authenticated
target and leave the foreach target intact. Deleting the policy must then delete
the foreach target. Both clone sources and unsigned objects carrying the same
downstream routing labels must survive both operations unchanged.

The test uses only its named namespaces, policy, and aggregated ConfigMap role.
It requires `kubectl`, `jq`, and permission to impersonate the configured
background-controller service account. Only `prepare-unsigned.sh` impersonates
that account to create unsigned fixture objects; it copies labels from genuine
targets but leaves out their provenance annotations. This models persisted
unsigned metadata without changing controller configuration. The script finds
the background-controller Deployment in `${KYVERNO_NAMESPACE:-kyverno}`, so
custom installation namespaces and service account names are supported.
