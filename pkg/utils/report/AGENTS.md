# Report revision contract

- `PolicyRevision` is the value written by `SetPolicyLabel` and compared by
  `SelectorPolicyNotEquals` and the background reports controller. Keep all
  readers, writers, and CEL informer update predicates consistent.
- CEL ValidatingPolicy, MutatingPolicy, ImageValidatingPolicy, and their
  namespaced variants use `EffectivePolicyRevision`. Include object UID,
  generation, reporting exclusion presence, category/severity/scored annotations,
  effective autogen configuration, and applicable native-policy generated state.
  Generic status conditions, unrelated metadata, and resourceVersion changes
  must not invalidate these revisions.
- Native ValidatingAdmissionPolicy and MutatingAdmissionPolicy keep their
  resourceVersion revisions, as do their bindings. Do not silently extend the
  CEL revision scheme to additional policy kinds.
- Revisions must be deterministic Kubernetes label values (at most 63
  characters). A legacy resourceVersion label requires one successful refresh;
  replace stale results rather than appending duplicates. Failed scans must not
  persist new revision labels or partial results.
- Cover revision changes and no-op updates in `policy_revision_test.go`, and
  controller persistence, migration, and retry behavior under
  `pkg/controllers/report/background`. Run both packages and the report
  controllers' race tests after changing this contract.
