# AGENTS.md — pkg/globalcontext

Gotchas for the GlobalContextEntry (GCE) cache/evaluation code, not covered by the general CEL or
controller docs.

- `object` binding differs by entry type and is NOT normalized between them:
  - `externalapi` (apiCall): `object` is the parsed API response body.
  - `k8sresource` (kubernetesResource): `object` is the list of matching objects directly — not
    wrapped in `{"items": [...]}`. Writing an expression that assumes the apiCall shape against a
    kubernetesResource entry (or vice versa) compiles fine and fails only at evaluation time.
- A projection evaluating to CEL null (or an empty optional) caches as a **present key with a nil
  value**. `Get` on both `externalapi.entry` and `k8sresource.entry` must use the comma-ok map
  idiom (`v, ok := m[k]`) to distinguish "cached nil" from "never computed" — a bare `v == nil`
  check collapses the two and misreports a valid nil result as "no data available".
- CEL projections (`expression`) keep their CEL-native numeric type (`int64`, `uint64`, or
  `float64`); the JMESPath twin (`jmesPath`) always produces `float64`. The two are not
  interchangeable in downstream arithmetic even for projections on the same entry.
- Projections re-evaluate on the refresh path, not at read time: `k8sresource` recomputes on every
  informer add/update/delete event (`recomputeProjections`, invoked from the handler goroutine);
  `externalapi` recomputes on its poll tick (`setData`). `Get` only ever returns the last cached
  result.
