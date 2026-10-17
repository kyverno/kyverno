# PolicyException inventory metrics

This package registers an OpenTelemetry scrape-time callback, not a queue-backed
controller. It has no Run method or event-maintained counters.

- `NewController` checks discovery once at startup for each optional PolicyException
  API before instantiating its shared-factory informer. Missing APIs and discovery
  failures must not add a metric-only informer to startup cache-sync requirements.
  A skipped source requires a controller restart after its API becomes available.
- Register before informer startup and outside leader election in `cmd/kyverno/main.go`
  so every admission replica exposes inventory. Other controllers may independently
  require these APIs; this package only controls its own informer dependencies.
- Scrape callbacks read synchronized listers only. Never perform discovery or other
  Kubernetes API calls during collection. Skip unsynchronized or absent sources
  quietly and handle source failures independently.
- Preserve resource inventory semantics: include expired exceptions and unresolved
  policy references, deduplicate references, and recompute each scrape so obsolete
  series disappear. Keep the documented five-label contract and namespace filtering.
- Retain callback registration across metric-provider recreation in `pkg/metrics`.
- Run race tests for this package and `pkg/metrics`. Constructor tests change the
  global metrics manager and must remain serial. The Chainsaw lifecycle proof is in
  `test/conformance/chainsaw/metrics/policy-exception-inventory`.
