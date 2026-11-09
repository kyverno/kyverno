# AGENTS.md — pkg/admissionpolicy

Helpers for the Kubernetes-native `ValidatingAdmissionPolicy`/`MutatingAdmissionPolicy` objects Kyverno generates
and evaluates. See [ARCHITECTURE.md](../../ARCHITECTURE.md) for where this sits relative to `pkg/cel`.

## `GetGVRs` builds GVRs straight from policy strings — no mapper lookup

`GetGVRs` (`validate.go`) turns `MatchResources.ResourceRules` into `schema.GroupVersionResource`s without
consulting the `RESTMapper` (the `mapper` argument only feeds kind resolution elsewhere in the file). Its caller,
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

`ValidatePathSegment` is not a complete chokepoint — any new code that builds a GVR or subresource from
policy-supplied strings and reaches the dynamic client has to call it explicitly. The same applies to
`pkg/cel/libs/resource_path.go` and `pkg/background/mpol/processor.go`.
