# Kubernetes-resource-backed GlobalContextEntry projections

This package owns the dynamic informer and derived JMESPath projections for a
Kubernetes-resource-backed GlobalContextEntry. Keep these invariants when
changing event handling or projection lifecycle:

- Informer Add, Update, and Delete callbacks must be nonblocking and must not
  enumerate the cache or evaluate JMESPath queries. A capacity-one dirty signal
  is sufficient because refresh always reads the entire latest lister snapshot.
- Wait until informer cache synchronization has succeeded before computing and
  publishing the initial projections. New must return only after an initial
  snapshot has been computed, so callers can read named projections immediately.
- Never lose notifications delivered while the first snapshot or any subsequent
  refresh is running. Changes during a refresh must trigger another refresh.
- A fixed coalescing window avoids redundant scans without indefinitely
  postponing refreshes under continuous events. Do not turn this into a
  reset-on-every-event debounce.
- Run at most one projection evaluation worker per entry. Publish successful
  results together under the projection write lock, and retain last-known-good
  values for projection failures.
- Attach any background worker to the same context and wait group as the
  informer. Never call Stop from the reflector's watch error callback: it can
  deadlock waiting on its own goroutine. Cancellation and repeated/concurrent
  Stop calls must be safe.
- Entries without named projections should not install the notification
  handler or launch a projection worker.

Before proposing lifecycle changes, run the tests in this package, including
the race detector, and cover the initial-sync-to-worker handoff with a fake
dynamic client. Measure the coalescing/freshness trade-off before changing the
refresh interval.
