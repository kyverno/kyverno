# Generated resource provenance

Run this test in isolation with `protectManagedResources=false` (the default),
before another generate policy watches Secrets. It needs `kubectl`, `jq`, and
`timeout` and grants the controllers Secret access.

Before installing the policy, the test plants two Secrets at the intended output
names with real trigger and source UIDs and downstream routing labels. One is
unsigned; the other carries a forged provenance annotation. This administrative
setup models persisted metadata. Only `prepare-existing-targets.sh` impersonates
the background controller, discovering its configured service account from the
single background-controller Deployment in `${KYVERNO_NAMESPACE:-kyverno}`.
All subsequent operations use the runner's ordinary identity.

The test verifies that an unchanged-label update, forged downstream deletion,
and clone-source data change cannot enqueue generation for the planted targets.
Each operation watches UpdateRequests from the preceding list resourceVersion
for ten seconds, including short lived requests. The unsigned target retains its
data and the deleted target stays absent. A new trigger then proves that genuine
generation still works, ordinary edits synchronize, and deleted targets are
recreated. Finally, policy deletion removes the authenticated target while
preserving the unsigned one.

Go regression tests cover the remaining permutations without extra live fixtures:
`pkg/webhooks/handlers/protect_provenance_test.go` covers annotation write guards;
`pkg/webhooks/resource/generation/provenance_test.go` covers copied and mismatched
identities and per-target clone-source verification;
`pkg/policy/generate_cleanup_discovery_test.go` covers kind-only and versioned
cleanup for data, clone, cloneList, and foreach rules; and
`pkg/background/generate/cleanup_provenance_test.go` covers authenticated deletion
and concurrent replacements. The provenance package and generation tests cover
key lifecycle, signing failures, and retry recovery.

## Compatibility and key lifecycle

Preserve the immutable `kyverno-generate-provenance` Secret in the Kyverno
installation namespace across restarts and upgrades. Admission creates it and
background reads it; missing, invalid, or replaced keys invalidate existing
provenance. Custom RBAC must allow admission to create/read this key, background
to read it, and background to `patch` every generated target kind, including
cloneList and foreach targets. Policy authors do not need `patch` solely for this
annotation. The default chart already grants these permissions for its supported
kinds.

Unsigned older downstreams are ignored by non-trigger events, clone-source
traversal, and cleanup. An authorized synchronized trigger evaluation or explicit
`generateExisting` evaluation can establish provenance; policy-update label scans
cannot. Targets skipped by `synchronize: false` remain unsigned. Pending cleanup
without the original policy UID is ignored. Ordinary source restores, unrelated
annotations, and data edits remain supported; full-object downstream replacements
must preserve the controller-owned provenance annotation.

A key snapshot precedes target writes. If a target write succeeds but its
provenance patch fails, a receipt on the UpdateRequest can recover that exact
server-assigned identity without rewriting data, including for unsynchronized
rules. Changed routing or replacement identities are rejected. Pending receipts
remain failed when clone sources disappear or cloneList/foreach results become
empty; restoring sources can allow recovery within the normal retry limit.
Creation and receipt persistence are separate API writes: a crash before saving
the receipt leaves the target unsigned. Restoring the original key does not
replay exhausted retries; generation then requires a new trusted trigger or an
explicit `generateExisting` evaluation, and cleanup requires separate
reconciliation.
