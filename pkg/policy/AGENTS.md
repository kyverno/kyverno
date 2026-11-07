# pkg/policy — policy lifecycle and background trigger discovery

This package owns policy lifecycle reconciliation and submission of `UpdateRequest` objects
for asynchronous GeneratingPolicy and MutatingPolicy operations. It is **not** the CEL
policy evaluator; see [`pkg/cel`](../cel/AGENTS.md) for compilation/evaluation
and [`pkg/background`](../background/AGENTS.md) for consuming the queued work.

## Trigger enumeration is shared by generation and mutation

- `getGpolTriggers` in [`gpol.go`](gpol.go) is called by cluster/namespaced
  GeneratingPolicy reconciliation and by `createTriggerURs` in
  [`mpol.go`](mpol.go), for MutatingPolicies whose target constraints use
  an expression. Changes to discovery can affect **both** policy kinds.
- `MatchResources.resourceRules` uses Kubernetes admissionregistration
  matching semantics. `apiGroups: ["*"]` must be expanded against discovery
  before calling `KindFor` or listing resources; `""` is the concrete core API
  group, not an alias for `"*"`.
- For wildcard **versions**, choose one preferred served version per
  concrete group/resource to avoid duplicate work. For **explicit** version
  lists, retain every requested served version: exclusion rules can filter
  one version but allow another.
- Resolve the underlying kind from a base resource name (e.g. `pods` for
  `pods/status`). Preserve the original subresource for matching exclusions
  and for the mutation request.
- Honor `Rule.Scope` by comparing it with the discovered RESTMapping scope
  before listing. Do not infer cluster scope from an empty namespace on
  individual objects.
- Apply selectors, names, and `excludeResourceRules` before deduplicating
  triggers. Across API versions, one Kubernetes resource has the same
  logical group/resource, namespace, name and UID, even if its GVK differs.
  The **shared trigger discovery** preserves distinct base-resource and
  subresource admission contexts: a MutatingPolicy may match `pods/status`
  without matching `pods`. **GeneratingPolicy callers** separately collapse
  those eligible contexts to one parent object before building RuleContext,
  because generation does not carry admission SubResource. Never conflate
  different API groups, kinds, namespaces, names, or object UIDs.
  Deduplication before exclusion filtering can silently lose eligible work.
- The current discovery helper logs failed mappings and continues; changing
  error propagation/reconciliation retry behavior requires separate review
  of all callers rather than a silent signature change.

## Regression tests and fake discovery

`gpol_test.go` includes discovery-backed RESTMapper fixtures and fake dynamic
clients. A custom Kind may intentionally have a plural resource name that does
*not* match client-go's default kind-to-plural guess. Create those fixtures
through the fake dynamic client's `CreateResource` after registering explicit
GVR-to-GVK discovery mappings; otherwise the object tracker can store data
under the wrong GVR and `ListResource` returns an empty list.

Run relevant tests with:

```sh
go test ./pkg/policy/... -run TestGetGpolTriggers -count=1 -v
go test -race ./pkg/policy/... -count=1
```
