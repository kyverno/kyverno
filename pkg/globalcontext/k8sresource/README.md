# Kubernetes-resource GlobalContextEntry informer lifecycle

This package caches Kubernetes objects for resource-backed `GlobalContextEntry` entries. Every entry owns a dynamic informer, its lister, and (when configured) projections.

## Startup and recovery invariants

- `New` waits for **initial** informer cache synchronization before publishing an entry. When initial synchronization cannot complete, return an error and release its informer. The cache-sync timeout bounds this path so one missing CRD cannot block the controller's reconciliation worker indefinitely.
- Errors before initial sync may cancel the entry context to interrupt initialization. Guard the initialization transition so concurrent reflector errors cannot cause a cancelled informer to be returned as a usable entry.
- **After a successful initial sync**, the informer must remain running through transient LIST/WATCH errors. Client-go's reflector owns retry/relist and cache recovery. Terminating the informer after sync leaves raw and projected values stale even if the Kubernetes API recovers.
- Never invoke `Stop` (or a `group.Wait`-based teardown) synchronously inside `SetWatchErrorHandler`: the callback executes on the reflector goroutine and waiting for that goroutine causes a self-join deadlock.
- Warning events for repeated post-sync failures are rate-limited per entry to avoid flooding Kubernetes events during extended outages. Initial sync failure diagnostics must remain observable.
- `Stop` must be safe on replacement and cancellation, including concurrent calls. A stopped entry must not be exposed as a successfully initialized entry.

## Regression coverage

`watch_recovery_repro_test.go` uses a real client-go dynamic informer and a fake LIST/WATCH source to exercise:

- healthy relist after watch expiration;
- one failed relist followed by successful recovery of raw objects and named projections without recreating the entry;
- recovery of entries without projections;
- initial LIST failure and prompt cancellation;
- error-event throttling boundaries.

Run `go test ./pkg/globalcontext/k8sresource -count=1` and `go test -race ./pkg/globalcontext/k8sresource -count=1` for this package. Integration coverage is also necessary to verify downstream policy reads.
