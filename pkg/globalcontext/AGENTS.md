# GlobalContextEntry cache ownership and lifecycle

This directory implements entries backed by Kubernetes shared informers and
periodically refreshed external APIs. These boundaries matter for correctness.

## Cache reads

- Objects returned by a shared informer/lister are not owned by the consumer.
  Do not expose or mutate `unstructured.Unstructured.Object` from the informer.
- `k8sresource.listObjects` must return independent deep-copied snapshots,
  including nested maps and slices, before JMESPath evaluation or policy use.
- Preserve isolation in both directions: changing a returned snapshot cannot
  alter the lister cache, and later cache changes cannot mutate that snapshot.

## External API polling and Stop

- An external API poll may finish concurrently with `Stop` or cancellation of
  its parent context. A standalone `ctx.Err()` check before publishing is
  insufficient, because shutdown can occur immediately after the check.
- `externalapi.entry.publishMu` serializes the final cancellation check
  and cache/event publication with `Stop`. Publication callbacks must not
  call `Stop` or perform blocking Kubernetes status I/O.
- `Stop` cancels the polling context **before** acquiring the publication
  gate, then marks the entry stopped and releases the gate before waiting for
  the polling worker. This enables cancellation of in-flight status calls.
- External API status refreshes run outside `publishMu`, under the separate
  `statusMu` gate, using the cancellable polling context. `Stop` cancels before
  acquiring `statusMu`, then waits for in-flight refreshes to leave that gate.
  Completed shutdown cannot leave an outstanding refresh, and cancellation-
  driven status failures must not be logged.
- A callback already inside the publication gate can finish during concurrent
  shutdown; no subsequent callback may start publishing after `Stop`
  completes.
- Do not retry API requests after context cancellation. Real API failures
  while the entry is active still need their normal error state and event.

When changing either path, retain regression tests for nested cache isolation,
in-flight cancellation, successful/failed polls and concurrent shutdown.
