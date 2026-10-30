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

## JMESPath-to-CEL projection translation

Migrating a projection's `jmesPath` to `expression` is not a mechanical find-replace. The shapes
below are the ones that actually occur in this repo's conformance fixtures
(`test/conformance/chainsaw/*/context/*/gctxentry.yaml`); verify any expression you write against
its downstream consumer, not just against the JMESPath source.

| JMESPath | CEL `expression` | Notes |
|---|---|---|
| `items \| length(@)` | `object.items.size()` | Result type differs: JMESPath always returns `float64`; `.size()` returns `int64`. |
| `items[?metadata.name=='x']` | `object.items.filter(i, i.metadata.name == 'x')` | CEL `filter` is the direct analog of JMESPath's `[?...]`. |
| `items[?metadata.name=='x'].metadata.labels.app` | `object.items.filter(i, i.metadata.name == 'x').map(i, i.metadata.labels.app)` | Filter projections return a list in both languages, even with exactly one match — don't collapse to a scalar unless the JMESPath source also had `\| [0]`. |
| `... \| [0]` | `...[0]` | **Not equivalent on an empty list.** JMESPath's `[0]` on an empty result returns `null`; CEL's `[0]` on an empty list throws a runtime error ("index out of bounds: 0") that surfaces as a projection evaluation failure. If the filtered list can legitimately be empty, guard it (e.g. with `.size() > 0 ? ...[0] : <default>`) instead of indexing directly — don't assume the two are interchangeable just because they look the same syntactically. |

The `object` binding and numeric-type notes above apply to every row in this table: an `apiCall`
entry's `object` is the parsed response (so `object.items...`), while a `kubernetesResource`
entry's `object` is already the list (so `object.filter(...)`, no `.items`).

This repo does not maintain a full JMESPath-to-CEL function parity table here; see issue #18341 for
the usage-metric-driven approach to that broader question.
