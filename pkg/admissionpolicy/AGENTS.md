# AGENTS.md — pkg/admissionpolicy

Helpers for the Kubernetes-native `ValidatingAdmissionPolicy`/`MutatingAdmissionPolicy` objects Kyverno generates
and evaluates. See [ARCHITECTURE.md](../../ARCHITECTURE.md) for where this sits relative to `pkg/cel`.

## `GetGVRs` builds GVRs straight from policy strings — no mapper lookup (except for `*` groups/versions)

`GetGVRs` (`validate.go`) turns `MatchResources.ResourceRules` into `schema.GroupVersionResource`s without
consulting the `RESTMapper` for concrete groups and versions (see the wildcard note below). Its caller,
`pkg/controllers/deleting/controller.go`, hands the result to the dynamic client, which joins group, version and
resource into the request path with `path.Join` — and `path.Join` resolves `.`/`..` and collapses empty segments.
An unvalidated component can therefore redirect the controller's LIST, under its own ServiceAccount, to a different
API path.

Invariants to keep when touching it:

- Every component goes through `isUnsafeGVRSegment`, which wraps the shared `dclient.ValidatePathSegment`
  (`pkg/clients/dclient/path_segment.go`). Unsafe rules are **skipped and logged**, not rewritten — an inert rule
  must stay inert.
- **Only the core API group (`group == ""`) may be empty.** An empty `version` or `resource` is rejected: it would
  collapse in `path.Join` (e.g. `/api/v1` for an empty resource).
- `*` wildcards in `resource` are valid and must keep resolving.
- A `*` in `apiGroups` or `apiVersions` is the one case that does consult the mapper: `resolveWildcardGVRs`
  expands it to the served resources (preferred version per group/resource, asked via `ResourceFor` because
  `ResourcesFor` ignores version priority), because the dynamic client would otherwise LIST a literal `*` path
  segment and get a 404. A wildcard that matches nothing (`meta.IsNoMatchError`, e.g. a CRD not installed yet) is
  skipped and logged. Any other mapper error (discovery unavailable) is returned by `GetGVRs`, so the deleting
  controller fails the run and requeues instead of recording `lastExecutionTime` for a run that deleted nothing.
  With a nil mapper, or when the resource is also `*` (the RESTMapper cannot expand a resource wildcard), the GVR
  passes through unchanged.

`ValidatePathSegment` is not a complete chokepoint — any new code that builds a GVR or subresource from
policy-supplied strings and reaches the dynamic client has to call it explicitly. The same applies to
`pkg/cel/libs/resource_path.go` and `pkg/background/mpol/processor.go`.
