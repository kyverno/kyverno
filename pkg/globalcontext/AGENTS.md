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
- `externalapi.entry.publishMu` serializes the final cancellation check and
  publication (data, events, and refresh status) with `Stop`. Hold this gate
  across related side effects; do not call `Stop` from the publish callback.
- `Stop` marks the entry stopped and cancels the polling context while holding
  that gate; it must release the gate before waiting for the worker to exit.
- Do not retry API requests after context cancellation. Real API failures
  while the entry is active still need their normal error state and event.

When changing either path, retain regression tests for nested cache isolation,
in-flight cancellation, successful/failed polls and concurrent shutdown.
