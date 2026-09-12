# API versioning and stability

Canonical source for Kyverno's API versioning, stability, and deprecation rules. `AGENTS.md`, `api/AGENTS.md`, and
`docs/dev/api/README.md` all link here instead of restating these rules — if you're updating them, this is the only
file to edit.

## Where the types live

- `kyverno.io`, `wgpolicyk8s.io` (as `policyreport`), and `reports.kyverno.io` are defined **in this repo**, under
  `api/kyverno/{v1,v1beta1,v2,v2alpha1,v2beta1}`, `api/policyreport/v1alpha2`, and `api/reports/v1`.
- **`policies.kyverno.io`** (the CEL-based types: ValidatingPolicy, MutatingPolicy, GeneratingPolicy, DeletingPolicy,
  ImageValidatingPolicy, PolicyException, and their Namespaced\* variants) is defined in the **separate
  `github.com/kyverno/api` Go module**, not in this repo's `api/` directory. It's pinned in `go.mod` and imported
  like `policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"`. Only the generated CRD manifests
  (`config/crds/policies.kyverno.io/*.yaml`) and consuming Go code live here. See
  [repo-boundaries.md](./repo-boundaries.md) for why (no backfilled ADR exists yet for this decision).

## Stability tiers

| Version tier | Compatibility guarantee | Minimum deprecation notice |
|---|---|---|
| `v1alpha1` / `v2alpha1` | No guarantee; fields may be renamed, removed, or restructured between minor releases | 1 minor release |
| `v1beta1` / `v2beta1` | Breaking changes discouraged; deprecated fields are retained with a `// Deprecated.` Go comment and noted in release notes | 2 minor releases |
| `v1` / `v2` (stable) | No breaking changes; deprecated fields are retained but may be removed after the notice period | 3 minor releases |

## Rules

- **Adding a new kind:** never add new types to `v1` (`kyverno.io`) — introduce them at `v2alpha1` and promote as
  they stabilize. New CEL-policy types go in `policies.kyverno.io` (in `github.com/kyverno/api`, not here).
- **Adding an attribute:** only *optional*, backward-compatible attributes can be added to an existing version
  without a new version. A new *required* field, or stricter validation that could reject writes that were
  previously valid, breaks existing clients and older controller versions — that needs a new version instead.
- **Deleting an attribute:** never delete in place within a version. Mark it deprecated and remove after 3 minor
  releases (stable tiers) per the notice periods above.
- **Modifying an attribute:** never modify in place. Deprecate the existing attribute and add a new one following
  the same compatibility rules.
- **Stable references:** newer API versions may reference older stable types; never the reverse (a `v1` resource
  must not reference a `v2alpha1` type; a `v2alpha1` type referencing `v1` is fine).

**Deprecation procedure:** mark the field/type deprecated in the Go struct comment and OpenAPI schema description →
announce in release notes with the planned removal minor version → remove once the notice window has elapsed. The
published deprecation schedule for legacy `kyverno.io` v1 types (`ClusterPolicy`, `Policy`) lives at
[kyverno.io's deprecation schedule](https://kyverno.io/docs/policy-types/overview/#deprecation-schedule-for-legacy-types)
— that page, not this one, is authoritative for which specific fields are currently deprecated. Their migration path
is **not** to a `kyverno.io/v2` version of the same types — `api/kyverno/v2` has no `ClusterPolicy`/`Policy` type at
all — it's to the CEL `policies.kyverno.io` types (`ValidatingPolicy`, `MutatingPolicy`, `GeneratingPolicy`,
`ImageValidatingPolicy`), per the deprecation marker on `v1.ClusterPolicy` itself (quoted in
[api/AGENTS.md](../../../api/AGENTS.md)).

None of the above is enforced by a linter or CI check today — it's a reviewer-enforced convention. If you're adding
tooling to check it, start here.

## A real worked example of the full lifecycle: `GlobalContextEntry`

`v2alpha1` is not an empty staging ground waiting for the next new type — right now it holds exactly one type,
`GlobalContextEntry`, and its lifecycle is further along than a simple two-version story. Three versions currently
exist, added in this order: `v2alpha1` (2024, `#9591`) → `v2beta1` (Oct 2025, `#14165`) → `v2` (Dec 2025, `#14540`).
`api/kyverno/v2alpha1/global_context_entry_types.go` carries
`+kubebuilder:deprecatedversion:warning="kyverno.io/v2alpha1 GlobalContextEntry is deprecated; use kyverno.io/v2
GlobalContextEntry"` — but the CRD's actual `storage: true` version right now is **`v2beta1`**
(`api/kyverno/v2beta1/global_context_entry_types.go` carries `+kubebuilder:storageversion`), not `v2`; `v2` is the
newest, stable-named version but hasn't been cut over as the storage version yet, and carries no deprecation marker
of its own. All three are still `served: true`
(`config/crds/kyverno/kyverno.io_globalcontextentries.yaml`). Don't assume "the stable-named version is the storage
version" — check the CRD's `storage:` flags directly. And don't read "`v2alpha1` currently has content" as evidence
it's a landing zone for a brand-new type today — check what's actually in a version before assuming its role.
